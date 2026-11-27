// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package custom

import (
	"os"
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

// This test builds a custom component and then tries to load the .so file
// Though both .so file and tests are being built using same go packages, it still gives an error in loading .so file
//
//	"plugin was built with a different version of package"
//
// Hence, this test is disabled for now.
// Same .so file loads fine when blobfuse is run from CLI.
// If you wish to debug blobfuse2 with a custom component then always build .so file with "-gcflags=all=-N -l" option
//
//	e.g. go build -buildmode=plugin -gcflags=all=-N -l -o x.so <path to source>
//
// This flag disables all optimizations and inline replacements and then .so will load in debug mode as well.
// However same .so will not work with cli mount and there you need to build .so without these flags.
//
// TODO: Fix this test, commenting it out for now to pass the lint checks.
//
// func (suite *customTestSuite) _TestInitializePluginsValidPath() {
// 	// Direct paths to the Go plugin source files
// 	source1 := "../../test/sample_custom_component1/main.go"
// 	source2 := "../../test/sample_custom_component2/main.go"

// 	// Paths to the compiled .so files in the current directory
// 	plugin1 := "./sample_custom_component1.so"
// 	plugin2 := "./sample_custom_component2.so"

// 	// Compile the Go plugin source files into .so files
// 	cmd := exec.Command("go", "build", "-buildmode=plugin", "-gcflags=all=-N -l", "-o", plugin1, source1)
// 	err := cmd.Run()
// 	suite.assert.NoError(err)
// 	cmd = exec.Command("go", "build", "-buildmode=plugin", "-gcflags=all=-N -l", "-o", plugin2, source2)
// 	err = cmd.Run()
// 	suite.assert.NoError(err)

// 	os.Setenv("BLOBFUSE_PLUGIN_PATH", plugin1+":"+plugin2)

// 	err = initializePlugins()
// 	suite.assert.NoError(err)

// 	// Clean up the generated .so files
// 	os.Remove(plugin1)
// 	os.Remove(plugin2)
// }

func (suite *customTestSuite) TestInitializePluginsInvalidPath() {
	dummyPath := "/invalid/path/plugin1.so"
	os.Setenv("BLOBFUSE_PLUGIN_PATH", dummyPath)

	err := initializePlugins()
	suite.assert.Error(err)
}

func (suite *customTestSuite) TestInitializePluginsEmptyPath() {
	os.Setenv("BLOBFUSE_PLUGIN_PATH", "")

	err := initializePlugins()
	suite.assert.NoError(err)
}

func TestCustomSuite(t *testing.T) {
	suite.Run(t, new(customTestSuite))
}
