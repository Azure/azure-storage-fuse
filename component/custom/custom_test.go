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
	source1 := "../../test/sample_custom_component1"
	source2 := "../../test/sample_custom_component2"

	// Paths to the compiled .so files in the current directory
	plugin1 := "sample_custom_component1.so"
	plugin2 := "sample_custom_component2.so"

	// Compile the Go plugin source files into .so files
	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", plugin1, source1)
	err := cmd.Run()
	suite.assert.Nil(err)
	cmd = exec.Command("go", "build", "-buildmode=plugin", "-o", plugin2, source2)
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
