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

package distributed_cache

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type distributedCacheTestSuite struct {
	suite.Suite
	assert           *assert.Assertions
	distributedCache *DistributedCache
	mockCtrl         *gomock.Controller
	mock             *internal.MockComponent
}

func (suite *distributedCacheTestSuite) SetupTest() {
	log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	defaultConfig := "distributed_cache:\n  cache-id: mycache1\n  cache-dirs:\n    - \\tmp"
	log.Debug(defaultConfig)

	suite.setupTestHelper(defaultConfig)
}

func (suite *distributedCacheTestSuite) setupTestHelper(cfg string) error {
	suite.assert = assert.New(suite.T())

	config.ReadConfigFromReader(strings.NewReader(cfg))

	suite.mockCtrl = gomock.NewController(suite.T())
	suite.mock = internal.NewMockComponent(suite.mockCtrl)
	suite.distributedCache = NewDistributedCacheComponent().(*DistributedCache)
	suite.distributedCache.SetNextComponent(suite.mock)
	err := suite.distributedCache.Configure(true)
	if err != nil {
		return fmt.Errorf("Unable to configure distributed cache [%s]", err.Error())
	}
	return nil
}

func (suite *distributedCacheTestSuite) TearDownTest() error {
	config.ResetConfig()

	err := suite.distributedCache.Stop()
	if err != nil {
		log.Err("Unable to stop distributed cache [%s]", err.Error())
		return nil
	}

	return nil
}

func (suite *distributedCacheTestSuite) TestManadatoryConfigMissing() {
	suite.assert.EqualValues("distributed_cache", suite.distributedCache.Name())
	suite.assert.EqualValues("mycache1", suite.distributedCache.cfg.CacheID)
	suite.assert.EqualValues(1, len(suite.distributedCache.cfg.CacheDirs))
	suite.assert.EqualValues(uint8(1), suite.distributedCache.cfg.Replicas)
	suite.assert.EqualValues(uint16(30), suite.distributedCache.cfg.HeartbeatDuration)
	suite.assert.EqualValues("automatic", suite.distributedCache.cfg.CacheAccess)
	suite.assert.EqualValues(uint64(4194304), suite.distributedCache.cfg.ChunkSize)
	suite.assert.EqualValues(300, suite.distributedCache.cfg.ClustermapEpoch)
	suite.assert.EqualValues(3, suite.distributedCache.cfg.MaxMissedHeartbeats)
	suite.assert.EqualValues(1, suite.distributedCache.cfg.MinNodes)
	suite.assert.EqualValues(1, suite.distributedCache.cfg.MVsPerRv)
	suite.assert.EqualValues(uint64(80), suite.distributedCache.cfg.RebalancePercentage)
	suite.assert.EqualValues(95, suite.distributedCache.cfg.RVFullThreshold)
	suite.assert.EqualValues(80, suite.distributedCache.cfg.RVNearfullThreshold)
	suite.assert.EqualValues(false, suite.distributedCache.cfg.SafeDeletes)
	suite.assert.EqualValues(uint64(16777216), suite.distributedCache.cfg.StripeSize)

	emptyConfig := "read-only: true\n\ndistributed_cache:\n  cache-id: mycache1"
	err := suite.setupTestHelper(emptyConfig)

	suite.assert.Equal("Unable to configure distributed cache [config error in distributed_cache: [cache-dirs not set]]", err.Error())

	emptyConfig = ""
	err = suite.setupTestHelper(emptyConfig)
	suite.assert.Equal("Unable to configure distributed cache [config error in distributed_cache: [cache-id not set]]", err.Error())

	emptyConfig = "read-only: true\n\ndistributed_cache:\n  path: \\tmp"
	err = suite.setupTestHelper(emptyConfig)
	suite.assert.Equal("Unable to configure distributed cache [config error in distributed_cache: [cache-id not set]]", err.Error())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDistributedCacheTestSuite(t *testing.T) {

	suite.Run(t, new(distributedCacheTestSuite))
}
