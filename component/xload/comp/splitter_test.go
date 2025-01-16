package comp

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	xcommon "github.com/Azure/azure-storage-fuse/v2/component/xload/common"
	xinternal "github.com/Azure/azure-storage-fuse/v2/component/xload/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type splitterTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

var remote internal.Component
var remote_path string

func (suite *splitterTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.Nil(err)

	remote_path = filepath.Join("/tmp/", "xload_"+randomString(8))
	err = os.MkdirAll(remote_path, 0777)
	suite.assert.Nil(err)

	cfg := fmt.Sprintf("loopbackfs:\n  path: %s\n", remote_path)
	config.ReadConfigFromReader(strings.NewReader(cfg))

	remote = loopback.NewLoopbackFSComponent()
	err = remote.Configure(true)
	suite.assert.Nil(err)

	suite.createDirs(remote_path)
}

func (suite *splitterTestSuite) TearDownSuite() {
	err := os.RemoveAll(remote_path)
	suite.assert.Nil(err)
}

func (suite *splitterTestSuite) createDirs(path string) {
	suite.createFiles(path)

	for i := 0; i < 2; i++ {
		dirName := filepath.Join(path, fmt.Sprintf("dir_%v", i))
		err := os.MkdirAll(dirName, 0777)
		suite.assert.Nil(err)

		suite.createFiles(dirName)
	}
}

func (suite *splitterTestSuite) createFiles(path string) {
	for i := 0; i < 5; i++ {
		filePath := filepath.Join(path, fmt.Sprintf("file_%v", i))
		f, err := os.Create(filePath)
		defer func() {
			err = f.Close()
			suite.assert.Nil(err)
		}()
		suite.assert.Nil(err)

		n, err := f.Write([]byte(randomString(9 * i)))
		suite.assert.Nil(err)
		suite.assert.Equal(n, 9*i)

		err = os.Truncate(filePath, int64(9*i))
		suite.assert.Nil(err)

	}
}

type testSplitter struct {
	path      string
	blockSize uint64
	blockPool *xcommon.BlockPool
	locks     *common.LockMap
	stMgr     *xinternal.StatsManager
}

func setupTestSplitter() (*testSplitter, error) {
	ts := &testSplitter{}
	ts.path = filepath.Join("/tmp/", fmt.Sprintf("xsplitter_%v", randomString(8)))
	err := os.MkdirAll(ts.path, 0777)
	if err != nil {
		return nil, err
	}

	ts.blockSize = 10
	ts.blockPool = xcommon.NewBlockPool(ts.blockSize, 10)
	ts.locks = common.NewLockMap()

	ts.stMgr, err = xinternal.NewStatsManager(10, false)
	if err != nil {
		return nil, err
	}

	ts.stMgr.Start()
	return ts, nil
}

func (ts *testSplitter) cleanup() error {
	ts.stMgr.Stop()
	ts.blockPool.Terminate()

	err := os.RemoveAll(ts.path)
	return err
}

func (suite *splitterTestSuite) TestNewDownloadSplitter() {
	ds, err := NewDownloadSplitter(0, nil, "", nil, nil, nil)
	suite.assert.NotNil(err)
	suite.assert.Nil(ds)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create download splitter")

	statsMgr, err := xinternal.NewStatsManager(1, false)
	suite.assert.Nil(err)
	suite.assert.NotNil(statsMgr)

	ds, err = NewDownloadSplitter(1, xcommon.NewBlockPool(1, 1), "/home/user/random_path", remote, statsMgr, common.NewLockMap())
	suite.assert.Nil(err)
	suite.assert.NotNil(ds)
}

func (suite *splitterTestSuite) TestSplitterStartStop() {
	ts, err := setupTestSplitter()
	suite.assert.Nil(err)
	suite.assert.NotNil(ts)

	defer func() {
		err = ts.cleanup()
		suite.assert.Nil(err)
	}()

	rl, err := NewRemoteLister(ts.path, remote, ts.stMgr)
	suite.assert.Nil(err)
	suite.assert.NotNil(rl)

	ds, err := NewDownloadSplitter(ts.blockSize, ts.blockPool, ts.path, remote, ts.stMgr, ts.locks)
	suite.assert.Nil(err)
	suite.assert.NotNil(ds)

	rdm, err := NewRemoteDataManager(remote, ts.stMgr)
	suite.assert.Nil(err)
	suite.assert.NotNil(rdm)

	// create chain
	rl.SetNext(ds)
	ds.SetNext(rdm)

	// start components
	rdm.Start()
	ds.Start()
	rl.Start()

	time.Sleep(5 * time.Second)

	// stop comoponents
	rl.Stop()

	suite.validateMD5(ts.path, remote_path)
}

func (suite *splitterTestSuite) validateMD5(localPath string, remotePath string) {
	entries, err := os.ReadDir(remotePath)
	suite.assert.Nil(err)

	for _, entry := range entries {
		localFile := filepath.Join(localPath, entry.Name())
		remoteFile := filepath.Join(remotePath, entry.Name())

		if entry.IsDir() {
			f, err := os.Stat(localFile)
			suite.assert.Nil(err)
			suite.assert.True(f.IsDir())

			suite.validateMD5(localFile, remoteFile)
		} else {
			l, err := computeMD5(localFile)
			suite.assert.Nil(err)

			r, err := computeMD5(remoteFile)
			suite.assert.Nil(err)

			suite.assert.Equal(l, r)
		}
	}
}

func computeMD5(filePath string) ([]byte, error) {
	fh, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	hash := md5.New()
	if _, err := io.Copy(hash, fh); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func TestSplitterSuite(t *testing.T) {
	suite.Run(t, new(splitterTestSuite))
}
