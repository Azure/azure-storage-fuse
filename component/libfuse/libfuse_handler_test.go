//go:build unittest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"

	"github.com/stretchr/testify/suite"
)

func (suite *libfuseTestSuite) TestRootOwnership() {
	defer suite.cleanupTest()
	originalUID, originalGID := rootOwnership(suite)
	mountPath := filepath.Join(suite.T().TempDir(), "missing")
	configure := func(configuration string) {
		suite.Require().NoError(config.ReadConfigFromReader(strings.NewReader(fmt.Sprintf("mount-path: %s\n%s", mountPath, configuration))))
		suite.Require().NoError(suite.libfuse.Configure(true))
		// A missing mountpoint exercises production initialization without mounting
		// or requiring /dev/fuse. Ownership must be ready before start_fuse runs.
		suite.Require().ErrorContains(suite.libfuse.initFuse(), "failed to mount fuse")
	}
	defer configure(fmt.Sprintf("libfuse:\n  uid: %d\n  gid: %d\n", originalUID, originalGID))
	suite.stubCurrentUser(4321, 8765, nil)
	for _, test := range []struct {
		name          string
		configuration string
		uid           uint32
		gid           uint32
	}{
		{"configured", "libfuse:\n  uid: 1234\n  gid: 5678\n", 1234, 5678},
		{"current-user", "", 4321, 8765},
		{"explicit-root", "libfuse:\n  uid: 0\n  gid: 0\n", 0, 0},
	} {
		suite.Run(test.name, func() {
			configure(test.configuration)
			uid, gid := rootOwnership(suite)
			suite.Require().Equal(test.uid, uid)
			suite.Require().Equal(test.gid, gid)
		})
	}
}

// Tests the default configuration of libfuse
func (suite *libfuseTestSuite) TestDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.False(suite.libfuse.readOnly)
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(common.DefaultDirectoryPermissionBits))
	suite.assert.Equal(suite.libfuse.filePermission, uint(common.DefaultFilePermissionBits))
	suite.assert.Equal(uint32(120), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(120), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(120), suite.libfuse.negativeTimeout)
	suite.assert.False(suite.libfuse.disableWritebackCache)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)
	suite.assert.False(suite.libfuse.directIO)
	testDefaultPermissionsArgs(suite, false)
}

func (suite *libfuseTestSuite) TestConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "allow-other: true\nread-only: true\nlibfuse:\n  attribute-expiration-sec: 60\n  entry-expiration-sec: 60\n  negative-entry-expiration-sec: 60\n  fuse-trace: true\n  disable-writeback-cache: true\n  ignore-open-flags: false\n  direct-io: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	// trace should only be enabled when mounted in foreground otherwise we don't honor the option
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.True(suite.libfuse.disableWritebackCache)
	suite.assert.False(suite.libfuse.ignoreOpenFlags)
	suite.assert.True(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(fs.FileMode(0777)))
	suite.assert.Equal(suite.libfuse.filePermission, uint(fs.FileMode(0777)))
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.True(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigZero() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\nlibfuse:\n  attribute-expiration-sec: 0\n  entry-expiration-sec: 0\n  negative-entry-expiration-sec: 0\n  fuse-trace: true\n  direct-io: false\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	// trace should only be enabled when mounted in foreground otherwise we don't honor the option
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(fs.FileMode(0775)))
	suite.assert.Equal(suite.libfuse.filePermission, uint(fs.FileMode(0755)))
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.False(suite.libfuse.directIO)
}

func (suite *libfuseTestSuite) TestConfigDefaultPermission() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\nlibfuse:\n  default-permission: 0555\n  attribute-expiration-sec: 0\n  entry-expiration-sec: 0\n  negative-entry-expiration-sec: 0\n  fuse-trace: true\n  direct-io: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.True(suite.libfuse.readOnly)
	// trace should only be enabled when mounted in foreground otherwise we don't honor the option
	suite.assert.False(suite.libfuse.traceEnable)
	suite.assert.False(suite.libfuse.allowOther)
	suite.assert.False(suite.libfuse.allowRoot)
	suite.assert.Equal(suite.libfuse.dirPermission, uint(fs.FileMode(0555)))
	suite.assert.Equal(suite.libfuse.filePermission, uint(fs.FileMode(0555)))
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.True(suite.libfuse.directIO)
	testDefaultPermissionsArgs(suite, false)
}

func (suite *libfuseTestSuite) TestConfigDefaultPermissions() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	suite.setupTestHelper("libfuse:\n  default-permissions: true\n  default-permission: 0600\n")
	suite.assert.Equal(uint(0600), suite.libfuse.filePermission)
	testDefaultPermissionsArgs(suite, true)
}

func (suite *libfuseTestSuite) TestLegacyConfigDefaultPermissions() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	suite.setupTestHelper("lfuse:\n  default-permissions: true\n")
	testDefaultPermissionsArgs(suite, true)
}

func (suite *libfuseTestSuite) TestConfigDisableKernelCache() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "read-only: true\ndisable-kernel-cache: true\n\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	suite.assert.Equal(uint32(0), suite.libfuse.entryExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.attributeExpiration)
	suite.assert.Equal(uint32(0), suite.libfuse.negativeTimeout)
	suite.assert.True(suite.libfuse.directIO)
}

// stubCurrentUser makes the lookup of the current user return uid, gid and err until the test ends
func (suite *libfuseTestSuite) stubCurrentUser(uid, gid uint32, err error) {
	saved := getCurrentUser
	getCurrentUser = func() (uint32, uint32, error) { return uid, gid, err }
	suite.T().Cleanup(func() { getCurrentUser = saved })
}

// TestConfigCurrentUser verifies that files are owned by the current user unless uid or gid is configured
func (suite *libfuseTestSuite) TestConfigCurrentUser() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	suite.stubCurrentUser(1234, 5678, nil)
	suite.setupTestHelper(emptyConfig)
	suite.assert.Equal(uint32(1234), suite.libfuse.ownerUID)
	suite.assert.Equal(uint32(5678), suite.libfuse.ownerGID)

	suite.cleanupTest()
	suite.stubCurrentUser(0, 0, errors.New("the lookup must not be needed"))
	suite.setupTestHelper("libfuse:\n  uid: 42\n  gid: 43\n")
	suite.assert.Equal(uint32(42), suite.libfuse.ownerUID)
	suite.assert.Equal(uint32(43), suite.libfuse.ownerGID)
}

// TestConfigCurrentUserLookupFails verifies that a failed lookup of the current user falls back to the uid and gid of
// the process, and that the rest of the configuration is still applied
func (suite *libfuseTestSuite) TestConfigCurrentUserLookupFails() {
	for _, tc := range []struct {
		config        string
		maxBackground uint32
	}{
		{emptyConfig, defaultMaxBackground},
		{"libfuse:\n  max-fuse-threads: 64\n", 64},
	} {
		suite.cleanupTest()
		suite.stubCurrentUser(1234, 5678, errors.New("user: unknown userid"))
		suite.setupTestHelper(tc.config)

		suite.assert.Equal(uint32(os.Getuid()), suite.libfuse.ownerUID, tc.config)
		suite.assert.Equal(uint32(os.Getgid()), suite.libfuse.ownerGID, tc.config)
		suite.assert.Equal(tc.maxBackground, suite.libfuse.maxBackground, tc.config)
	}
	suite.cleanupTest()
}

func (suite *libfuseTestSuite) TestConfigFuseTraceEnable() {
	defer suite.cleanupTest()
	suite.cleanupTest() // clean up the default libfuse generated
	config := "foreground: true\nlibfuse:\n  fuse-trace: true\n"

	// Foreground mount option is global config option which is exported to others using a global variable.
	// Hence setting the option before starting the test.
	common.ForegroundMount = true
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)

	suite.assert.Equal("libfuse", suite.libfuse.Name())
	suite.assert.Empty(suite.libfuse.mountPath)
	// Fuse trace should work as we are mounting using foreground option.
	suite.assert.True(suite.libfuse.traceEnable)
	common.ForegroundMount = false
}

func (suite *libfuseTestSuite) TestDisableWritebackCache() {
	defer suite.cleanupTest()
	suite.assert.False(suite.libfuse.disableWritebackCache)

	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  disable-writeback-cache: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.disableWritebackCache)

	suite.cleanupTest() // clean up the default libfuse generated
	config = "libfuse:\n  disable-writeback-cache: false\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.False(suite.libfuse.disableWritebackCache)
}

func (suite *libfuseTestSuite) TestIgnoreAppendFlag() {
	defer suite.cleanupTest()
	suite.assert.True(suite.libfuse.ignoreOpenFlags)

	suite.cleanupTest() // clean up the default libfuse generated
	config := "libfuse:\n  ignore-open-flags: false\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.False(suite.libfuse.ignoreOpenFlags)

	suite.cleanupTest() // clean up the default libfuse generated
	config = "libfuse:\n  ignore-open-flags: true\n"
	suite.setupTestHelper(config) // setup a new libfuse with a custom config (clean up will occur after the test as usual)
	suite.assert.True(suite.libfuse.ignoreOpenFlags)
}

// getattr

func (suite *libfuseTestSuite) TestMkDir() {
	testMkDir(suite)
}

func (suite *libfuseTestSuite) TestMkDirError() {
	testMkDirError(suite)
}

// readdir

func (suite *libfuseTestSuite) TestReaddir() {
	testReaddir(suite)
}

func (suite *libfuseTestSuite) TestReaddirError() {
	testReaddirError(suite)
}

func (suite *libfuseTestSuite) TestRmDir() {
	testRmDir(suite)
}

func (suite *libfuseTestSuite) TestRmDirNotEmpty() {
	testRmDirNotEmpty(suite)
}

func (suite *libfuseTestSuite) TestRmDirError() {
	testRmDirError(suite)
}

func (suite *libfuseTestSuite) TestRmDirInvalidPath() {
	testRmDirInvalidPath(suite)
}

func (suite *libfuseTestSuite) TestCreate() {
	testCreate(suite)
}

func (suite *libfuseTestSuite) TestCreateNativeIO() {
	testCreateNativeIO(suite)
}

func (suite *libfuseTestSuite) TestNativeWriteDirty() {
	testNativeWriteDirty(suite)
}

func (suite *libfuseTestSuite) TestFlushNativeDirty() {
	testFlushNativeDirty(suite)
}

func (suite *libfuseTestSuite) TestFlushNativeWriteDuringUpload() {
	testFlushNativeWriteDuringUpload(suite)
}

func (suite *libfuseTestSuite) TestFsyncNativeWrite() {
	testFsyncNativeWrite(suite)
}

func (suite *libfuseTestSuite) TestReleaseError() {
	testReleaseError(suite)
}

func (suite *libfuseTestSuite) TestNativeDirtyHandOver() {
	testNativeDirtyHandOver(suite)
}

func (suite *libfuseTestSuite) TestNativeWriteDuringHandOver() {
	testNativeWriteDuringHandOver(suite)
}

func (suite *libfuseTestSuite) TestCreateError() {
	testCreateError(suite)
}

func (suite *libfuseTestSuite) TestCreateUpdatesStats() {
	testCreateUpdatesStats(suite)
}

func (suite *libfuseTestSuite) TestCreateErrorDoesNotUpdateStats() {
	testCreateErrorDoesNotUpdateStats(suite)
}

func (suite *libfuseTestSuite) TestCreateInvalidArgument() {
	testCreateInvalidArgument(suite)
}

func (suite *libfuseTestSuite) TestGetAttrInvalidArgument() {
	testGetAttrInvalidArgument(suite)
}

func (suite *libfuseTestSuite) TestOpen() {
	testOpen(suite)
}

func (suite *libfuseTestSuite) TestOpenSyncDirectFlag() {
	testOpenSyncDirectFlag(suite)
}

func (suite *libfuseTestSuite) TestOpenAppendFlagDefault() {
	testOpenAppendFlagDefault(suite)
}

func (suite *libfuseTestSuite) TestOpenAppendFlagDisableWritebackCache() {
	testOpenAppendFlagDisableWritebackCache(suite)
}

func (suite *libfuseTestSuite) TestOpenAppendFlagIgnoreAppendFlag() {
	testOpenAppendFlagIgnoreAppendFlag(suite)
}

func (suite *libfuseTestSuite) TestOpenNotExists() {
	testOpenNotExists(suite)
}

func (suite *libfuseTestSuite) TestOpenError() {
	testOpenError(suite)
}

// read, write

func (suite *libfuseTestSuite) TestPipelineReadWrite() {
	testPipelineReadWrite(suite)
}

// flush

func (suite *libfuseTestSuite) TestTruncate() {
	testTruncate(suite)
}

func (suite *libfuseTestSuite) TestTruncateError() {
	testTruncateError(suite)
}

func (suite *libfuseTestSuite) TestFTruncate() {
	testFTruncate(suite)
}

func (suite *libfuseTestSuite) TestFTruncateError() {
	testFTruncateError(suite)
}

// release

func (suite *libfuseTestSuite) TestUnlink() {
	testUnlink(suite)
}

func (suite *libfuseTestSuite) TestUnlinkNotExists() {
	testUnlinkNotExists(suite)
}

func (suite *libfuseTestSuite) TestUnlinkError() {
	testUnlinkError(suite)
}

// rename

func (suite *libfuseTestSuite) TestRenameDirEnametoolong() {
	testRenameDirEnametoolong(suite)
}

func (suite *libfuseTestSuite) TestRenameInvalidPath() {
	testRenameInvalidPath(suite)
}

func (suite *libfuseTestSuite) TestRenameBackslashName() {
	testRenameBackslashName(suite)
}

func (suite *libfuseTestSuite) TestRenameAttrError() {
	testRenameAttrError(suite)
}

func (suite *libfuseTestSuite) TestRenameChecks() {
	testRenameChecks(suite)
}

func (suite *libfuseTestSuite) TestRenameSamePath() {
	testRenameSamePath(suite)
}

func (suite *libfuseTestSuite) TestBackslashNameRejected() {
	testBackslashNameRejected(suite)
}

func (suite *libfuseTestSuite) TestSymlinkBackslashTarget() {
	testSymlinkBackslashTarget(suite)
}

func (suite *libfuseTestSuite) TestSymlink() {
	testSymlink(suite)
}

func (suite *libfuseTestSuite) TestSymlinkError() {
	testSymlinkError(suite)
}

func (suite *libfuseTestSuite) TestReadLink() {
	testReadLink(suite)
}

func (suite *libfuseTestSuite) TestReadLinkEmpty() {
	testReadLinkEmpty(suite)
}

func (suite *libfuseTestSuite) TestReadLinkMaxTarget() {
	testReadLinkMaxTarget(suite)
}

func (suite *libfuseTestSuite) TestReadLinkNotExists() {
	testReadLinkNotExists(suite)
}

func (suite *libfuseTestSuite) TestReadLinkGetAttrError() {
	testReadLinkGetAttrError(suite)
}

func (suite *libfuseTestSuite) TestReadLinkNilAttr() {
	testReadLinkNilAttr(suite)
}

func (suite *libfuseTestSuite) TestReadLinkError() {
	testReadLinkError(suite)
}

func (suite *libfuseTestSuite) TestReadLinkTargetTooLong() {
	testReadLinkTargetTooLong(suite)
}

func (suite *libfuseTestSuite) TestReadLinkTargetExceedsBuffer() {
	testReadLinkTargetExceedsBuffer(suite)
}

func (suite *libfuseTestSuite) TestReadLinkTargetLongerThanAttr() {
	testReadLinkTargetLongerThanAttr(suite)
}

func (suite *libfuseTestSuite) TestReadLinkZeroSize() {
	testReadLinkZeroSize(suite)
}

func (suite *libfuseTestSuite) TestFsync() {
	testFsync(suite)
}

func (suite *libfuseTestSuite) TestFsyncHandleError() {
	testFsyncHandleError(suite)
}

func (suite *libfuseTestSuite) TestFsyncError() {
	testFsyncError(suite)
}

func (suite *libfuseTestSuite) TestFsyncDir() {
	testFsyncDir(suite)
}

func (suite *libfuseTestSuite) TestFsyncDirError() {
	testFsyncDirError(suite)
}

func (suite *libfuseTestSuite) TestChmod() {
	testChmod(suite)
}

func (suite *libfuseTestSuite) TestChmodNotExists() {
	testChmodNotExists(suite)
}

func (suite *libfuseTestSuite) TestStatFs() {
	testStatFs(suite)
}

func (suite *libfuseTestSuite) TestChmodError() {
	testChmodError(suite)
}

func (suite *libfuseTestSuite) TestChown() {
	testChown(suite)
}

func (suite *libfuseTestSuite) TestUtimens() {
	testUtimens(suite)
}

func (suite *libfuseTestSuite) TestKernelListCacheDefault() {
	defer suite.cleanupTest()
	suite.assert.Equal(uint32(defaultKernelListCacheTtlInSec), suite.libfuse.kernelListCacheTtlInSec)
}

func (suite *libfuseTestSuite) TestKernelListCacheConfig() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "libfuse:\n  kernel-list-cache-expiration-sec: 60\n"
	suite.setupTestHelper(config)
	suite.assert.Equal(uint32(60), suite.libfuse.kernelListCacheTtlInSec)
}

func (suite *libfuseTestSuite) TestKernelListCacheDisabledWithDirectIO() {
	defer suite.cleanupTest()
	suite.cleanupTest()
	config := "libfuse:\n  direct-io: true\n  kernel-list-cache-expiration-sec: 30\n"
	suite.setupTestHelper(config)
	suite.assert.Equal(uint32(0), suite.libfuse.kernelListCacheTtlInSec)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestLibfuseTestSuite(t *testing.T) {
	suite.Run(t, new(libfuseTestSuite))
}
