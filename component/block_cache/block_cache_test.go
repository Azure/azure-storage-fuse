/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package block_cache

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/pbnjay/memory"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()
var mountpoint = home_dir + "mountpoint"
var dataBuff []byte
var r *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

type blockCacheTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockCacheTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())

	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	suite.assert.NoError(err)
}

type testObj struct {
	fake_storage_path string
	disk_cache_path   string
	loopback          internal.Component
	blockCache        *BlockCache
}

func randomString(length int) string {
	b := make([]byte, length)
	r.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func getFakeStoragePath(base string) string {
	tmp_path := filepath.Join(home_dir, base+randomString(8))
	_ = os.Mkdir(tmp_path, 0777)
	return tmp_path
}

func getTestFileName(name string) string {
	n := strings.Split(name, "/")
	return n[len(n)-1]
}

func setupPipeline(cfg string) (*testObj, error) {
	tobj := &testObj{
		fake_storage_path: getFakeStoragePath("block_cache"),
		disk_cache_path:   getFakeStoragePath("fake_storage"),
	}

	if cfg == "" {
		cfg = fmt.Sprintf("read-only: true\n\nloopbackfs:\n  path: %s\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10\n  path: %s\n  disk-size-mb: 50\n  disk-timeout-sec: 20", tobj.fake_storage_path, tobj.disk_cache_path)
	} else {
		cfg = fmt.Sprintf("%s\n\nloopbackfs:\n  path: %s\n", cfg, tobj.fake_storage_path)
	}

	config.ReadConfigFromReader(strings.NewReader(cfg))
	config.Set("mount-path", mountpoint)
	tobj.loopback = loopback.NewLoopbackFSComponent()
	err := tobj.loopback.Configure(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure loopback [%s]", err.Error())
	}

	tobj.blockCache = NewBlockCacheComponent().(*BlockCache)
	tobj.blockCache.SetNextComponent(tobj.loopback)
	err = tobj.blockCache.Configure(true)
	if err != nil {
		return nil, fmt.Errorf("Unable to configure blockcache [%s]", err.Error())
	}

	err = tobj.loopback.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Unable to start loopback [%s]", err.Error())
	}

	err = tobj.blockCache.Start(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Unable to start blockcache [%s]", err.Error())
	}

	return tobj, nil
}

func (tobj *testObj) cleanupPipeline() error {
	if tobj == nil {
		return nil
	}

	if tobj.loopback != nil {
		err := tobj.loopback.Stop()
		if err != nil {
			return fmt.Errorf("Unable to stop loopback [%s]", err.Error())
		}
	}

	if tobj.blockCache != nil {
		err := tobj.blockCache.Stop()
		if err != nil {
			return fmt.Errorf("Unable to stop block cache [%s]", err.Error())
		}
	}

	os.RemoveAll(tobj.fake_storage_path)
	os.RemoveAll(tobj.disk_cache_path)

	common.IsStream = false
	return nil
}

// Tests the default configuration of block cache
func (suite *blockCacheTestSuite) TestEmpty() {
	emptyConfig := "read-only: true"
	tobj, err := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	if err != nil {
		// On some distros due to low memory, block cache init fails.
		suite.assert.Contains(err.Error(), "memory limit too low for configured prefetch")
		return
	}

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())
	suite.assert.EqualValues(16*_1MB, tobj.blockCache.blockSize)
	suite.assert.EqualValues(0, tobj.blockCache.diskSize)
	suite.assert.EqualValues(defaultTimeout, tobj.blockCache.diskTimeout)

	cmd := exec.Command("nproc")
	output, err := cmd.Output()
	suite.assert.NoError(err)
	coresStr := strings.TrimSpace(string(output))
	cores, err := strconv.Atoi(coresStr)
	suite.assert.NoError(err)
	suite.assert.Equal(tobj.blockCache.workers, uint32(3*cores))
	suite.assert.EqualValues(tobj.blockCache.prefetch, math.Max((MIN_PREFETCH*2)+1, float64(2*cores)))
	suite.assert.False(tobj.blockCache.noPrefetch)
	suite.assert.NotNil(tobj.blockCache.blockPool)
	suite.assert.NotNil(tobj.blockCache.threadPool)
}

func (suite *blockCacheTestSuite) TestMemory() {
	emptyConfig := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n"
	tobj, err := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	if err != nil {
		// On some distros due to low memory, block cache init fails.
		suite.assert.Contains(err.Error(), "memory limit too low for configured prefetch")
		return
	}

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())
	cmd := exec.Command("bash", "-c", "free -b | grep Mem | awk '{print $4}'")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)
	free, err := strconv.Atoi(strings.TrimSpace(out.String()))
	suite.assert.NoError(err)
	expected := uint64(0.8 * float64(free))
	actual := tobj.blockCache.memSize
	difference := math.Abs(float64(actual) - float64(expected))
	tolerance := 0.10 * float64(math.Max(float64(actual), float64(expected)))
	suite.assert.LessOrEqual(difference, tolerance)
}

func (suite *blockCacheTestSuite) TestFreeDiskSpace() {
	disk_cache_path := getFakeStoragePath("fake_storage")
	config := fmt.Sprintf("read-only: true\n\nblock_cache:\n  block-size-mb: 1\n  path: %s", disk_cache_path)
	tobj, err := setupPipeline(config)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())

	cmd := exec.Command("bash", "-c", fmt.Sprintf("df -B1 %s | awk 'NR==2{print $4}'", disk_cache_path))
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)
	freeDisk, err := strconv.Atoi(strings.TrimSpace(out.String()))
	suite.assert.NoError(err)
	expected := uint64(0.8 * float64(freeDisk))
	actual := tobj.blockCache.diskSize
	difference := math.Abs(float64(actual) - float64(expected))
	tolerance := 0.10 * float64(math.Max(float64(actual), float64(expected)))
	suite.assert.LessOrEqual(difference, tolerance)
}

func (suite *blockCacheTestSuite) TestStatfsMemory() {
	emptyConfig := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n"
	tobj, err := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())
	cmd := exec.Command("bash", "-c", "free -b | grep Mem | awk '{print $4}'")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)
	free, err := strconv.Atoi(strings.TrimSpace(out.String()))
	suite.assert.NoError(err)
	expected := uint64(0.8 * float64(free))
	stat, ret, err := tobj.blockCache.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotEqual(&syscall.Statfs_t{}, stat)
	actual := tobj.blockCache.memSize
	difference := math.Abs(float64(actual) - float64(expected))
	tolerance := 0.10 * float64(math.Max(float64(actual), float64(expected)))
	suite.assert.LessOrEqual(difference, tolerance)
}

func (suite *blockCacheTestSuite) TestStatfsDisk() {
	disk_cache_path := getFakeStoragePath("fake_storage")
	config := fmt.Sprintf("read-only: true\n\nblock_cache:\n  block-size-mb: 1\n  path: %s", disk_cache_path)
	tobj, err := setupPipeline(config)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())

	cmd := exec.Command("bash", "-c", fmt.Sprintf("df -B1 %s | awk 'NR==2{print $4}'", disk_cache_path))
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)
	freeDisk, err := strconv.Atoi(strings.TrimSpace(out.String()))
	suite.assert.NoError(err)
	expected := uint64(0.8 * float64(freeDisk))
	stat, ret, err := tobj.blockCache.StatFs()
	suite.assert.True(ret)
	suite.assert.NoError(err)
	suite.assert.NotEqual(&syscall.Statfs_t{}, stat)
	actual := tobj.blockCache.diskSize
	difference := math.Abs(float64(actual) - float64(expected))
	tolerance := 0.10 * float64(math.Max(float64(actual), float64(expected)))
	suite.assert.LessOrEqual(difference, tolerance)
}

func (suite *blockCacheTestSuite) TestInvalidPrefetchCount() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 8\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "invalid prefetch count")
}

func (suite *blockCacheTestSuite) TestInvalidMemoryLimitPrefetchCount() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 320\n  prefetch: 50\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "[memory limit too low for configured prefetch")
}

func (suite *blockCacheTestSuite) TestNoPrefetchConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 500\n  prefetch: 0\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)
	suite.assert.True(tobj.blockCache.noPrefetch)
}

func (suite *blockCacheTestSuite) TestInvalidDiskPath() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: /abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "permission denied")
}

func (suite *blockCacheTestSuite) TestSomeInvalidConfigs() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 0\n"
	_, err := setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "failed to init thread pool")

	cfg = "read-only: true\n\nblock_cache:\n  block-size-mb: 1024000\n  mem-size-mb: 20240000\n  prefetch: 12\n  parallelism: 1\n"
	_, err = setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "failed to init block pool")

	cfg = "read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 5\n  path: ./bctemp \n  disk-size-mb: 100\n  disk-timeout-sec: 0"
	_, err = setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "timeout can not be zero")

	cfg = "read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 5\n  path: ./ \n  disk-size-mb: 100\n  disk-timeout-sec: 0"
	_, err = setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "temp directory not empty")

	cfg = fmt.Sprintf("read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 5\n  path: %s \n  disk-size-mb: 100\n  disk-timeout-sec: 0", mountpoint)
	_, err = setupPipeline(cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "tmp-path is same as mount path")
}

func (suite *blockCacheTestSuite) TestManualConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.Equal("block_cache", tobj.blockCache.Name())
	suite.assert.EqualValues(16*_1MB, tobj.blockCache.blockSize)
	suite.assert.EqualValues(500*_1MB, tobj.blockCache.memSize)
	suite.assert.EqualValues(10, tobj.blockCache.workers)
	suite.assert.EqualValues(100*_1MB, tobj.blockCache.diskSize)
	suite.assert.EqualValues(5, tobj.blockCache.diskTimeout)
	suite.assert.EqualValues(12, tobj.blockCache.prefetch)
	suite.assert.EqualValues(10, tobj.blockCache.workers)

	suite.assert.NotNil(tobj.blockCache.blockPool)
}

func (suite *blockCacheTestSuite) TestOpenFileFail() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	options := internal.OpenFileOptions{Name: path}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Error(err)
	suite.assert.Nil(h)
	suite.assert.Contains(err.Error(), "no such file or directory")
}

func (suite *blockCacheTestSuite) TestFileOpenClose() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 5*_1MB)
	_, _ = r.Read(data)
	os.WriteFile(storagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestValidateBlockList() {
	config := "read-only: true\n\nblock_cache:\n  block-size-mb: 20"
	tobj, err := setupPipeline(config)
	defer tobj.cleanupPipeline()
	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)
	suite.assert.Equal(20*_1MB, tobj.blockCache.blockSize)

	fileName := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, fileName)
	os.WriteFile(storagePath, []byte("Hello, World!"), 0777)
	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)

	//Test for Valid BlockList
	var blockLst internal.CommittedBlockList
	noOfBlocks := 20
	var startOffset int64

	//Generate blocklist, blocks with size equal to configured block size
	blockLst = nil
	startOffset = 0
	for range noOfBlocks {
		blockSize := tobj.blockCache.blockSize
		blk := internal.CommittedBlock{
			Id:     base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(32)),
			Offset: startOffset,
			Size:   uint64(blockSize),
		}
		startOffset += int64(blockSize)
		blockLst = append(blockLst, blk)
	}
	valid := tobj.blockCache.validateBlockList(h, options, &blockLst)
	suite.assert.True(valid)

	//Generate blocklist, blocks with size equal to configured block size and last block size <= config's block size
	blockLst = nil
	startOffset = 0
	for i := range noOfBlocks {
		blockSize := tobj.blockCache.blockSize
		if i == noOfBlocks-1 {
			blockSize = uint64(rand.Intn(int(tobj.blockCache.blockSize)))
		}
		blk := internal.CommittedBlock{
			Id:     base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(32)),
			Offset: startOffset,
			Size:   uint64(blockSize),
		}
		startOffset += int64(blockSize)
		blockLst = append(blockLst, blk)
	}
	valid = tobj.blockCache.validateBlockList(h, options, &blockLst)
	suite.assert.True(valid)

	//Generate Blocklist, blocks with size equal to configured block size and last block size > config's block size
	blockLst = nil
	startOffset = 0
	for i := range noOfBlocks {
		blockSize := tobj.blockCache.blockSize
		if i == noOfBlocks-1 {
			blockSize = tobj.blockCache.blockSize + uint64(rand.Intn(100)) + 1
		}
		blk := internal.CommittedBlock{
			Id:     base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(32)),
			Offset: startOffset,
			Size:   uint64(blockSize),
		}
		startOffset += int64(blockSize)
		blockLst = append(blockLst, blk)
	}
	valid = tobj.blockCache.validateBlockList(h, options, &blockLst)
	suite.assert.False(valid)

	//Generate Blocklist, blocks with random size
	blockLst = nil
	startOffset = 0
	for range noOfBlocks {
		blockSize := uint64(rand.Intn(int(tobj.blockCache.blockSize + 1)))
		blk := internal.CommittedBlock{
			Id:     base64.StdEncoding.EncodeToString(common.NewUUIDWithLength(32)),
			Offset: startOffset,
			Size:   uint64(blockSize),
		}
		startOffset += int64(blockSize)
		blockLst = append(blockLst, blk)
	}
	valid = tobj.blockCache.validateBlockList(h, options, &blockLst)
	suite.assert.False(valid)

}

func (suite *blockCacheTestSuite) TestFileReadTotalBytes() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	storagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())
	//Generate random size of file in bytes less than 2MB
	size := rand.Intn(2097152)
	data := make([]byte, size)

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // Write data to file
	suite.assert.NoError(err)
	suite.assert.Equal(n, size)
	suite.assert.Equal(h.Size, int64(size))

	data = make([]byte, 1000)

	totaldata := uint64(0)
	for {
		n, err := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: int64(totaldata), Data: data})
		totaldata += uint64(n)
		if err != nil {
			suite.assert.Contains(err.Error(), "EOF")
			break
		}
		suite.assert.LessOrEqual(n, 1000)
	}
	suite.assert.Equal(totaldata, uint64(size))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadBlockCacheTmpPath() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	storagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())
	//Size is 1MB + 7 bytes
	size := 1048583
	data := make([]byte, size)

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // Write data to file
	suite.assert.NoError(err)
	suite.assert.Equal(n, size)
	suite.assert.Equal(h.Size, int64(size))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	options2 := internal.OpenFileOptions{Name: path}
	h, err = tobj.blockCache.OpenFile(options2)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(size))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 1000)

	totaldata := uint64(0)
	for {
		n, err := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: int64(totaldata), Data: data})
		totaldata += uint64(n)
		if err != nil {
			suite.assert.Contains(err.Error(), "EOF")
			break
		}
		suite.assert.LessOrEqual(n, 1000)
	}
	suite.assert.Equal(totaldata, uint64(size))

	data = make([]byte, 1000)

	totaldata = uint64(0)
	for {
		n, err := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: int64(totaldata), Data: data})
		totaldata += uint64(n)
		if err != nil {
			suite.assert.Contains(err.Error(), "EOF")
			break
		}
		suite.assert.LessOrEqual(n, 1000)
	}
	suite.assert.Equal(totaldata, uint64(size))

	tmpPath := tobj.blockCache.tmpPath

	entries, err := os.ReadDir(tmpPath)
	suite.assert.NoError(err)

	var size1048576, size7 bool
	for _, entry := range entries {
		f, err := entry.Info()
		suite.assert.NoError(err)

		if f.Size() == 1048576 {
			size1048576 = true
		}
		if f.Size() == 7 {
			size7 = true
		}
	}

	suite.assert.True(size1048576)
	suite.assert.True(size7)
	suite.assert.Equal(2, len(entries))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestFileReadSerial() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 50*_1MB)
	_, _ = r.Read(data)
	os.WriteFile(storagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(50*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 1000)

	totaldata := uint64(0)
	for {
		n, err := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: int64(totaldata), Data: data})
		totaldata += uint64(n)
		if err != nil {
			break
		}
		suite.assert.LessOrEqual(n, 1000)
	}

	suite.assert.Equal(totaldata, uint64(50*_1MB))
	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(12, cnt)

	tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadRandom() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 100*_1MB)
	_, _ = r.Read(data)
	os.WriteFile(storagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(100*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 100)
	max := int64(100 * _1MB)
	for range 50 {
		offset := rand.Int63n(max)
		n, _ := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: offset, Data: data})
		suite.assert.LessOrEqual(n, 100)
	}

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.LessOrEqual(cnt, 8)

	tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadRandomNoPrefetch() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	// Set the no prefetch mode here
	tobj.blockCache.noPrefetch = true
	tobj.blockCache.prefetch = 0

	fileName := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 100*_1MB)
	_, _ = r.Read(data)
	os.WriteFile(storagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(100*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 100)
	max := int64(100 * _1MB)
	for range 50 {
		offset := rand.Int63n(max)
		n, _ := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: offset, Data: data})
		suite.assert.Equal(1, h.Buffers.Cooked.Len())
		suite.assert.Equal(0, h.Buffers.Cooking.Len())
		suite.assert.LessOrEqual(n, 100)
	}

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(1, cnt)

	tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestDiskUsageCheck() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	usage, err := common.GetUsage(tobj.disk_cache_path)
	suite.assert.NoError(err)
	suite.assert.Less(usage, float64(1.0))
	suite.assert.False(tobj.blockCache.checkDiskUsage())

	// Default disk size is 50MB
	data := make([]byte, 5*_1MB)
	_, _ = r.Read(data)

	type diskusagedata struct {
		name     string
		diskflag bool
	}

	localfiles := make([]diskusagedata, 0)
	for i := range 13 {
		fname := randomString(5)
		diskFile := filepath.Join(tobj.disk_cache_path, fname)
		localfiles = append(localfiles, diskusagedata{name: diskFile, diskflag: i >= 7})
	}

	for i := range 13 {
		os.WriteFile(localfiles[i].name, data, 0777)
		usage, err := common.GetUsage(tobj.disk_cache_path)
		suite.assert.NoError(err)
		fmt.Printf("%d : %v (%v : %v) Usage %v\n", i, localfiles[i].name, localfiles[i].diskflag, tobj.blockCache.checkDiskUsage(), usage)
		suite.assert.Equal(tobj.blockCache.checkDiskUsage(), localfiles[i].diskflag)
	}

	for i := range 13 {
		localfiles[i].diskflag = i < 8
	}

	for i := range 13 {
		os.Remove(localfiles[i].name)
		usage, err := common.GetUsage(tobj.disk_cache_path)
		suite.assert.NoError(err)
		fmt.Printf("%d : %v (%v : %v) Usage %v\n", i, localfiles[i].name, localfiles[i].diskflag, tobj.blockCache.checkDiskUsage(), usage)
		suite.assert.Equal(tobj.blockCache.checkDiskUsage(), localfiles[i].diskflag)
	}
}

// Block-cache Writer related test cases
func (suite *blockCacheTestSuite) TestCreateFile() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	options := internal.CreateFileOptions{Name: path}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	storagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())

	path = "FailThis"
	options = internal.CreateFileOptions{Name: path}
	h, err = tobj.blockCache.CreateFile(options)
	suite.assert.Error(err)
	suite.assert.Nil(h)
	suite.assert.Contains(err.Error(), "Failed to create file")
}

func (suite *blockCacheTestSuite) TestOpenWithTruncate() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 5*_1MB)
	_, _ = r.Read(data)
	os.WriteFile(storagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	options = internal.OpenFileOptions{Name: fileName, Flags: os.O_TRUNC}
	h, err = tobj.blockCache.OpenFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteFileSimple() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	storagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("Hello")}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)
	suite.assert.Equal(int64(5), h.Size)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())

	node, found := h.GetValue("0")
	suite.assert.True(found)
	block := node.(*Block)
	suite.assert.NotNil(block)
	suite.assert.Equal(int64(0), block.id)
	suite.assert.Equal(uint64(0), block.offset)

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	storagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err = os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(5), fs.Size())

	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 5, Data: []byte("Gello")}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)
	suite.assert.Equal(int64(10), h.Size)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	storagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err = os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(10), fs.Size())

	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteFileMultiBlock() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, 5*_1MB)
	_, _ = r.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooked.Len())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	storagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(len(data)))

	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteFileMultiBlockWithOverwrite() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, 5*_1MB)
	_, _ = r.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooked.Len())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:100]}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(100, n)

	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:100]}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(100, n)

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	storagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(len(data)))

	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWritefileWithAppend() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.prefetchOnOpen = true

	path := getTestFileName(suite.T().Name())
	data := make([]byte, 13*_1MB)
	_, _ = r.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	suite.assert.NoError(err)

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(data, n)
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777})
	suite.assert.NoError(err)
	dataNew := make([]byte, 10*_1MB)
	_, _ = r.Read(data)

	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: h.Size, Data: dataNew}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(dataNew, n)
	suite.assert.Equal(h.Size, int64(len(data)+len(dataNew)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(len(data)+len(dataNew)))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestWriteBlockOutOfRange() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.prefetchOnOpen = true
	tobj.blockCache.blockSize = 10

	path := getTestFileName(suite.T().Name())
	data := make([]byte, 20*_1MB)
	_, _ = r.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)

	dataNew := make([]byte, 1*_1MB)
	_, _ = r.Read(data)

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 10 * 50001, Data: dataNew}) // 5 bytes
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "block index out of range")
	suite.assert.Equal(0, n)

	tobj.blockCache.blockSize = 1048576
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 10 * 50001, Data: dataNew}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Len(dataNew, n)

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestDeleteAndRenameDirAndFile() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	err = tobj.blockCache.CreateDir(internal.CreateDirOptions{Name: "testCreateDir", Mode: 0777})
	suite.assert.NoError(err)

	options := internal.CreateFileOptions{Name: "testCreateDir/a.txt", Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("Hello")}) // 5 bytes
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)
	suite.assert.Equal(int64(5), h.Size)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	err = tobj.blockCache.RenameDir(internal.RenameDirOptions{Src: "testCreateDir", Dst: "testCreateDirNew"})
	suite.assert.NoError(err)

	err = tobj.blockCache.DeleteDir(internal.DeleteDirOptions{Name: "testCreateDirNew"})
	suite.assert.Error(err)

	err = os.MkdirAll(filepath.Join(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew")), 0777)
	suite.assert.NoError(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::0"), []byte("Hello"), 0777)
	suite.assert.NoError(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::1"), []byte("Hello"), 0777)
	suite.assert.NoError(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::2"), []byte("Hello"), 0777)
	suite.assert.NoError(err)

	err = tobj.blockCache.RenameFile(internal.RenameFileOptions{Src: "testCreateDirNew/a.txt", Dst: "testCreateDirNew/b.txt"})
	suite.assert.NoError(err)

	err = tobj.blockCache.DeleteFile(internal.DeleteFileOptions{Name: "testCreateDirNew/b.txt"})
	suite.assert.NoError(err)

	err = tobj.blockCache.DeleteDir(internal.DeleteDirOptions{Name: "testCreateDirNew"})
	suite.assert.NoError(err)
}

func (suite *blockCacheTestSuite) TestTempCacheCleanup() {
	tobj, _ := setupPipeline("")
	defer tobj.cleanupPipeline()

	items, _ := os.ReadDir(tobj.disk_cache_path)
	suite.assert.Empty(items)
	_ = common.TempCacheCleanup(tobj.blockCache.tmpPath)

	for i := range 5 {
		_ = os.Mkdir(filepath.Join(tobj.disk_cache_path, fmt.Sprintf("temp_%d", i)), 0777)
		for j := range 5 {
			_, _ = os.Create(filepath.Join(tobj.disk_cache_path, fmt.Sprintf("temp_%d", i), fmt.Sprintf("temp_%d", j)))
		}
	}

	items, _ = os.ReadDir(tobj.disk_cache_path)
	suite.assert.Equal(5, len(items))

	_ = common.TempCacheCleanup(tobj.blockCache.tmpPath)
	items, _ = os.ReadDir(tobj.disk_cache_path)
	suite.assert.Empty(items)

	tobj.blockCache.tmpPath = ""
	_ = common.TempCacheCleanup(tobj.blockCache.tmpPath)
}

func (suite *blockCacheTestSuite) TestZZZZLazyWrite() {
	tobj, _ := setupPipeline("")
	defer tobj.cleanupPipeline()

	tobj.blockCache.lazyWrite = true

	file := getTestFileName(suite.T().Name())
	handle, _ := tobj.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 10*1024*1024)
	_, _ = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	_ = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// As lazy write is enabled flush shall not upload the file
	suite.assert.True(handle.Dirty())

	_ = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	time.Sleep(5 * time.Second)
	tobj.blockCache.lazyWrite = false

	// As lazy write is enabled flush shall not upload the file
	suite.assert.False(handle.Dirty())
}

func computeMD5(fh *os.File) ([]byte, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, fh); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func (suite *blockCacheTestSuite) TestRandomWriteSparseFile() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 1MB data at offset 0
	n, err := fh.WriteAt(dataBuff[:_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 9*_1MB
	n, err = fh.WriteAt(dataBuff[4*_1MB:], int64(9*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 5*_1MB
	n, err = fh.WriteAt(dataBuff[2*_1MB:3*_1MB], int64(5*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 1MB data at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())

	// write 1MB data at offset 9*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(9 * _1MB), Data: dataBuff[4*_1MB:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 5*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(5 * _1MB), Data: dataBuff[2*_1MB : 3*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(10*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestRandomWriteSparseFileWithPartialBlock() {
	cfg := "block_cache:\n  block-size-mb: 4\n  mem-size-mb: 100\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 1MB data at offset 0
	n, err := fh.WriteAt(dataBuff[:_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 18*_1MB
	n, err = fh.WriteAt(dataBuff[4*_1MB:], int64(18*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 9*_1MB
	n, err = fh.WriteAt(dataBuff[2*_1MB:3*_1MB], int64(9*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 1MB data at offset 0
	// partial block where it has data only from 0 to 1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())

	// write 1MB data at offset 9*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(9 * _1MB), Data: dataBuff[2*_1MB : 3*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 18*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(18 * _1MB), Data: dataBuff[4*_1MB:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(19*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestRandomWriteSparseFileWithBlockOverlap() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 1MB data at offset 0
	n, err := fh.WriteAt(dataBuff[:_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 9*_1MB
	n, err = fh.WriteAt(dataBuff[4*_1MB:], int64(9*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 5.5*_1MB
	n, err = fh.WriteAt(dataBuff[2*_1MB:3*_1MB], int64(5*_1MB+1024*512))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 1MB data at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())

	// write 1MB data at offset 9*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(9 * _1MB), Data: dataBuff[4*_1MB:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 5*_1MB
	// data is written to last 0.5MB of block 5 and first 0.5MB of block 6
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(5*_1MB + 1024*512), Data: dataBuff[2*_1MB : 3*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(10*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestRandomWriteFileOneBlock() {
	cfg := "block_cache:\n  block-size-mb: 8\n  mem-size-mb: 100\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 2MB data at offset 4*1_MB
	n, err := fh.WriteAt(dataBuff[3*_1MB:], int64(4*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(2*_1MB))

	// write 1MB data at offset 2*_1MB
	n, err = fh.WriteAt(dataBuff[2*_1MB:3*_1MB], int64(2*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 2MB data at offset 4*1_MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(4 * _1MB), Data: dataBuff[3*_1MB:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(2*_1MB))
	suite.assert.True(h.Dirty())

	// write 1MB data at offset 2*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(2 * _1MB), Data: dataBuff[2*_1MB : 3*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(6*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestRandomWriteFlushAndOverwrite() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 1MB data at offset 0
	n, err := fh.WriteAt(dataBuff[:_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 9*_1MB
	n, err = fh.WriteAt(dataBuff[4*_1MB:], int64(9*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 5.5*_1MB
	n, err = fh.WriteAt(dataBuff[2*_1MB:3*_1MB], int64(5*_1MB+1024*512))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 18*_1MB
	n, err = fh.WriteAt(dataBuff[4*_1MB:], int64(18*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 1MB data at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())

	// write 1MB data at offset 9*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(9 * _1MB), Data: dataBuff[4*_1MB:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// flush the file
	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.NoError(err)

	// write 1MB data at offset 5.5*_1MB
	// overwriting last 0.5MB of block 5 and first 0.5MB of block 6 after flush
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(5*_1MB + 1024*512), Data: dataBuff[2*_1MB : 3*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write 1MB data at offset 18*_1MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(18 * _1MB), Data: dataBuff[4*_1MB:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(19*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestRandomWriteUncommittedBlockValidation() {
	prefetch := 12
	cfg := fmt.Sprintf("block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: %v\n  parallelism: 10", prefetch)
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 62MB data
	for i := 0; i < prefetch+50; i++ {
		n, err := fh.WriteAt(dataBuff[:_1MB], int64(i*int(_1MB)))
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
	}

	// update 10 bytes at 0 offset
	n, err := fh.WriteAt(dataBuff[_1MB:_1MB+10], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)

	// update 10 bytes at 5MB offset
	n, err = fh.WriteAt(dataBuff[2*_1MB:2*_1MB+10], int64(5*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	for i := 0; i < prefetch+50; i++ {
		n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(i * int(_1MB)), Data: dataBuff[:_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
	}

	suite.assert.Equal(h.Buffers.Cooking.Len()+h.Buffers.Cooked.Len(), prefetch)

	// update 10 bytes at 0 offset
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[_1MB : _1MB+10]})
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)
	suite.assert.True(h.Dirty())

	// update 10 bytes at 5MB offset
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(5 * _1MB), Data: dataBuff[2*_1MB : 2*_1MB+10]})
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(62*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestRandomWriteExistingFile() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 5MB data
	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(5*_1MB))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(5*_1MB))

	// open new handle in read-write mode
	nh, err := tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(nh)
	suite.assert.Equal(nh.Size, int64(5*_1MB))
	suite.assert.False(h.Dirty())

	// write randomly in new handle at offset 2MB
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: nh, Offset: int64(2 * _1MB), Data: dataBuff[:10]})
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)
	suite.assert.True(nh.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: nh})
	suite.assert.NoError(err)

	fs, err = os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(5*_1MB))
}

func (suite *blockCacheTestSuite) TestPreventRaceCondition() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, _1MB)
	_, _ = r.Read(data)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// writing at offset 0 in block 0
	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:10]})
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// writing at offset 1MB in block 1
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(_1MB), Data: data[:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// writing at offset 2MB in block 2
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(2 * _1MB), Data: data[:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// writing at offset 3MB in block 3
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(3 * _1MB), Data: data[:1]})
	suite.assert.NoError(err)
	suite.assert.Equal(1, n)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())
	suite.assert.Equal(1, h.Buffers.Cooked.Len())

	// writing at offset 10 in block 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 10, Data: data[10:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB-10))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(4, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(3*_1MB+1))
}

func (suite *blockCacheTestSuite) TestBlockParallelUploadAndWrite() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, _1MB)
	_, _ = r.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// writing at offset 0 in block 0
	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:10]})
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// writing at offset 1MB in block 1
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(_1MB), Data: data[:100]})
	suite.assert.NoError(err)
	suite.assert.Equal(100, n)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// staging block 0
	err = tobj.blockCache.stageBlocks(h, 1)
	suite.assert.NoError(err)
	suite.assert.Equal(1, h.Buffers.Cooking.Len())
	suite.assert.Equal(1, h.Buffers.Cooked.Len())

	// writing at offset 10 in block 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 10, Data: data[10:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB-10))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(_1MB+100))
}

func (suite *blockCacheTestSuite) TestBlockParallelUploadAndWriteValidation() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	data := make([]byte, _1MB)
	_, _ = r.Read(data)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write at offset 0 in block 0
	n, err := fh.WriteAt(data[:10], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)

	// write at offset 1MB in block 1
	n, err = fh.WriteAt(data[:], int64(_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write at offset 2MB in block 2
	n, err = fh.WriteAt(data[:], int64(2*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write at offset 3MB in block 3
	n, err = fh.WriteAt(data[:100], int64(3*_1MB))
	suite.assert.NoError(err)
	suite.assert.Equal(100, n)

	// write at offset 1MB in block 1
	n, err = fh.WriteAt(data[10:], 10)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB-10))

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// writing at offset 0 in block 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:10]})
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(1, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// writing at offset 1MB in block 1
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(_1MB), Data: data[:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(2, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// writing at offset 2MB in block 2
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(2 * _1MB), Data: data[:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	// writing at offset 3MB in block 3
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(3 * _1MB), Data: data[:100]})
	suite.assert.NoError(err)
	suite.assert.Equal(100, n)
	suite.assert.True(h.Dirty())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())
	suite.assert.Equal(1, h.Buffers.Cooked.Len())

	// writing at offset 10 in block 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 10, Data: data[10:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB-10))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(4, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(3*_1MB+100))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestBlockParallelReadAndWriteValidation() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 3MB data at offset 0
	n, err := fh.WriteAt(dataBuff[:3*_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(3*_1MB))

	// update 1MB data at offset 0
	n, err = fh.WriteAt(dataBuff[4*_1MB:5*_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 3MB at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:3*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(3*_1MB))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	nh, err := tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(nh)
	suite.assert.Equal(nh.Size, int64(3*_1MB))
	suite.assert.False(nh.Dirty())

	// read 1MB data at offset 0
	data := make([]byte, _1MB)
	n, err = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: nh, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// update 1MB data at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: nh, Offset: 0, Data: dataBuff[4*_1MB : 5*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: nh})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(3*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestBlockOverwriteValidation() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 3MB data at offset 0
	n, err := fh.WriteAt(dataBuff[:3*_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(3*_1MB))

	// update 10 bytes data at offset 0
	n, err = fh.WriteAt(dataBuff[4*_1MB:4*_1MB+10], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(10, n)

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 3MB at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:3*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(3*_1MB))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())
	suite.assert.Equal(0, h.Buffers.Cooked.Len())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	nh, err := tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(nh)
	suite.assert.Equal(nh.Size, int64(3*_1MB))
	suite.assert.False(nh.Dirty())

	// update 5 bytes data at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: nh, Offset: 0, Data: dataBuff[4*_1MB : 4*_1MB+5]})
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)

	// update 5 bytes data at offset 5
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: nh, Offset: 5, Data: dataBuff[4*_1MB+5 : 4*_1MB+10]})
	suite.assert.NoError(err)
	suite.assert.Equal(5, n)

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: nh})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(3*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestBlockFailOverwrite() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// updating the size and adding entry in block list map to replicate the download failure of the first block
	h.Size = int64(_1MB)
	lst, _ := h.GetValue("blockList")
	listMap := lst.(map[int64]*blockInfo)
	listMap[0] = &blockInfo{
		id:        "AAAAAAAA",
		committed: true,
		size:      _1MB,
	}

	// write at offset 0 where block 0 download will fail
	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:1*_1MB]})
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "failed to download block")
	suite.assert.Equal(0, n)
	suite.assert.False(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())
}

func (suite *blockCacheTestSuite) TestBlockDownloadOffsetGreaterThanFileSize() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// updating the size to replicate the download failure
	h.Size = int64(4 * _1MB)

	data := make([]byte, _1MB)
	n, err := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	// write at offset 1MB where block 1 download will fail
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(_1MB), Data: dataBuff[:1*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(2*_1MB))
}

func (suite *blockCacheTestSuite) TestReadStagedBlock() {
	cfg := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 10"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 4MB at offset 0
	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:4*_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(4*_1MB))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(3, h.Buffers.Cooking.Len())
	suite.assert.Equal(1, h.Buffers.Cooked.Len())

	data := make([]byte, _1MB)
	n, err = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooking)
	suite.assert.Nil(h.Buffers.Cooked)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(4*_1MB))
}

func (suite *blockCacheTestSuite) TestReadUncommittedBlockValidation() {
	prefetch := 12
	cfg := fmt.Sprintf("block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: %v\n  parallelism: 10", prefetch)
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// write to local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(fh)

	// write 62MB data
	ind := uint64(0)
	for i := 0; i < prefetch+50; i++ {
		n, err := fh.WriteAt(dataBuff[ind*_1MB:(ind+1)*_1MB], int64(i*int(_1MB)))
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		ind = (ind + 1) % 5
	}

	l, err := computeMD5(fh)
	suite.assert.NoError(err)

	// ------------------------------------------------------------------
	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	ind = 0
	for i := 0; i < prefetch+50; i++ {
		n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(i * int(_1MB)), Data: dataBuff[ind*_1MB : (ind+1)*_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
		ind = (ind + 1) % 5
	}

	suite.assert.Equal(h.Buffers.Cooking.Len()+h.Buffers.Cooked.Len(), prefetch)

	// read blocks 0, 1 and 2 which are uncommitted
	data := make([]byte, 2*_1MB)
	n, err := tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 512, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(2*_1MB))
	suite.assert.Equal(data[:], dataBuff[512:2*_1MB+512])
	suite.assert.False(h.Dirty())

	// read block 4 which has been committed by the previous read
	data = make([]byte, _1MB)
	n, err = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: int64(4 * _1MB), Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.Equal(data[:], dataBuff[4*_1MB:5*_1MB])
	suite.assert.False(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(62*_1MB))

	rfh, err := os.Open(storagePath)
	suite.assert.NoError(err)

	defer func(fh *os.File) {
		err := fh.Close()
		suite.assert.NoError(err)
	}(rfh)

	r, err := computeMD5(rfh)
	suite.assert.NoError(err)

	// validate md5sum
	suite.assert.Equal(l, r)
}

func (suite *blockCacheTestSuite) TestReadUncommittedPrefetchedBlock() {
	prefetch := 12
	cfg := fmt.Sprintf("block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: %v\n  parallelism: 10", prefetch)
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(_1MB))
	suite.assert.False(h.Dirty())

	ind := uint64(1)
	for i := 1; i < prefetch+50; i++ {
		n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(i * int(_1MB)), Data: dataBuff[ind*_1MB : (ind+1)*_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
		ind = (ind + 1) % 5
	}

	suite.assert.Equal(h.Buffers.Cooking.Len()+h.Buffers.Cooked.Len(), prefetch)

	// read blocks 0, 1 and 2 where prefetched blocks 1 and 2 are uncommitted
	data := make([]byte, 2*_1MB)
	n, err = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 512, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(2*_1MB))
	suite.assert.Equal(data[:], dataBuff[512:2*_1MB+512])
	suite.assert.False(h.Dirty())

	// read block 4 which has been committed by the previous read
	data = make([]byte, _1MB)
	n, err = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: int64(4 * _1MB), Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.Equal(data[:], dataBuff[4*_1MB:5*_1MB])
	suite.assert.False(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(62*_1MB))
}

func (suite *blockCacheTestSuite) TestReadWriteBlockInParallel() {
	prefetch := 12
	cfg := fmt.Sprintf("block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: %v\n  parallelism: 1", prefetch)
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(5*_1MB))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.False(h.Dirty())

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))
	suite.assert.False(h.Dirty())

	ind := uint64(0)
	for i := 5; i < prefetch+50; i++ {
		n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(i * int(_1MB)), Data: dataBuff[ind*_1MB : (ind+1)*_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
		ind = (ind + 1) % 5
	}

	suite.assert.Equal(h.Buffers.Cooking.Len()+h.Buffers.Cooked.Len(), prefetch)

	// read blocks 0, 1 and 2
	data := make([]byte, 2*_1MB)
	n, err = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 512, Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(2*_1MB))
	suite.assert.Equal(data[:], dataBuff[512:2*_1MB+512])
	suite.assert.True(h.Dirty())

	// read blocks 4 and 5
	n, err = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: int64(4 * _1MB), Data: data})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(2*_1MB))
	suite.assert.Equal(data[:_1MB], dataBuff[4*_1MB:])
	suite.assert.Equal(data[_1MB:], dataBuff[:_1MB])
	suite.assert.False(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(fs.Size(), int64(62*_1MB))
}

func (suite *blockCacheTestSuite) TestZZZZZStreamToBlockCacheConfig() {

	free := memory.FreeMemory()
	maxbuffers := max(1, free/_1MB-1)
	common.IsStream = true
	config := fmt.Sprintf("read-only: true\n\nstream:\n  block-size-mb: 2\n  max-buffers: %d\n  buffer-size-mb: 1\n", maxbuffers)
	tobj, err := setupPipeline(config)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	if err == nil {
		suite.assert.Equal("block_cache", tobj.blockCache.Name())
		suite.assert.EqualValues(2*_1MB, tobj.blockCache.blockSize)
		suite.assert.Equal(tobj.blockCache.memSize, 1*_1MB*maxbuffers)
	}
}

func (suite *blockCacheTestSuite) TestSizeOfFileInOpen() {
	// Write-back cache is turned on by default while mounting.
	config := "block_cache:\n  block-size-mb: 1\n  mem-size-mb: 20\n  prefetch: 12\n  parallelism: 1"
	tobj, err := setupPipeline(config)
	suite.assert.NoError(err)
	defer tobj.cleanupPipeline()

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)
	localPath := filepath.Join(tobj.disk_cache_path, path)

	// ------------------------------------------------------------------
	// Create a local file
	fh, err := os.Create(localPath)
	suite.assert.NoError(err)

	// write 1MB data at offset 0
	n, err := fh.WriteAt(dataBuff[:_1MB], 0)
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))

	err = fh.Close()
	suite.assert.NoError(err)
	// ------------------------------------------------------------------
	// Create a file using Mountpoint
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// write 1MB data at offset 0
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: dataBuff[:_1MB]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	//---------------------------------------------------------------------

	//Open and close the file using the given flag in local and mountpoint and
	// check the size is same or not.
	check := func(flag int) int {
		lfh, err := os.OpenFile(localPath, flag, 0666)
		suite.assert.NoError(err)
		suite.assert.NotNil(lfh)
		err = lfh.Close()
		suite.assert.NoError(err)

		openFileOptions := internal.OpenFileOptions{Name: path, Flags: flag, Mode: 0777}
		rfh, err := tobj.blockCache.OpenFile(openFileOptions)
		suite.assert.NoError(err)
		err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: rfh})
		suite.assert.NoError(err)

		statInfoLocal, err := os.Stat(localPath)
		suite.assert.NoError(err)
		sizeInLocal := statInfoLocal.Size()

		statInfoMount, err := os.Stat(storagePath)
		suite.assert.NoError(err)
		sizeInMount := statInfoMount.Size()
		suite.assert.Equal(sizeInLocal, sizeInMount)
		return int(sizeInLocal)
	}
	size := check(os.O_WRONLY) // size of the file would be 1MB
	suite.assert.Equal(size, int(_1MB))
	size = check(os.O_TRUNC) // size of the file would be zero here.
	suite.assert.Equal(int(0), size)
}

func (suite *blockCacheTestSuite) TestStrongConsistency() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.consistency = true

	path := getTestFileName(suite.T().Name())
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	storagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(int64(0), fs.Size())
	//Generate random size of file in bytes less than 2MB

	size := rand.Intn(2097152)
	data := make([]byte, size)

	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // Write data to file
	suite.assert.NoError(err)
	suite.assert.Equal(n, size)
	suite.assert.Equal(h.Size, int64(size))

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	localPath := filepath.Join(tobj.disk_cache_path, path+"::0")

	xattrMd5sumOrg := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sumOrg)
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	_, _ = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	xattrMd5sumRead := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sumRead)
	suite.assert.NoError(err)
	suite.assert.Equal(xattrMd5sumOrg, xattrMd5sumRead)

	err = syscall.Setxattr(localPath, "user.md5sum", []byte("000"), 0)
	suite.assert.NoError(err)

	xattrMd5sum1 := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sum1)
	suite.assert.NoError(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR})
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	_, _ = tobj.blockCache.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)

	xattrMd5sum2 := make([]byte, 32)
	_, err = syscall.Getxattr(localPath, "user.md5sum", xattrMd5sum2)
	suite.assert.NoError(err)

	suite.assert.NotEqual(xattrMd5sum1, xattrMd5sum2)
}

func (suite *blockCacheTestSuite) TestReadCommittedLastBlockAfterAppends() {
	prefetch := 12
	cfg := fmt.Sprintf("block_cache:\n  block-size-mb: 1\n  mem-size-mb: 25\n  prefetch: %v\n  parallelism: 10", prefetch)
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// Jump to 13thMB offset and write 500kb of data
	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(13 * _1MB), Data: dataBuff[:(_1MB / 2)]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB/2))
	suite.assert.True(h.Dirty())

	// Write remaining data backwards so that last block is staged first
	for i := range 12 {

		n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(uint64(12-i) * _1MB), Data: dataBuff[:_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
	}

	// Now Jump to 15thMB offset and write 500kb of data
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(20 * _1MB), Data: dataBuff[:(_1MB / 2)]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB/2))
	suite.assert.True(h.Dirty())

	tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	_, err = os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(h.Size, int64((20*_1MB)+(_1MB/2)))
}

func (suite *blockCacheTestSuite) TestReadCommittedLastBlocksOverwrite() {
	prefetch := 12
	cfg := fmt.Sprintf("block_cache:\n  block-size-mb: 1\n  mem-size-mb: 12\n  prefetch: %v\n  parallelism: 10", prefetch)
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NoError(err)
	suite.assert.NotNil(tobj.blockCache)

	path := getTestFileName(suite.T().Name())
	storagePath := filepath.Join(tobj.fake_storage_path, path)

	tobj.blockCache.prefetch = 3

	// write using block cache
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.NoError(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(int64(0), h.Size)
	suite.assert.False(h.Dirty())

	// At 3MB offset write half mb data, assuming this is the last block
	n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(3 * _1MB), Data: dataBuff[:(_1MB / 2)]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB/2))
	suite.assert.True(h.Dirty())

	// Fill some data before that so that last block gets committed
	for i := int64(2); i >= 0; i-- {
		n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(uint64(i) * _1MB), Data: dataBuff[:_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
	}

	// At 10MB offset write half mb data, assuming this is the last block
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(10 * _1MB), Data: dataBuff[:(_1MB / 2)]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB/2))
	suite.assert.True(h.Dirty())

	// Fill some data before that so that last block gets committed
	for i := int64(9); i >= 7; i-- {
		n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(uint64(i) * _1MB), Data: dataBuff[:_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
	}

	// At 15MB offset write half mb data, assuming this is the last block
	n, err = tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(15 * _1MB), Data: dataBuff[:(_1MB / 2)]})
	suite.assert.NoError(err)
	suite.assert.Equal(n, int(_1MB/2))
	suite.assert.True(h.Dirty())

	// Fill some data before that so that last block gets committed
	for i := int64(14); i >= 12; i-- {
		n, err := tobj.blockCache.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(uint64(i) * _1MB), Data: dataBuff[:_1MB]})
		suite.assert.NoError(err)
		suite.assert.Equal(n, int(_1MB))
		suite.assert.True(h.Dirty())
	}

	tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})

	err = tobj.blockCache.ReleaseFile(internal.ReleaseFileOptions{Handle: h})
	suite.assert.NoError(err)

	_, err = os.Stat(storagePath)
	suite.assert.NoError(err)
	suite.assert.Equal(h.Size, int64((15*_1MB)+(_1MB/2)))
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockCacheTestSuite(t *testing.T) {
	dataBuff = make([]byte, 5*_1MB)
	_, _ = r.Read(dataBuff)

	suite.Run(t, new(blockCacheTestSuite))
}
