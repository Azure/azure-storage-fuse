/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

var home_dir, _ = os.UserHomeDir()

type blockCacheTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *blockCacheTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

type testObj struct {
	fake_storage_path string
	disk_cache_path   string
	loopback          internal.Component
	blockCache        *BlockCache
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func getFakeStoragePath(base string) string {
	tmp_path := filepath.Join(home_dir, base+randomString(8))
	_ = os.Mkdir(tmp_path, 0777)
	return tmp_path
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

	return nil
}

// Tests the default configuration of block cache
func (suite *blockCacheTestSuite) TestEmpty() {
	emptyConfig := "read-only: true"
	tobj, err := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")
	suite.assert.EqualValues(tobj.blockCache.blockSize, 16*_1MB)
	suite.assert.EqualValues(tobj.blockCache.diskSize, 0)
	suite.assert.EqualValues(tobj.blockCache.diskTimeout, defaultTimeout)

	cmd := exec.Command("nproc")
	output, err := cmd.Output()
	suite.assert.Nil(err)
	coresStr := strings.TrimSpace(string(output))
	cores, err := strconv.Atoi(coresStr)
	suite.assert.Nil(err)
	suite.assert.EqualValues(tobj.blockCache.workers, uint32(3*cores))
	suite.assert.EqualValues(tobj.blockCache.prefetch, uint32(2*cores))
	suite.assert.EqualValues(tobj.blockCache.noPrefetch, false)
	suite.assert.NotNil(tobj.blockCache.blockPool)
	suite.assert.NotNil(tobj.blockCache.threadPool)
}

func (suite *blockCacheTestSuite) TestMemory() {
	emptyConfig := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n"
	tobj, err := setupPipeline(emptyConfig)
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")
	cmd := exec.Command("bash", "-c", "free -b | grep Mem | awk '{print $4}'")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Nil(err)
	free, err := strconv.Atoi(strings.TrimSpace(out.String()))
	suite.assert.Nil(err)
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

	suite.assert.Nil(err)
	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")

	cmd := exec.Command("bash", "-c", fmt.Sprintf("df -B1 %s | awk 'NR==2{print $4}'", disk_cache_path))
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Nil(err)
	freeDisk, err := strconv.Atoi(strings.TrimSpace(out.String()))
	suite.assert.Nil(err)
	expected := uint64(0.8 * float64(freeDisk))
	actual := tobj.blockCache.diskSize
	difference := math.Abs(float64(actual) - float64(expected))
	tolerance := 0.10 * float64(math.Max(float64(actual), float64(expected)))
	suite.assert.LessOrEqual(difference, tolerance)
}

func (suite *blockCacheTestSuite) TestInvalidPrefetchCount() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 8\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "invalid prefetch count")
}

func (suite *blockCacheTestSuite) TestInvalidMemoryLimitPrefetchCount() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 320\n  prefetch: 50\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "[memory limit too low for configured prefetch")
}

func (suite *blockCacheTestSuite) TestNoPrefetchConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 1\n  mem-size-mb: 500\n  prefetch: 0\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)
	suite.assert.Equal(tobj.blockCache.noPrefetch, true)
}

func (suite *blockCacheTestSuite) TestInvalidDiskPath() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: /abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "permission denied")
}

func (suite *blockCacheTestSuite) TestSomeInvalidConfigs() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 0\n"
	_, err := setupPipeline(cfg)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "fail to init thread pool")

	cfg = "read-only: true\n\nblock_cache:\n  block-size-mb: 1024000\n  mem-size-mb: 20240000\n  prefetch: 12\n  parallelism: 1\n"
	_, err = setupPipeline(cfg)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "fail to init block pool")

	cfg = "read-only: true\n\nblock_cache:\n  block-size-mb: 8\n  mem-size-mb: 800\n  prefetch: 12\n  parallelism: 5\n  path: ./\n  disk-size-mb: 100\n  disk-timeout-sec: 0"
	_, err = setupPipeline(cfg)
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "timeout can not be zero")
}

func (suite *blockCacheTestSuite) TestManualConfig() {
	cfg := "read-only: true\n\nblock_cache:\n  block-size-mb: 16\n  mem-size-mb: 500\n  prefetch: 12\n  parallelism: 10\n  path: abcd\n  disk-size-mb: 100\n  disk-timeout-sec: 5"
	tobj, err := setupPipeline(cfg)
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.Equal(tobj.blockCache.Name(), "block_cache")
	suite.assert.EqualValues(tobj.blockCache.blockSize, 16*_1MB)
	suite.assert.EqualValues(tobj.blockCache.memSize, 500*_1MB)
	suite.assert.EqualValues(tobj.blockCache.workers, 10)
	suite.assert.EqualValues(tobj.blockCache.diskSize, 100*_1MB)
	suite.assert.EqualValues(tobj.blockCache.diskTimeout, 5)
	suite.assert.EqualValues(tobj.blockCache.prefetch, 12)
	suite.assert.EqualValues(tobj.blockCache.workers, 10)

	suite.assert.NotNil(tobj.blockCache.blockPool)
}

func (suite *blockCacheTestSuite) TestOpenFileFail() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "a"
	options := internal.OpenFileOptions{Name: path}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.NotNil(err)
	suite.assert.Nil(h)
	suite.assert.Contains(err.Error(), "no such file or directory")
}

func (suite *blockCacheTestSuite) TestFileOpenClose() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)
	ioutil.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileRead() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 50*_1MB)
	_, _ = rand.Read(data)
	ioutil.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(50*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 1000)

	// Read beyond end of file
	n, err := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: int64((50 * _1MB) + 1), Data: data})
	suite.assert.NotNil(err)
	suite.assert.Equal(n, 0)
	suite.assert.Contains(err.Error(), "EOF")

	// Read exactly at last offset
	n, err = tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: int64(50 * _1MB), Data: data})
	suite.assert.NotNil(err)
	suite.assert.Equal(n, 0)
	suite.assert.Contains(err.Error(), "EOF")

	n, err = tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: 0, Data: data})
	suite.assert.Nil(err)
	suite.assert.Equal(n, 1000)

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(cnt, MIN_PREFETCH*2)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadSerial() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 50*_1MB)
	_, _ = rand.Read(data)
	ioutil.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(50*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 1000)

	totaldata := uint64(0)
	for {
		n, err := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: int64(totaldata), Data: data})
		totaldata += uint64(n)
		if err != nil {
			break
		}
		suite.assert.LessOrEqual(n, 1000)
	}

	suite.assert.Equal(totaldata, uint64(50*_1MB))
	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(cnt, 12)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadRandom() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 100*_1MB)
	_, _ = rand.Read(data)
	ioutil.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(100*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 100)
	max := int64(100 * _1MB)
	for i := 0; i < 50; i++ {
		offset := rand.Int63n(max)
		n, _ := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: offset, Data: data})
		suite.assert.LessOrEqual(n, 100)
	}

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.LessOrEqual(cnt, 8)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestFileReadRandomNoPrefetch() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	// Set the no prefetch mode here
	tobj.blockCache.noPrefetch = true
	tobj.blockCache.prefetch = 0

	fileName := "bc.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 100*_1MB)
	_, _ = rand.Read(data)
	ioutil.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(100*_1MB))
	suite.assert.NotNil(h.Buffers.Cooked)
	suite.assert.NotNil(h.Buffers.Cooking)

	data = make([]byte, 100)
	max := int64(100 * _1MB)
	for i := 0; i < 50; i++ {
		offset := rand.Int63n(max)
		n, _ := tobj.blockCache.ReadInBuffer(internal.ReadInBufferOptions{Handle: h, Offset: offset, Data: data})
		suite.assert.Equal(h.Buffers.Cooked.Len(), 1)
		suite.assert.Equal(h.Buffers.Cooking.Len(), 0)
		suite.assert.LessOrEqual(n, 100)
	}

	cnt := h.Buffers.Cooked.Len() + h.Buffers.Cooking.Len()
	suite.assert.Equal(cnt, 1)

	tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(h.Buffers.Cooked)
	suite.assert.Nil(h.Buffers.Cooking)
}

func (suite *blockCacheTestSuite) TestDiskUsageCheck() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	usage, err := common.GetUsage(tobj.disk_cache_path)
	suite.assert.Nil(err)
	suite.assert.Less(usage, float64(1.0))
	suite.assert.Equal(tobj.blockCache.checkDiskUsage(), false)

	// Default disk size is 50MB
	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)

	type diskusagedata struct {
		name     string
		diskflag bool
	}

	localfiles := make([]diskusagedata, 0)
	for i := 0; i < 13; i++ {
		fname := randomString(5)
		diskFile := filepath.Join(tobj.disk_cache_path, fname)
		localfiles = append(localfiles, diskusagedata{name: diskFile, diskflag: i >= 7})
	}

	for i := 0; i < 13; i++ {
		ioutil.WriteFile(localfiles[i].name, data, 0777)
		usage, err := common.GetUsage(tobj.disk_cache_path)
		suite.assert.Nil(err)
		fmt.Printf("%d : %v (%v : %v) Usage %v\n", i, localfiles[i].name, localfiles[i].diskflag, tobj.blockCache.checkDiskUsage(), usage)
		suite.assert.Equal(tobj.blockCache.checkDiskUsage(), localfiles[i].diskflag)
	}

	for i := 0; i < 13; i++ {
		localfiles[i].diskflag = i < 8
	}

	for i := 0; i < 13; i++ {
		os.Remove(localfiles[i].name)
		usage, err := common.GetUsage(tobj.disk_cache_path)
		suite.assert.Nil(err)
		fmt.Printf("%d : %v (%v : %v) Usage %v\n", i, localfiles[i].name, localfiles[i].diskflag, tobj.blockCache.checkDiskUsage(), usage)
		suite.assert.Equal(tobj.blockCache.checkDiskUsage(), localfiles[i].diskflag)
	}
}

// Block-cache Writer related test cases
func (suite *blockCacheTestSuite) TestCreateFile() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testCreate"
	options := internal.CreateFileOptions{Name: path}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(0))
	suite.assert.False(h.Dirty())

	stroagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.Nil(err)
	suite.assert.Equal(fs.Size(), int64(0))

	path = "FailThis"
	options = internal.CreateFileOptions{Name: path}
	h, err = tobj.blockCache.CreateFile(options)
	suite.assert.NotNil(err)
	suite.assert.Nil(h)
	suite.assert.Contains(err.Error(), "Failed to create file")
}

func (suite *blockCacheTestSuite) TestOpenWithTruncate() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	fileName := "testTruncate.tst"
	stroagePath := filepath.Join(tobj.fake_storage_path, fileName)
	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)
	ioutil.WriteFile(stroagePath, data, 0777)

	options := internal.OpenFileOptions{Name: fileName}
	h, err := tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(5*_1MB))

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	options = internal.OpenFileOptions{Name: fileName, Flags: os.O_TRUNC}
	h, err = tobj.blockCache.OpenFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(0))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestWriteFileSimple() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testWriteSimple"
	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(0))
	suite.assert.False(h.Dirty())

	stroagePath := filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.Nil(err)
	suite.assert.Equal(fs.Size(), int64(0))

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("Hello")}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, 5)
	suite.assert.Equal(h.Size, int64(5))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(h.Buffers.Cooked.Len(), 0)
	suite.assert.Equal(h.Buffers.Cooking.Len(), 1)

	node, found := h.GetValue("0")
	suite.assert.True(found)
	block := node.(*Block)
	suite.assert.NotNil(block)
	suite.assert.Equal(block.id, int64(0))
	suite.assert.Equal(block.offset, uint64(0))

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.Nil(err)
	suite.assert.False(h.Dirty())

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err = os.Stat(stroagePath)
	suite.assert.Nil(err)
	suite.assert.Equal(fs.Size(), int64(5))

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 5, Data: []byte("Gello")}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, 5)
	suite.assert.Equal(h.Size, int64(10))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(h.Buffers.Cooked.Len(), 0)
	suite.assert.Equal(h.Buffers.Cooking.Len(), 1)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err = os.Stat(stroagePath)
	suite.assert.Nil(err)
	suite.assert.Equal(fs.Size(), int64(10))

	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestWriteFileMultiBlock() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testWriteBlock"
	stroagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(0))
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, len(data))
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(h.Buffers.Cooked.Len(), 2)
	suite.assert.Equal(h.Buffers.Cooking.Len(), 3)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.Nil(err)
	suite.assert.Equal(fs.Size(), int64(len(data)))

	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestWriteFileMultiBlockWithOverwrite() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	path := "testWriteBlock"
	stroagePath := filepath.Join(tobj.fake_storage_path, path)

	data := make([]byte, 5*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(0))
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, len(data))
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(h.Buffers.Cooked.Len(), 2)
	suite.assert.Equal(h.Buffers.Cooking.Len(), 3)

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.Nil(err)
	suite.assert.False(h.Dirty())

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:100]}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, 100)

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data[:100]}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, 100)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	stroagePath = filepath.Join(tobj.fake_storage_path, path)
	fs, err := os.Stat(stroagePath)
	suite.assert.Nil(err)
	suite.assert.Equal(fs.Size(), int64(len(data)))

	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestWritefileWithAppend() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.prefetchOnOpen = true

	path := "testWriteBlockAppend"
	data := make([]byte, 20*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(0))
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, len(data))
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.SyncFile(internal.SyncFileOptions{Handle: h})
	suite.assert.Nil(err)

	err = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: h})
	suite.assert.Nil(err)
	suite.assert.False(h.Dirty())

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: data}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, len(data))
	suite.assert.Equal(h.Size, int64(len(data)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777})
	suite.assert.Nil(err)
	dataNew := make([]byte, 10*_1MB)
	_, _ = rand.Read(data)

	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: h.Size, Data: dataNew}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, len(dataNew))
	suite.assert.Equal(h.Size, int64(len(data)+len(dataNew)))
	suite.assert.True(h.Dirty())

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	h, err = tobj.blockCache.OpenFile(internal.OpenFileOptions{Name: path, Flags: os.O_RDWR, Mode: 0777})
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(len(data)+len(dataNew)))

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestWriteBlockOutOfRange() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	tobj.blockCache.prefetchOnOpen = true
	tobj.blockCache.blockSize = 10

	path := "testInvalidWriteBlock"
	data := make([]byte, 20*_1MB)
	_, _ = rand.Read(data)

	options := internal.CreateFileOptions{Name: path, Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.Nil(err)

	dataNew := make([]byte, 1*_1MB)
	_, _ = rand.Read(data)

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 10 * 50001, Data: dataNew}) // 5 bytes
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "block index out of range")
	suite.assert.Equal(n, 0)

	tobj.blockCache.blockSize = 1048576
	n, err = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 10 * 50001, Data: dataNew}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, len(dataNew))

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestDeleteAndRenameDirAndFile() {
	tobj, err := setupPipeline("")
	defer tobj.cleanupPipeline()

	suite.assert.Nil(err)
	suite.assert.NotNil(tobj.blockCache)

	err = tobj.blockCache.CreateDir(internal.CreateDirOptions{Name: "testCreateDir", Mode: 0777})
	suite.assert.Nil(err)

	options := internal.CreateFileOptions{Name: "testCreateDir/a.txt", Mode: 0777}
	h, err := tobj.blockCache.CreateFile(options)
	suite.assert.Nil(err)
	suite.assert.NotNil(h)
	suite.assert.Equal(h.Size, int64(0))
	suite.assert.False(h.Dirty())

	n, err := tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("Hello")}) // 5 bytes
	suite.assert.Nil(err)
	suite.assert.Equal(n, 5)
	suite.assert.Equal(h.Size, int64(5))
	suite.assert.True(h.Dirty())
	suite.assert.Equal(h.Buffers.Cooked.Len(), 0)
	suite.assert.Equal(h.Buffers.Cooking.Len(), 1)

	err = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: h})
	suite.assert.Nil(err)

	err = tobj.blockCache.RenameDir(internal.RenameDirOptions{Src: "testCreateDir", Dst: "testCreateDirNew"})
	suite.assert.Nil(err)

	err = tobj.blockCache.DeleteDir(internal.DeleteDirOptions{Name: "testCreateDirNew"})
	suite.assert.NotNil(err)

	err = os.MkdirAll(filepath.Join(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew")), 0777)
	suite.assert.Nil(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::0"), []byte("Hello"), 0777)
	suite.assert.Nil(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::1"), []byte("Hello"), 0777)
	suite.assert.Nil(err)
	err = os.WriteFile(filepath.Join(tobj.blockCache.tmpPath, "testCreateDirNew/a.txt::2"), []byte("Hello"), 0777)
	suite.assert.Nil(err)

	err = tobj.blockCache.RenameFile(internal.RenameFileOptions{Src: "testCreateDirNew/a.txt", Dst: "testCreateDirNew/b.txt"})
	suite.assert.Nil(err)

	err = tobj.blockCache.DeleteFile(internal.DeleteFileOptions{Name: "testCreateDirNew/b.txt"})
	suite.assert.Nil(err)

	err = tobj.blockCache.DeleteDir(internal.DeleteDirOptions{Name: "testCreateDirNew"})
	suite.assert.Nil(err)
}

func (suite *blockCacheTestSuite) TestTempCacheCleanup() {
	tobj, _ := setupPipeline("")
	defer tobj.cleanupPipeline()

	items, _ := os.ReadDir(tobj.disk_cache_path)
	suite.assert.Equal(len(items), 0)
	_ = tobj.blockCache.TempCacheCleanup()

	for i := 0; i < 5; i++ {
		_ = os.Mkdir(filepath.Join(tobj.disk_cache_path, fmt.Sprintf("temp_%d", i)), 0777)
		for j := 0; j < 5; j++ {
			_, _ = os.Create(filepath.Join(tobj.disk_cache_path, fmt.Sprintf("temp_%d", i), fmt.Sprintf("temp_%d", j)))
		}
	}

	items, _ = os.ReadDir(tobj.disk_cache_path)
	suite.assert.Equal(len(items), 5)

	_ = tobj.blockCache.TempCacheCleanup()
	items, _ = os.ReadDir(tobj.disk_cache_path)
	suite.assert.Equal(len(items), 0)

	tobj.blockCache.tmpPath = ""
	_ = tobj.blockCache.TempCacheCleanup()

	tobj.blockCache.tmpPath = "~/ABCD"
	err := tobj.blockCache.TempCacheCleanup()
	suite.assert.NotNil(err)
	suite.assert.Contains(err.Error(), "failed to list directory")
}

func (suite *blockCacheTestSuite) TestZZZZLazyWrite() {
	tobj, _ := setupPipeline("")
	defer tobj.cleanupPipeline()

	tobj.blockCache.lazyWrite = true

	file := "file101"
	handle, _ := tobj.blockCache.CreateFile(internal.CreateFileOptions{Name: file, Mode: 0777})
	data := make([]byte, 10*1024*1024)
	_, _ = tobj.blockCache.WriteFile(internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data})
	_ = tobj.blockCache.FlushFile(internal.FlushFileOptions{Handle: handle})

	// As lazy write is enabled flush shall not upload the file
	suite.assert.True(handle.Dirty())

	_ = tobj.blockCache.CloseFile(internal.CloseFileOptions{Handle: handle})
	time.Sleep(5 * time.Second)
	tobj.blockCache.lazyWrite = false

	// As lazy write is enabled flush shall not upload the file
	suite.assert.False(handle.Dirty())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestBlockCacheTestSuite(t *testing.T) {
	bcsuite := new(blockCacheTestSuite)
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}

	suite.Run(t, bcsuite)
}
