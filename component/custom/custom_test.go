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

package custom

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type customTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *customTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func (suite *customTestSuite) TestInitializePluginsValidPath() {
	// Direct paths to the Go plugin source files
	source1 := "../../test/sample_custom_component1/main.go"
	source2 := "../../test/sample_custom_component2/main.go"

	// Paths to the compiled .so files in the current directory
	plugin1 := "./sample_custom_component1.so"
	plugin2 := "./sample_custom_component2.so"

	// Compile the Go plugin source files into .so files
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-gcflags=all=-N -l", "-o", plugin1, source1)
	err := cmd.Run()
	suite.assert.Nil(err)
	cmd = exec.Command("go", "build", "-buildmode=plugin", "-gcflags=all=-N -l", "-o", plugin2, source2)
	err = cmd.Run()
	suite.assert.Nil(err)

	os.Setenv("BLOBFUSE_PLUGIN_PATH", plugin1+":"+plugin2)

	err = initializePlugins()
	suite.assert.Nil(err)

	// Clean up the generated .so files
	os.Remove(plugin1)
	os.Remove(plugin2)
}

func (suite *customTestSuite) TestInitializePluginsInvalidPath() {
	dummyPath := "/invalid/path/plugin1.so"
	os.Setenv("BLOBFUSE_PLUGIN_PATH", dummyPath)

	err := initializePlugins()
	suite.assert.NotNil(err)
}

func (suite *customTestSuite) TestInitializePluginsEmptyPath() {
	os.Setenv("BLOBFUSE_PLUGIN_PATH", "")

	err := initializePlugins()
	suite.assert.Nil(err)
}

func TestCustomSuite(t *testing.T) {
	suite.Run(t, new(customTestSuite))
}
