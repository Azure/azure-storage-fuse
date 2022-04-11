// +build !unittest

package mount_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

var blobfuseBinary string = "blobfuse2"
var mntDir string = "mntdir"
var configFile string

type mountSuite struct {
	suite.Suite
}

func remountCheck(suite *mountSuite) {
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "directory is already mounted")
}

// list blobfuse mounted directories
func listBlobfuseMounts(suite *mountSuite) []byte {
	mntListCmd := exec.Command(blobfuseBinary, "mount", "list")
	cliOut, err := mntListCmd.Output()
	fmt.Println(string(cliOut))
	suite.Equal(nil, err)
	return cliOut
}

// unmount blobfuse
func blobfuseUnmount(suite *mountSuite, unmountOutput string) {
	unmountCmd := exec.Command(blobfuseBinary, "unmount", "all")
	cliOut, err := unmountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.Equal(nil, err)
	suite.Contains(string(cliOut), unmountOutput)

	// wait after unmount
	time.Sleep(5 * time.Second)

	// validate unmount
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))
}

// mount command test along with remount on the same path
func (suite *mountSuite) TestMountCmd() {
	// run mount command
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.Equal(0, len(cliOut))
	suite.Equal(nil, err)

	// wait for mount
	time.Sleep(10 * time.Second)

	// validate mount
	cliOut = listBlobfuseMounts(suite)
	suite.NotEqual(0, len(cliOut))
	suite.Contains(string(cliOut), mntDir)

	remountCheck(suite)

	// unmount
	blobfuseUnmount(suite, mntDir)
}

// mount failure test where the mount directory does not exists
func (suite *mountSuite) TestMountDirNotExists() {
	tempDir := filepath.Join(mntDir, "tempdir")
	mountCmd := exec.Command(blobfuseBinary, "mount", tempDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "mount directory does not exists")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test where the mount directory is not empty
func (suite *mountSuite) TestMountDirNotEmpty() {
	tempDir := filepath.Join(mntDir, "tempdir")
	os.Mkdir(tempDir, 0777)
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "mount directory is not empty")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	os.RemoveAll(tempDir)

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test where the mount path is not provided
func (suite *mountSuite) TestMountPathNotProvided() {
	mountCmd := exec.Command(blobfuseBinary, "mount", "", "--config-file="+configFile)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "mount path not provided")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test where config file is not provided
func (suite *mountSuite) TestConfigFileNotProvided() {
	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir)
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(0, len(cliOut))
	suite.NotEqual(nil, err)
	suite.Contains(string(cliOut), "failed to mount")

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")
}

// mount failure test using environment variables for mounting
func (suite *mountSuite) TestEnvVarMount() {
	tempDir := filepath.Join(mntDir, "..", "tempdir")
	os.Mkdir(tempDir, 0777)

	// create environment variables
	os.Setenv("AZURE_STORAGE_ACCOUNT", "myAccount")
	os.Setenv("AZURE_STORAGE_ACCESS_KEY", "myKey")
	os.Setenv("AZURE_STORAGE_BLOB_ENDPOINT", "https://myAccount.dfs.core.windows.net")

	mountCmd := exec.Command(blobfuseBinary, "mount", mntDir, "--tmp-path="+tempDir, "--container-name=myContainer")
	cliOut, err := mountCmd.Output()
	fmt.Println(string(cliOut))
	suite.NotEqual(nil, err)

	// list blobfuse mounted directories
	cliOut = listBlobfuseMounts(suite)
	suite.Equal(0, len(cliOut))

	// unmount
	blobfuseUnmount(suite, "nothing to unmount")

	os.Unsetenv("AZURE_STORAGE_ACCOUNT")
	os.Unsetenv("AZURE_STORAGE_ACCESS_KEY")
	os.Unsetenv("AZURE_STORAGE_BLOB_ENDPOINT")

	os.RemoveAll(tempDir)
}

func TestMountSuite(t *testing.T) {
	suite.Run(t, new(mountSuite))
}

func TestMain(m *testing.M) {
	workingDirPtr := flag.String("working-dir", "", "Directory containing the blobfuse binary")
	pathPtr := flag.String("mnt-path", ".", "Mount Path of Container")
	configPtr := flag.String("config-file", "", "Config file for mounting")

	flag.Parse()

	blobfuseBinary = filepath.Join(*workingDirPtr, blobfuseBinary)
	mntDir = filepath.Join(*pathPtr, mntDir)
	configFile = *configPtr

	err := os.RemoveAll(mntDir)
	if err != nil {
		fmt.Println("Could not cleanup mount directory before testing")
	}
	os.Mkdir(mntDir, 0777)

	m.Run()

	os.RemoveAll(mntDir)
}
