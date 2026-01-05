//go:build linux

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

package common

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home, _ = os.UserHomeDir()

type utilLinuxTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *utilLinuxTestSuite) TestIsMountActiveNoMount() {
	var out bytes.Buffer
	cmd := exec.Command("../blobfuse2", "unmount", "all")
	cmd.Stdout = &out
	err := cmd.Run()
	suite.assert.NoError(err)
	cmd = exec.Command("pidof", "blobfuse2")
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.Equal("exit status 1", err.Error())
	res, err := IsMountActive("/mnt/blobfuse")
	suite.assert.NoError(err)
	suite.assert.False(res)
}

func (suite *utilLinuxTestSuite) TestIsMountActiveTwoMounts() {
	var out bytes.Buffer

	// Define the file name and the content you want to write
	fileName := "config.yaml"

	lbpath := filepath.Join(home, "lbpath")
	os.MkdirAll(lbpath, 0777)
	defer os.RemoveAll(lbpath)

	content := "components:\n" +
		"  - libfuse\n" +
		"  - loopbackfs\n\n" +
		"loopbackfs:\n" +
		"  path: " + lbpath + "\n\n"

	mntdir := filepath.Join(home, "mountdir")
	os.MkdirAll(mntdir, 0777)
	defer os.RemoveAll(mntdir)

	dir, err := os.Getwd()
	suite.assert.NoError(err)
	configFile := filepath.Join(dir, "config.yaml")
	// Create or open the file. If it doesn't exist, it will be created.
	file, err := os.Create(fileName)
	suite.assert.NoError(err)
	defer file.Close() // Ensure the file is closed after we're done

	// Write the content to the file
	_, err = file.WriteString(content)
	suite.assert.NoError(err)

	err = os.Chdir("..")
	suite.assert.NoError(err)

	dir, err = os.Getwd()
	suite.assert.NoError(err)
	binary := filepath.Join(dir, "blobfuse2")
	cmd := exec.Command(binary, mntdir, "--config-file", configFile)
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)

	res, err := IsMountActive(mntdir)
	suite.assert.NoError(err)
	suite.assert.True(res)

	res, err = IsMountActive("/mnt/blobfuse")
	suite.assert.NoError(err)
	suite.assert.False(res)

	cmd = exec.Command(binary, "unmount", mntdir)
	cmd.Stdout = &out
	err = cmd.Run()
	suite.assert.NoError(err)
}

func (suite *utilLinuxTestSuite) TestGetFuseMinorVersion() {
	i := GetFuseMinorVersion()
	suite.assert.GreaterOrEqual(i, 0)
}