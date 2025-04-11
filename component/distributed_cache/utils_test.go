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
	"net"
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

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
