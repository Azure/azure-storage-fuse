// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cpu_mem_profiler

import (
	"fmt"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type cpuMemMonitorTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *cpuMemMonitorTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *cpuMemMonitorTestSuite) TestGetCpuMemoryUsage() {
	cm := &CpuMemProfiler{
		name:         hmcommon.CpuMemoryProfiler,
		pid:          fmt.Sprintf("%v", os.Getpid()),
		pollInterval: 5,
	}

	c, err := cm.getCpuMemoryUsage()
	suite.assert.NotNil(c)
	suite.assert.NoError(err)
}

func (suite *cpuMemMonitorTestSuite) TestGetCpuMemoryUsageFailure() {
	cm := &CpuMemProfiler{
		name:         hmcommon.CpuMemoryProfiler,
		pid:          "abcd",
		pollInterval: 5,
	}

	c, err := cm.getCpuMemoryUsage()
	suite.assert.Nil(c)
	suite.assert.Error(err)
}

func TestCpuMemMonitor(t *testing.T) {
	suite.Run(t, new(cpuMemMonitorTestSuite))
}
