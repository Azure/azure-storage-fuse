// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package xload

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type listTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

var lb internal.Component
var lb_path string
var r *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(length int) string {
	b := make([]byte, length)
	r.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *listTestSuite) SetupSuite() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)

	lb_path = filepath.Join("/tmp/", "xload_"+randomString(8))
	err = os.MkdirAll(lb_path, 0777)
	suite.assert.NoError(err)

	cfg := fmt.Sprintf("loopbackfs:\n  path: %s\n", lb_path)
	err = config.ReadConfigFromReader(strings.NewReader(cfg))
	suite.assert.NoError(err)

	lb = loopback.NewLoopbackFSComponent()
	err = lb.Configure(true)
	suite.assert.NoError(err)

	suite.createDirsAndFiles(lb_path)
}

func (suite *listTestSuite) TearDownSuite() {
	err := os.RemoveAll(lb_path)
	suite.assert.NoError(err)
}

func (suite *listTestSuite) createFile(filePath string, size int64) {
	f, err := os.Create(filePath)
	defer func() {
		err = f.Close()
		suite.assert.NoError(err)
	}()
	suite.assert.NoError(err)

	err = os.Truncate(filePath, size)
	suite.assert.NoError(err)
}

func (suite *listTestSuite) createDirsAndFiles(path string) {
	for i := range 10 {
		filePath := filepath.Join(path, fmt.Sprintf("file_%v", i))
		suite.createFile(filePath, int64(9*i))
	}

	for i := range 10 {
		dirName := filepath.Join(path, fmt.Sprintf("dir_%v", i))
		err := os.MkdirAll(dirName, 0777)
		suite.assert.NoError(err)

		for j := range 5 {
			filePath := filepath.Join(dirName, fmt.Sprintf("file_%v%v", i, j))
			suite.createFile(filePath, int64(i*j))
		}
	}
}

type testComponent struct {
	XBase
	ctr atomic.Int64
}

func getTestcomponent() *testComponent {
	tc := &testComponent{}
	tc.SetThreadPool(NewThreadPool(1, tc.Process))
	tc.GetThreadPool().Start(context.TODO())
	return tc
}

func (tcmp *testComponent) Stop() {
	tcmp.GetThreadPool().Stop()
}

func (tcmp *testComponent) Process(item *WorkItem) (int, error) {
	tcmp.ctr.Add(1)
	return int(tcmp.ctr.Load()), nil
}

type testLister struct {
	path  string
	root  *os.Root
	stMgr *StatsManager
}

func setupTestLister() (*testLister, error) {
	tl := &testLister{}
	tl.path = filepath.Join("/tmp/", fmt.Sprintf("xlister_%v", randomString(8)))
	err := os.MkdirAll(tl.path, 0777)
	if err != nil {
		return nil, err
	}

	tl.root, err = os.OpenRoot(tl.path)
	if err != nil {
		return nil, err
	}

	tl.stMgr, err = NewStatsManager(1, false, nil)
	if err != nil {
		return nil, err
	}

	tl.stMgr.Start()
	return tl, nil
}

func (tl *testLister) cleanup() error {
	tl.stMgr.Stop()
	_ = tl.root.Close()

	err := os.RemoveAll(tl.path)
	return err
}

func (suite *listTestSuite) TestNewRemoteLister() {
	root, err := os.OpenRoot(os.TempDir())
	suite.assert.NoError(err)
	defer root.Close()

	rl, err := newRemoteLister(nil)
	suite.assert.Error(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	rl, err = newRemoteLister(&remoteListerOptions{
		root:              nil,
		workerCount:       0,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            nil,
		statsMgr:          nil,
	})
	suite.assert.Error(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	rl, err = newRemoteLister(&remoteListerOptions{
		root:              root,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            nil,
		statsMgr:          nil,
	})
	suite.assert.Error(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	rl, err = newRemoteLister(&remoteListerOptions{
		root:              root,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          nil,
	})
	suite.assert.Error(err)
	suite.assert.Nil(rl)
	suite.assert.Contains(err.Error(), "invalid parameters sent to create remote lister")

	statsMgr, err := NewStatsManager(1, false, nil)
	suite.assert.NoError(err)
	suite.assert.NotNil(statsMgr)

	rl, err = newRemoteLister(&remoteListerOptions{
		root:              root,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          statsMgr,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(rl)
}

func (suite *listTestSuite) TestListerStartStop() {
	tl, err := setupTestLister()
	suite.assert.NoError(err)
	suite.assert.NotNil(tl)

	defer func() {
		err = tl.cleanup()
		suite.assert.NoError(err)
	}()

	rl, err := newRemoteLister(&remoteListerOptions{
		root:              tl.root,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          tl.stMgr,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(rl)

	testComp := getTestcomponent()
	rl.SetNext(testComp)

	rl.Start(context.TODO())
	time.Sleep(5 * time.Second)
	rl.Stop()

	suite.assert.Equal(int64(60), testComp.ctr.Load())

	entries, err := os.ReadDir(tl.path)
	suite.assert.NoError(err)
	suite.assert.Len(entries, 10)
}

func (suite *listTestSuite) TestListerMkdir() {
	tl, err := setupTestLister()
	suite.assert.NoError(err)
	suite.assert.NotNil(tl)

	defer func() {
		err = tl.cleanup()
		suite.assert.NoError(err)
	}()

	rl, err := newRemoteLister(&remoteListerOptions{
		root:              tl.root,
		workerCount:       4,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            lb,
		statsMgr:          tl.stMgr,
	})
	suite.assert.NoError(err)
	suite.assert.NotNil(rl)

	for i := range 5 {
		err = rl.mkdir(fmt.Sprintf("dir%v", i))
		suite.assert.NoError(err)
	}

	entries, err := os.ReadDir(tl.path)
	suite.assert.NoError(err)
	suite.assert.Len(entries, 5)
}

type nonLocalRemote struct {
	internal.BaseComponent
	data   []byte
	listed []string
	mu     sync.Mutex
}

func (n *nonLocalRemote) Name() string { return "nonLocalRemote" }

func (n *nonLocalRemote) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	n.mu.Lock()
	n.listed = append(n.listed, options.Name)
	n.mu.Unlock()

	if options.Name != "" {
		return nil, "", nil
	}

	up := ".."
	fileAttr := func(path string) *internal.ObjAttr {
		return &internal.ObjAttr{Path: path, Size: int64(len(n.data)), Mode: 0644, Flags: internal.NewFileBitMap()}
	}
	return []*internal.ObjAttr{
		{Path: "safe_dir", Flags: internal.NewDirBitMap()},
		{Path: "", Flags: internal.NewDirBitMap()},
		{Path: filepath.Join(up, "escaped_dir"), Flags: internal.NewDirBitMap()},
		{Path: "link_dir/escaped_dir", Flags: internal.NewDirBitMap()},
		fileAttr("safe_file"),
		fileAttr(filepath.Join(up, "victim")),
		fileAttr("/" + filepath.Join(up, "escaped_file")),
		fileAttr("safe_dir/" + up + "/" + up + "/escaped_file"),
		fileAttr("safe_dir/" + up + "/aliased_file"),
		fileAttr("link_dir/escaped_file"),
		fileAttr("link_file"),
	}, "", nil
}

func (n *nonLocalRemote) ReadInBuffer(options *internal.ReadInBufferOptions) (int, error) {
	end := min(options.Offset+int64(len(options.Data)), int64(len(n.data)))
	return copy(options.Data, n.data[options.Offset:end]), nil
}

func (suite *listTestSuite) TestListerStaysWithinPath() {
	tl, err := setupTestLister()
	suite.assert.NoError(err)
	defer func() {
		suite.assert.NoError(tl.cleanup())
	}()

	cachePath := filepath.Join(tl.path, "cache")
	suite.assert.NoError(os.Mkdir(cachePath, 0777))
	cacheRoot, err := os.OpenRoot(cachePath)
	suite.assert.NoError(err)
	defer cacheRoot.Close()

	victimData := []byte("local data outside the cache")
	victimPath := filepath.Join(tl.path, "victim")
	suite.assert.NoError(os.WriteFile(victimPath, victimData, 0644))
	outsideDir := filepath.Join(tl.path, "outside_dir")
	suite.assert.NoError(os.Mkdir(outsideDir, 0777))

	// symlinks planted in the cache that point outside of it
	suite.assert.NoError(os.Symlink(outsideDir, filepath.Join(cachePath, "link_dir")))
	suite.assert.NoError(os.Symlink(victimPath, filepath.Join(cachePath, "link_file")))

	remote := &nonLocalRemote{data: []byte("remote data")}
	blockPool := NewBlockPool(4, 10, context.TODO())
	defer blockPool.Terminate()

	statsMgr, err := NewStatsManager(10, false, nil)
	suite.assert.NoError(err)
	statsMgr.Start()

	rl, err := newRemoteLister(&remoteListerOptions{
		root:              cacheRoot,
		workerCount:       2,
		defaultPermission: common.DefaultFilePermissionBits,
		remote:            remote,
		statsMgr:          statsMgr,
	})
	suite.assert.NoError(err)

	ds, err := newDownloadSplitter(&downloadSplitterOptions{blockPool, cacheRoot, 2, remote, statsMgr, common.NewLockMap(), false})
	suite.assert.NoError(err)

	rdm, err := newRemoteDataManager(&remoteDataManagerOptions{workerCount: 2, remote: remote, statsMgr: statsMgr})
	suite.assert.NoError(err)

	rl.SetNext(ds)
	ds.SetNext(rdm)
	rdm.Start(context.TODO())
	ds.Start(context.TODO())
	rl.Start(context.TODO())
	time.Sleep(2 * time.Second)
	rl.Stop()
	ds.Stop()
	rdm.Stop()
	statsMgr.Stop()

	// every listed entry ends as done or failed, so that the progress can reach 100%
	suite.assert.EqualValues(11, statsMgr.totalFiles)
	suite.assert.EqualValues(2, statsMgr.success)
	suite.assert.EqualValues(9, statsMgr.failed)

	entries, err := os.ReadDir(tl.path)
	suite.assert.NoError(err)
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	suite.assert.ElementsMatch([]string{"cache", "outside_dir", "victim"}, names)

	entries, err = os.ReadDir(outsideDir)
	suite.assert.NoError(err)
	suite.assert.Empty(entries)

	data, err := os.ReadFile(victimPath)
	suite.assert.NoError(err)
	suite.assert.Equal(victimData, data)

	data, err = os.ReadFile(filepath.Join(cachePath, "safe_file"))
	suite.assert.NoError(err)
	suite.assert.Equal(remote.data, data)
	suite.assert.DirExists(filepath.Join(cachePath, "safe_dir"))
	suite.assert.NoFileExists(filepath.Join(cachePath, "aliased_file"))

	remote.mu.Lock()
	suite.assert.ElementsMatch([]string{"", "safe_dir"}, remote.listed)
	remote.mu.Unlock()
}

func TestListSuite(t *testing.T) {
	suite.Run(t, new(listTestSuite))
}
