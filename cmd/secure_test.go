// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type secureConfigTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *secureConfigTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic("Unable to set silent logger as default.")
	}
}

func (suite *secureConfigTestSuite) cleanupTest() {
	resetSecureCLIFlags()
}

func executeCommandSecure(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()

	return buf.String(), err
}

func resetSecureCLIFlags() {
	generateConfigCmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
	})
}

func TestSecureConfig(t *testing.T) {
	suite.Run(t, new(secureConfigTestSuite))
}

func (suite *secureConfigTestSuite) TestHelp() {
	defer suite.cleanupTest()
	_, err := executeCommandSecure(rootCmd, "secure", "-h")
	suite.assert.NoError(err)
}

var testPlainTextConfig string = `
foreground: false
read-only: true
allow-other: true

logging:  
  type: base
  level: log_debug
  file-path: /home/blobfuse2.log
  max-file-size: 100
  file-count: 300
  track-time: true

components:
  - libfuse
  - file_cache
  - attr_cache
  - azstorage

libfuse:
  attribute-expiration-sec: 1
  entry-expiration-sec: 1`

func (suite *secureConfigTestSuite) TestSecureConfigEncrypt() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile.Name()), "--passphrase=123123123123123123123123", fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NoError(err)
	suite.assert.NoFileExists(confFile.Name())
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptRejectsParentComponent() {
	defer suite.cleanupTest()
	t := suite.T()
	dir := privateTestDir(t)
	source := filepath.Join(dir, "plain.yaml")
	destination := filepath.Join(dir, "encrypted.azsec")
	require.NoError(t, os.WriteFile(source, []byte(testPlainTextConfig), 0600))
	require.NoError(t, os.WriteFile(destination, []byte("unchanged"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0700))

	_, err := executeCommandSecure(rootCmd, "secure", "encrypt", "--config-file="+source,
		"--passphrase=123123123123123123123123", "--output-file="+dir+"/nested/../encrypted.azsec")
	require.ErrorContains(t, err, "must not contain a '..' component")
	for path, want := range map[string]string{source: testPlainTextConfig, destination: "unchanged"} {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, want, string(data))
	}
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptNotExistent() {
	defer suite.cleanupTest()
	confFile := "abcd.yaml"
	_, err := executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile), "--passphrase=123123123123123123123123")
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptNoConfig() {
	defer suite.cleanupTest()

	_, err := executeCommandSecure(rootCmd, "secure", "encrypt")
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptNoKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigEncryptInvalidKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile.Name()), "--passphrase=123", fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigDecrypt() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile.Name()), "--passphrase=123123123123123123123123", fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NoError(err)

	decrypted := filepath.Join(privateTestDir(suite.T()), "decrypted.yaml")
	_, err = executeCommandSecure(rootCmd, "secure", "decrypt", fmt.Sprintf("--config-file=%s", outFile.Name()), "--passphrase=123123123123123123123123", "--output-file="+decrypted)
	suite.assert.NoError(err)

	info, err := os.Stat(decrypted)
	require.NoError(suite.T(), err)
	suite.assert.Equal(os.FileMode(0600), info.Mode().Perm())

	data, err := os.ReadFile(decrypted)
	suite.assert.NoError(err)

	suite.assert.Equal(testPlainTextConfig, string(data))

	os.Remove(confFile.Name() + "." + SecureConfigExtension)
}

func (suite *secureConfigTestSuite) TestSecureConfigDecryptNoConfig() {
	defer suite.cleanupTest()

	_, err := executeCommandSecure(rootCmd, "secure", "decrypt")
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigDecryptNoKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "decrypt", fmt.Sprintf("--config-file=%s", confFile.Name()))
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigGet() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile.Name()), "--passphrase=123123123123123123123123", fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "get", fmt.Sprintf("--config-file=%s", outFile.Name()), "--passphrase=123123123123123123123123", "--key=logging.level")
	suite.assert.NoError(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigGetInvalidKey() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile.Name()), "--passphrase=123123123123123123123123", fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "get", fmt.Sprintf("--config-file=%s", outFile.Name()), "--passphrase=123123123123123123123123", "--key=abcd.efg")
	suite.assert.Error(err)
}

func (suite *secureConfigTestSuite) TestSecureConfigSet() {
	defer suite.cleanupTest()
	confFile, _ := os.CreateTemp("", "conf*.yaml")
	outFile, _ := os.CreateTemp("", "conf*.yaml")

	defer os.Remove(confFile.Name())
	defer os.Remove(outFile.Name())

	_, err := confFile.WriteString(testPlainTextConfig)
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "encrypt", fmt.Sprintf("--config-file=%s", confFile.Name()), "--passphrase=123123123123123123123123", fmt.Sprintf("--output-file=%s", outFile.Name()))
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "get", fmt.Sprintf("--config-file=%s", outFile.Name()), "--passphrase=123123123123123123123123", "--key=logging.level")
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "set", fmt.Sprintf("--config-file=%s", outFile.Name()), "--passphrase=123123123123123123123123", "--key=logging.level", "--value=log_err")
	suite.assert.NoError(err)

	_, err = executeCommandSecure(rootCmd, "secure", "get", fmt.Sprintf("--config-file=%s", outFile.Name()), "--passphrase=123123123123123123123123", "--key=logging.level")
	suite.assert.NoError(err)

	t := suite.T()
	before, err := os.ReadFile(outFile.Name())
	require.NoError(t, err)
	require.NoError(t, os.Chmod(outFile.Name(), 0400))
	_, err = executeCommandSecure(rootCmd, "secure", "set", fmt.Sprintf("--config-file=%s", outFile.Name()), "--passphrase=123123123123123123123123", "--key=logging.level", "--value=log_debug")
	require.ErrorContains(t, err, "owner write permission is not set")
	after, err := os.ReadFile(outFile.Name())
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
