package distributed_cache

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type utilsTestSuite struct {
	suite.Suite
}

func (suite *utilsTestSuite) TestGetVmIp() {
	assert := assert.New(suite.T())
	originalGetNetAddrs := getNetAddrs
	defer func() { getNetAddrs = originalGetNetAddrs }()

	getNetAddrs = func() ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.IPv4(192, 168, 1, 1)},
		}, nil
	}

	ip, _ := getVmIp()
	assert.Equal("192.168.1.1", ip)

	getNetAddrs = func() ([]net.Addr, error) {
		return []net.Addr{
			&net.IPNet{IP: net.IPv4(127, 0, 0, 1)},
		}, nil
	}
	_, err := getVmIp()
	assert.Equal("unable to find a valid non-loopback IPv4 address", err.Error())

	getNetAddrs = func() ([]net.Addr, error) {
		return nil, fmt.Errorf("mock error")
	}

	_, err = getVmIp()
	assert.Equal("mock error", err.Error())
}

func (suite *utilsTestSuite) TestEvaluateVMStorage() {
	assert := assert.New(suite.T())

	total, _, err := evaluateVMStorage("/mock/path")
	assert.Equal("no such file or directory", err.Error())

	pwd, err := os.Getwd()
	if err != nil {
		return
	}
	dirName := filepath.Join(pwd, "mock")
	err = os.MkdirAll(dirName, 0777)
	assert.Nil(err)

	total, free, err := evaluateVMStorage(dirName)
	assert.Nil(err)
	assert.NotEqual(0, total)
	assert.NotEqual(100, free)
	_ = os.RemoveAll(filepath.Join(pwd, "mock"))
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
