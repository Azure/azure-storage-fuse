// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loopback

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestMain(m *testing.M) {
	if err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()}); err != nil {
		panic(fmt.Sprintf("unable to set silent logger: %v", err))
	}
	os.Exit(m.Run())
}

// newTestLoopback configures and starts a loopbackfs over dir. options are extra lines of its configuration section.
func newTestLoopback(t *testing.T, dir string, options ...string) *LoopbackFS {
	t.Helper()

	cfg := fmt.Sprintf("loopbackfs:\n  path: %s\n", dir)
	for _, option := range options {
		cfg += "  " + option + "\n"
	}
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	lfs := NewLoopbackFSComponent().(*LoopbackFS)
	require.NoError(t, lfs.Configure(true))
	require.NoError(t, lfs.Start(context.Background()))
	t.Cleanup(func() { _ = lfs.Stop() })

	return lfs
}

// openFiles returns the files the process has open, as "descriptor -> target" entries.
func openFiles(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)

	files := make(map[string]bool, len(entries))
	for _, entry := range entries {
		// The descriptor of the directory listed here is closed already, so it has no target.
		if target, err := os.Readlink("/proc/self/fd/" + entry.Name()); err == nil {
			files[entry.Name()+" -> "+target] = true
		}
	}
	return files
}

type loopbackTestSuite struct {
	suite.Suite
	lfs  *LoopbackFS
	path string
}

func TestLoopbackFS(t *testing.T) {
	suite.Run(t, new(loopbackTestSuite))
}

func (s *loopbackTestSuite) SetupTest() {
	s.path = s.T().TempDir()
	s.lfs = newTestLoopback(s.T(), s.path)
}

// write creates the file name, and its parents, directly in the backing directory.
func (s *loopbackTestSuite) write(name string, data []byte) {
	p := filepath.Join(s.path, name)
	s.Require().NoError(os.MkdirAll(filepath.Dir(p), 0o755))
	s.Require().NoError(os.WriteFile(p, data, 0o644))
}

// read returns the content of the file name of the backing directory.
func (s *loopbackTestSuite) read(name string) []byte {
	data, err := os.ReadFile(filepath.Join(s.path, name))
	s.Require().NoError(err)
	return data
}

func (s *loopbackTestSuite) exists(name string) bool {
	_, err := os.Lstat(filepath.Join(s.path, name))
	return err == nil
}

func (s *loopbackTestSuite) handle(name string) *handlemap.Handle {
	h, err := s.lfs.OpenFile(internal.OpenFileOptions{Name: name, Flags: os.O_RDWR})
	s.Require().NoError(err)
	return h
}

func (s *loopbackTestSuite) names(attrs []*internal.ObjAttr) []string {
	names := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		names = append(names, attr.Name)
	}
	return names
}

// ------------------------- Configuration -------------------------

func TestConfigure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing", "backing")
	lfs := newTestLoopback(t, dir)

	info, err := os.Stat(dir)
	require.NoError(t, err, "Configure must create the backing directory")
	assert.True(t, info.IsDir())
	assert.Equal(t, dir, lfs.path)
	assert.Equal(t, int32(common.MaxDirListCount), lfs.listPageSize.Load())
	assert.False(t, lfs.contentMD5.Load())

	lfs = newTestLoopback(t, dir, "list-page-size: 7", "content-md5: true")
	assert.Equal(t, int32(7), lfs.listPageSize.Load())
	assert.True(t, lfs.contentMD5.Load())
}

func TestConfigureErrors(t *testing.T) {
	dir := t.TempDir()

	testCases := []struct {
		name string
		cfg  string
	}{
		{"no path", "loopbackfs:\n  list-page-size: 10\n"},
		{"page size too large", fmt.Sprintf("loopbackfs:\n  path: %s\n  list-page-size: 5001\n", dir)},
		{"unknown fault op", fmt.Sprintf("loopbackfs:\n  path: %s\n  faults:\n    - op: Bogus\n      error: EIO\n", dir)},
		{"unknown fault error", fmt.Sprintf("loopbackfs:\n  path: %s\n  faults:\n    - error: EBOGUS\n", dir)},
		{"empty fault", fmt.Sprintf("loopbackfs:\n  path: %s\n  faults:\n    - op: GetAttr\n", dir)},
		{"bad fault pattern", fmt.Sprintf("loopbackfs:\n  path: %s\n  faults:\n    - path: \"[\"\n      error: EIO\n", dir)},
		{"path within the mount", fmt.Sprintf("mount-path: %s\nloopbackfs:\n  path: %s\n", dir, filepath.Join(dir, "inner"))},
		{"path is the mount", fmt.Sprintf("mount-path: %s\nloopbackfs:\n  path: %s\n", dir, dir)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, config.ReadConfigFromReader(strings.NewReader(tc.cfg)))
			lfs := NewLoopbackFSComponent()
			assert.Error(t, lfs.Configure(true))
		})
	}
}

func TestStartWithoutConfigure(t *testing.T) {
	// The component starts, as pipelines that never reach it are tested without its configuration, but it never
	// resolves names against the working directory.
	lfs := NewLoopbackFSComponent()
	require.NoError(t, lfs.Start(context.Background()))

	_, err := lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
	assert.ErrorIs(t, err, errNotConfigured)

	_, err = lfs.CreateFile(internal.CreateFileOptions{Name: "file", Mode: 0o644})
	assert.ErrorIs(t, err, errNotConfigured)
}

func TestOnConfigChange(t *testing.T) {
	dir := t.TempDir()
	lfs := newTestLoopback(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), []byte("data"), 0o644))

	cfg := fmt.Sprintf("loopbackfs:\n  path: %s\n  list-page-size: 3\n  faults:\n    - op: GetAttr\n      error: EIO\n", dir)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))
	lfs.OnConfigChange()

	assert.Equal(t, int32(3), lfs.listPageSize.Load())
	_, err := lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
	assert.Equal(t, syscall.EIO, err)

	// An invalid configuration keeps the previous options.
	cfg = fmt.Sprintf("loopbackfs:\n  path: %s\n  list-page-size: 9999\n", dir)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))
	lfs.OnConfigChange()
	assert.Equal(t, int32(3), lfs.listPageSize.Load())

	// Removing the faults from the configuration removes them from the component.
	cfg = fmt.Sprintf("loopbackfs:\n  path: %s\n", dir)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))
	lfs.OnConfigChange()
	_, err = lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
	assert.NoError(t, err)
	assert.Equal(t, int32(common.MaxDirListCount), lfs.listPageSize.Load())
}

// ------------------------- Names and errors -------------------------

func (s *loopbackTestSuite) TestInvalidNames() {
	s.write("file", []byte("data"))

	for _, name := range []string{`dir\file`, "../outside", "/../outside", stateDir, stateDir + "/staged"} {
		_, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: name})
		s.Equal(syscall.ENOENT, err, "GetAttr(%q)", name)

		_, err = s.lfs.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0o644})
		s.Equal(syscall.EINVAL, err, "CreateFile(%q)", name)

		s.Equal(syscall.EINVAL, s.lfs.CreateDir(internal.CreateDirOptions{Name: name, Mode: 0o755}), "CreateDir(%q)", name)
		s.Equal(syscall.EINVAL, s.lfs.DeleteFile(internal.DeleteFileOptions{Name: name}), "DeleteFile(%q)", name)
		s.Equal(syscall.EINVAL, s.lfs.RenameFile(internal.RenameFileOptions{Src: "file", Dst: name}), "RenameFile(%q)", name)
	}

	s.False(s.exists(`dir\file`))
	s.True(s.exists("file"))
}

func (s *loopbackTestSuite) TestNamesWithSlashes() {
	s.write("dir/file", []byte("data"))

	for _, name := range []string{"dir/file", "/dir/file", "dir/file/", "dir//file", "/dir/./file"} {
		attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: name})
		s.Require().NoError(err, name)
		s.Equal("dir/file", attr.Path, name)
		s.Equal("file", attr.Name, name)
	}

	attrs, err := s.lfs.ReadDir(internal.ReadDirOptions{Name: "/"})
	s.Require().NoError(err)
	s.Equal([]string{"dir"}, s.names(attrs))
	s.Equal("dir", attrs[0].Path, "entries of the root have no leading slash")
}

// Errors must be the bare errno values that azstorage returns, because callers such as libfuse compare them with ==.
func (s *loopbackTestSuite) TestErrorsAreBareErrno() {
	s.write("file", []byte("data"))
	s.write("dir/child", []byte("data"))

	_, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "missing"})
	s.Equal(syscall.ENOENT, err)

	_, err = s.lfs.GetAttr(internal.GetAttrOptions{Name: "file/below"})
	s.Equal(syscall.ENOENT, err, "an object below a file does not exist")

	_, err = s.lfs.OpenFile(internal.OpenFileOptions{Name: "missing", Flags: os.O_RDONLY})
	s.Equal(syscall.ENOENT, err)

	s.Equal(syscall.ENOENT, s.lfs.DeleteFile(internal.DeleteFileOptions{Name: "missing"}))
	s.Equal(syscall.ENOENT, s.lfs.DeleteDir(internal.DeleteDirOptions{Name: "missing"}))
	s.Equal(syscall.ENOENT, s.lfs.RenameFile(internal.RenameFileOptions{Src: "missing", Dst: "other"}))
	s.Equal(syscall.ENOENT, s.lfs.RenameDir(internal.RenameDirOptions{Src: "missing", Dst: "other"}))
	s.Equal(syscall.EEXIST, s.lfs.CreateDir(internal.CreateDirOptions{Name: "dir", Mode: 0o755}))
	s.Equal(syscall.ENOTEMPTY, s.lfs.DeleteDir(internal.DeleteDirOptions{Name: "dir"}))
	s.Equal(syscall.ENOTDIR, s.lfs.DeleteDir(internal.DeleteDirOptions{Name: "file"}))
	s.Equal(syscall.EISDIR, s.lfs.DeleteFile(internal.DeleteFileOptions{Name: "dir"}))

	_, err = s.lfs.ReadDir(internal.ReadDirOptions{Name: "missing"})
	s.Equal(syscall.ENOENT, err)
}

// ------------------------- Attributes -------------------------

func (s *loopbackTestSuite) TestGetAttrFile() {
	s.write("dir/file", []byte("hello"))
	info, err := os.Lstat(filepath.Join(s.path, "dir/file"))
	s.Require().NoError(err)

	attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "dir/file"})
	s.Require().NoError(err)

	s.Equal("dir/file", attr.Path)
	s.Equal("file", attr.Name)
	s.Equal(int64(5), attr.Size)
	s.Equal(info.Mode(), attr.Mode)
	s.True(info.ModTime().Equal(attr.Mtime))
	s.True(attr.Mtime.Equal(attr.Atime), "Azure Storage reports one time")
	s.True(attr.Mtime.Equal(attr.Ctime), "Azure Storage reports one time")
	s.True(attr.Mtime.Equal(attr.Crtime), "Azure Storage reports one time")
	s.True(attr.IsModeDefault())
	s.False(attr.IsDir())
	s.False(attr.IsSymlink())
	s.NotEmpty(attr.ETag)
	s.Nil(attr.MD5)
}

func (s *loopbackTestSuite) TestGetAttrDirAndSymlink() {
	s.write("dir/file", []byte("hello"))
	s.Require().NoError(os.Symlink("dir/file", filepath.Join(s.path, "link")))

	attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "dir"})
	s.Require().NoError(err)
	s.True(attr.IsDir())
	s.False(attr.IsSymlink())

	attr, err = s.lfs.GetAttr(internal.GetAttrOptions{Name: "link"})
	s.Require().NoError(err)
	s.True(attr.IsSymlink())
	s.False(attr.IsDir())
	s.Equal(int64(len("dir/file")), attr.Size, "the size of a symlink is the length of its target")

	attr, err = s.lfs.GetAttr(internal.GetAttrOptions{Name: ""})
	s.Require().NoError(err)
	s.True(attr.IsDir(), "the root is a directory")
	s.Empty(attr.Path)
}

func (s *loopbackTestSuite) TestETagChangesWithEveryChange() {
	s.write("file", []byte("aaaa"))

	etag := func() string {
		attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
		s.Require().NoError(err)
		return attr.ETag
	}

	first := etag()
	s.Equal(first, etag(), "the ETag of an unchanged object must be stable")

	seen := map[string]bool{first: true}
	h := s.handle("file")
	for i := range 5 {
		// Writes of the same size, quicker than the resolution of file system timestamps on many systems.
		_, err := s.lfs.WriteFile(&internal.WriteFileOptions{Handle: h, Data: []byte(fmt.Sprintf("b%03d", i))})
		s.Require().NoError(err)

		current := etag()
		s.False(seen[current], "write %d kept an earlier ETag", i)
		seen[current] = true
	}
}

func (s *loopbackTestSuite) TestContentMD5() {
	data := []byte("content to hash")
	s.write("dir/file", data)
	sum := md5.Sum(data)

	attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "dir/file"})
	s.Require().NoError(err)
	s.Nil(attr.MD5)

	s.lfs.SetConsistency(true)

	attr, err = s.lfs.GetAttr(internal.GetAttrOptions{Name: "dir/file"})
	s.Require().NoError(err)
	s.Equal(sum[:], attr.MD5)

	attrs, _, err := s.lfs.StreamDir(internal.StreamDirOptions{Name: "dir"})
	s.Require().NoError(err)
	s.Require().Len(attrs, 1)
	s.Equal(sum[:], attrs[0].MD5)

	attr, err = s.lfs.GetAttr(internal.GetAttrOptions{Name: "dir"})
	s.Require().NoError(err)
	s.Nil(attr.MD5, "directories have no MD5")
}

// ------------------------- Directories -------------------------

func (s *loopbackTestSuite) TestCreateDir() {
	s.Require().NoError(s.lfs.CreateDir(internal.CreateDirOptions{Name: "a/b/c", Mode: 0o755}))

	info, err := os.Stat(filepath.Join(s.path, "a/b/c"))
	s.Require().NoError(err)
	s.True(info.IsDir())

	// The owner keeps the right to use the directory, whatever mode is asked for.
	s.Require().NoError(s.lfs.CreateDir(internal.CreateDirOptions{Name: "locked", Mode: 0}))
	s.Require().NoError(s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: "locked/file", File: s.tempFile("data")}))
	s.Equal([]byte("data"), s.read("locked/file"))
}

func (s *loopbackTestSuite) TestDeleteDir() {
	s.write("dir/file", []byte("data"))
	s.Require().NoError(os.Mkdir(filepath.Join(s.path, "empty"), 0o755))

	s.Require().NoError(s.lfs.DeleteDir(internal.DeleteDirOptions{Name: "empty"}))
	s.False(s.exists("empty"))

	s.Equal(syscall.ENOTEMPTY, s.lfs.DeleteDir(internal.DeleteDirOptions{Name: "dir"}))
	s.True(s.exists("dir/file"))

	s.Equal(syscall.EBUSY, s.lfs.DeleteDir(internal.DeleteDirOptions{Name: ""}))
}

func (s *loopbackTestSuite) TestIsDirEmpty() {
	s.write("dir/file", []byte("data"))
	s.write("file", []byte("data"))
	s.Require().NoError(os.Mkdir(filepath.Join(s.path, "empty"), 0o755))

	s.True(s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: "empty"}))
	s.False(s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: "dir"}))
	s.False(s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: ""}))

	// Like a listing of Azure Storage, nothing is listed under a missing directory or under a file.
	s.True(s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: "missing"}))
	s.True(s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: "file"}))
	s.False(s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: `bad\name`}))
}

func (s *loopbackTestSuite) TestIsDirEmptyIgnoresStateDir() {
	// An upload goes through the state directory.
	s.Require().NoError(s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: "file", File: s.tempFile("data")}))
	s.Require().NoError(s.lfs.DeleteFile(internal.DeleteFileOptions{Name: "file"}))
	s.True(s.exists(stateDir))

	s.True(s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: ""}))

	attrs, err := s.lfs.ReadDir(internal.ReadDirOptions{Name: ""})
	s.Require().NoError(err)
	s.Empty(attrs, "the state directory must not be listed")
}

func (s *loopbackTestSuite) TestReadDir() {
	s.write("dir/b", []byte("bb"))
	s.write("dir/a.txt", []byte("a"))
	s.write("dir/a/child", []byte("c"))
	s.Require().NoError(os.Symlink("b", filepath.Join(s.path, "dir/link")))

	attrs, err := s.lfs.ReadDir(internal.ReadDirOptions{Name: "dir"})
	s.Require().NoError(err)

	// Azure Storage lists prefixes with their trailing slash, so "a.txt" comes before the directory "a".
	s.Equal([]string{"a.txt", "a", "b", "link"}, s.names(attrs))
	s.Equal("dir/a.txt", attrs[0].Path)
	s.True(attrs[1].IsDir())
	s.Equal(int64(2), attrs[2].Size)
	s.True(attrs[3].IsSymlink())
	for _, attr := range attrs {
		s.True(attr.IsModeDefault())
		s.NotEmpty(attr.ETag)
		s.True(attr.Mtime.Equal(attr.Atime))
	}
}

func (s *loopbackTestSuite) TestStreamDirPages() {
	lfs := newTestLoopback(s.T(), s.path, "list-page-size: 3")
	var want []string
	for i := range 7 {
		name := fmt.Sprintf("file%d", i)
		s.write("dir/"+name, []byte(name))
		want = append(want, name)
	}

	var got []string
	token := ""
	pages := 0
	for {
		attrs, next, err := lfs.StreamDir(internal.StreamDirOptions{Name: "dir", Token: token, Count: common.MaxDirListCount})
		s.Require().NoError(err)
		s.LessOrEqual(len(attrs), 3)
		got = append(got, s.names(attrs)...)
		pages++
		if next == "" {
			break
		}
		token = next
	}

	s.Equal(want, got)
	s.Equal(3, pages)

	// The caller can ask for smaller pages.
	attrs, next, err := lfs.StreamDir(internal.StreamDirOptions{Name: "dir", Count: 2})
	s.Require().NoError(err)
	s.Equal([]string{"file0", "file1"}, s.names(attrs))
	s.NotEmpty(next)

	// A token marks a position in the listing, so changes before or after it are seen as with Azure Storage. Tokens do
	// not depend on the component that returned them.
	s.Require().NoError(os.Remove(filepath.Join(s.path, "dir/file2")))
	s.write("dir/file0a", []byte("new"))
	s.write("dir/file9", []byte("new"))
	attrs, _, err = s.lfs.StreamDir(internal.StreamDirOptions{Name: "dir", Token: next, Count: 100})
	s.Require().NoError(err)
	s.Equal([]string{"file3", "file4", "file5", "file6", "file9"}, s.names(attrs))

	_, _, err = lfs.StreamDir(internal.StreamDirOptions{Name: "dir", Token: "!not a token!"})
	s.Equal(syscall.EINVAL, err)

	_, _, err = lfs.StreamDir(internal.StreamDirOptions{Name: "missing"})
	s.Equal(syscall.ENOENT, err)
}

func (s *loopbackTestSuite) TestRenameDir() {
	s.write("src/file", []byte("data"))

	s.Require().NoError(s.lfs.RenameDir(internal.RenameDirOptions{Src: "src", Dst: "new/parent/dst"}))
	s.False(s.exists("src"))
	s.Equal([]byte("data"), s.read("new/parent/dst/file"))
}

// ------------------------- Files -------------------------

func (s *loopbackTestSuite) tempFile(content string) *os.File {
	f, err := os.CreateTemp(s.T().TempDir(), "upload")
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = f.Close() })

	_, err = f.WriteString(content)
	s.Require().NoError(err)
	return f
}

func (s *loopbackTestSuite) TestCreateFile() {
	h, err := s.lfs.CreateFile(internal.CreateFileOptions{Name: "dir/sub/file", Mode: 0o644})
	s.Require().NoError(err)
	s.Equal("dir/sub/file", h.Path)
	s.Zero(h.Size)
	s.Nil(h.GetFileObject(), "handles must not hold a file")
	s.Empty(s.read("dir/sub/file"))

	// Like an upload of a block blob, creating an existing object replaces it.
	s.write("existing", []byte("old content"))
	_, err = s.lfs.CreateFile(internal.CreateFileOptions{Name: "existing", Mode: 0o644})
	s.Require().NoError(err)
	s.Empty(s.read("existing"))

	// A file created with no permissions stays readable by the component.
	_, err = s.lfs.CreateFile(internal.CreateFileOptions{Name: "noperm", Mode: 0})
	s.Require().NoError(err)
	_, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Handle: s.handle("noperm"), Data: make([]byte, 1)})
	s.NoError(err)

	s.Require().NoError(os.WriteFile(filepath.Join(s.path, "plain"), nil, 0o644))
	_, err = s.lfs.CreateFile(internal.CreateFileOptions{Name: "plain/below", Mode: 0o644})
	s.Equal(syscall.ENOTDIR, err)
}

func (s *loopbackTestSuite) TestCreateFileReplacesSymlink() {
	s.write("target", []byte("keep me"))
	s.Require().NoError(os.Symlink("target", filepath.Join(s.path, "link")))

	_, err := s.lfs.CreateFile(internal.CreateFileOptions{Name: "link", Mode: 0o644})
	s.Require().NoError(err)

	s.Equal([]byte("keep me"), s.read("target"), "the target of the link must not be truncated")
	info, err := os.Lstat(filepath.Join(s.path, "link"))
	s.Require().NoError(err)
	s.True(info.Mode().IsRegular())
}

// Callers such as file_cache and block_cache drop the handle that CreateFile returns, so handles must not hold file
// descriptors. No operation may leak one either.
func (s *loopbackTestSuite) TestNoDescriptorLeaks() {
	s.write("dir/file", []byte("content"))
	upload := s.tempFile("uploaded content")
	download, err := os.Create(filepath.Join(s.T().TempDir(), "download"))
	s.Require().NoError(err)
	defer download.Close()

	before := openFiles(s.T())
	for i := range 20 {
		name := fmt.Sprintf("dir/new%d", i)
		_, err := s.lfs.CreateFile(internal.CreateFileOptions{Name: name, Mode: 0o644})
		s.Require().NoError(err)

		h, err := s.lfs.OpenFile(internal.OpenFileOptions{Name: "dir/file", Flags: os.O_RDONLY})
		s.Require().NoError(err)
		s.Nil(h.GetFileObject())

		s.lfs.IsDirEmpty(internal.IsDirEmptyOptions{Name: "dir"})
		s.Require().NoError(s.lfs.CopyToFile(internal.CopyToFileOptions{Name: "dir/file", File: download}))
		s.Require().NoError(s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: name, File: upload}))
		_, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Data: make([]byte, 4)})
		s.Require().NoError(err)
		_, err = s.lfs.ReadDir(internal.ReadDirOptions{Name: "dir"})
		s.Require().NoError(err)
	}

	// Only descriptors opened by the operations matter: others, such as the socket of a replaced logger, can be closed
	// by the garbage collector meanwhile.
	for file := range openFiles(s.T()) {
		s.True(before[file], "descriptor left open: %s", file)
	}
}

func (s *loopbackTestSuite) TestOpenFile() {
	s.write("file", []byte("content"))
	s.write("dir/child", nil)

	h, err := s.lfs.OpenFile(internal.OpenFileOptions{Name: "file", Flags: os.O_RDONLY})
	s.Require().NoError(err)
	s.Equal("file", h.Path)
	s.Equal(int64(7), h.Size)
	s.False(h.Mtime.IsZero())

	_, err = s.lfs.OpenFile(internal.OpenFileOptions{Name: "dir", Flags: os.O_RDONLY})
	s.Equal(syscall.EISDIR, err)

	// Objects are created by CreateFile, not by OpenFile.
	_, err = s.lfs.OpenFile(internal.OpenFileOptions{Name: "new", Flags: os.O_RDWR | os.O_CREATE})
	s.Equal(syscall.ENOENT, err)

	h, err = s.lfs.OpenFile(internal.OpenFileOptions{Name: "file", Flags: os.O_WRONLY | os.O_TRUNC})
	s.Require().NoError(err)
	s.Zero(h.Size)
	s.Empty(s.read("file"))

	s.Require().NoError(s.lfs.FlushFile(internal.FlushFileOptions{Handle: h}))
	s.Require().NoError(s.lfs.ReleaseFile(internal.ReleaseFileOptions{Handle: h}))
	s.Equal(syscall.EINVAL, s.lfs.ReleaseFile(internal.ReleaseFileOptions{}))
}

func (s *loopbackTestSuite) TestWriteAndReadThroughHandle() {
	h, err := s.lfs.CreateFile(internal.CreateFileOptions{Name: "file", Mode: 0o644})
	s.Require().NoError(err)

	n, err := s.lfs.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: 0, Data: []byte("hello ")})
	s.Require().NoError(err)
	s.Equal(6, n)

	// Even an O_APPEND handle writes at the offset it is given.
	appendHandle, err := s.lfs.OpenFile(internal.OpenFileOptions{Name: "file", Flags: os.O_WRONLY | os.O_APPEND})
	s.Require().NoError(err)
	n, err = s.lfs.WriteFile(&internal.WriteFileOptions{Handle: appendHandle, Offset: 6, Data: []byte("world")})
	s.Require().NoError(err)
	s.Equal(5, n)

	// A handle reads the current content, whatever size it was opened with.
	buf := make([]byte, 64)
	n, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Data: buf})
	s.Require().NoError(err)
	s.Equal("hello world", string(buf[:n]))

	n, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: 6, Data: buf[:3]})
	s.Require().NoError(err)
	s.Equal("wor", string(buf[:n]))

	// At and past the end of the object, a read through a handle returns no data, like pread(2).
	for _, offset := range []int64{11, 100} {
		n, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Offset: offset, Data: buf})
		s.Require().NoError(err)
		s.Zero(n)
	}

	data, err := s.lfs.ReadFile(internal.ReadFileOptions{Handle: h})
	s.Require().NoError(err)
	s.Equal("hello world", string(data))

	data, err = s.lfs.ReadFile(internal.ReadFileOptions{Handle: h})
	s.Require().NoError(err)
	s.Equal("hello world", string(data), "every ReadFile must return the whole object")

	// Handles address the object by name, like the handles of azstorage.
	s.Require().NoError(s.lfs.RenameFile(internal.RenameFileOptions{Src: "file", Dst: "renamed"}))
	_, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Data: buf})
	s.Equal(syscall.ENOENT, err)
	_, err = s.lfs.WriteFile(&internal.WriteFileOptions{Handle: h, Data: []byte("x")})
	s.Equal(syscall.ENOENT, err)
	s.False(s.exists("file"), "a write through a stale handle must not create the object")

	_, err = s.lfs.WriteFile(&internal.WriteFileOptions{Data: []byte("x")})
	s.Equal(syscall.EINVAL, err)
}

func (s *loopbackTestSuite) TestReadInBufferWithoutHandle() {
	s.write("file", []byte("0123456789"))

	read := func(offset, size int64, length int) (string, string, error) {
		etag := "stale"
		buf := make([]byte, length)
		n, err := s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Path: "file", Offset: offset, Size: size, Data: buf, Etag: &etag})
		return string(buf[:n]), etag, err
	}

	attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
	s.Require().NoError(err)

	data, etag, err := read(2, 10, 4)
	s.Require().NoError(err)
	s.Equal("2345", data)
	s.Equal(attr.ETag, etag)

	// Reads stop at the size the caller gives, like azstorage.
	data, _, err = read(6, 8, 10)
	s.Require().NoError(err)
	s.Equal("67", data)

	data, etag, err = read(10, 10, 4)
	s.Require().NoError(err)
	s.Empty(data)
	s.Equal("stale", etag, "a read of no data does not touch the ETag")

	_, _, err = read(11, 10, 4)
	s.Equal(syscall.ERANGE, err)

	// The object has shrunk since the caller got its size.
	s.write("file", []byte("01234"))
	_, etag, err = read(6, 10, 4)
	s.Equal(syscall.ERANGE, err)
	s.Empty(etag, "a failed read must not report an ETag")

	data, _, err = read(3, 10, 4)
	s.Require().NoError(err)
	s.Equal("34", data, "a short read is not an error")

	_, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Offset: 0, Size: 10, Data: make([]byte, 4)})
	s.Equal(syscall.EINVAL, err, "a path is needed without a handle")

	_, err = s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Path: "missing", Size: 10, Data: make([]byte, 4)})
	s.Equal(syscall.ENOENT, err)
}

func (s *loopbackTestSuite) TestReadSymlinkContent() {
	s.Require().NoError(os.Symlink("/etc/passwd", filepath.Join(s.path, "link")))

	// The content of a symlink blob is its target, the link is never followed.
	buf := make([]byte, 64)
	n, err := s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Path: "link", Size: 64, Data: buf})
	s.Require().NoError(err)
	s.Equal("/etc/passwd", string(buf[:n]))
}

func (s *loopbackTestSuite) TestSymlinksStayInBackingDirectory() {
	s.Require().NoError(os.Symlink("/etc", filepath.Join(s.path, "etc")))
	s.Require().NoError(os.Symlink("../..", filepath.Join(s.path, "up")))

	for _, name := range []string{"etc/passwd", "up/etc/passwd"} {
		attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: name})
		s.Error(err, name)
		s.Nil(attr, name)

		n, err := s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Path: name, Size: 10, Data: make([]byte, 10)})
		s.Error(err, name)
		s.Zero(n, name)
	}

	_, err := s.lfs.ReadDir(internal.ReadDirOptions{Name: "etc"})
	s.Error(err)
}

func (s *loopbackTestSuite) TestTruncateFile() {
	s.write("file", []byte("0123456789"))

	s.Require().NoError(s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "file", OldSize: -1, NewSize: 4}))
	s.Equal([]byte("0123"), s.read("file"))

	s.Require().NoError(s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "file", OldSize: 4, NewSize: 6}))
	s.Equal([]byte("0123\x00\x00"), s.read("file"))

	s.Equal(syscall.ENOENT, s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "missing", NewSize: 1}))
	s.Equal(syscall.EINVAL, s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "file", NewSize: -1}))
	s.Require().NoError(os.Mkdir(filepath.Join(s.path, "dir"), 0o755))
	s.Equal(syscall.EISDIR, s.lfs.TruncateFile(internal.TruncateFileOptions{Name: "dir", NewSize: 1}))
}

func (s *loopbackTestSuite) TestCopyToFile() {
	s.write("file", []byte("0123456789"))

	download := func(offset, count int64) (string, error) {
		f, err := os.Create(filepath.Join(s.T().TempDir(), "download"))
		s.Require().NoError(err)
		defer f.Close()

		// The download replaces the content of the file, from its start, whatever its offset.
		_, err = f.WriteString("previous content that is longer")
		s.Require().NoError(err)

		if err := s.lfs.CopyToFile(internal.CopyToFileOptions{Name: "file", Offset: offset, Count: count, File: f}); err != nil {
			return "", err
		}

		data, err := os.ReadFile(f.Name())
		s.Require().NoError(err)
		return string(data), nil
	}

	data, err := download(0, 0)
	s.Require().NoError(err)
	s.Equal("0123456789", data)

	data, err = download(0, 10)
	s.Require().NoError(err)
	s.Equal("0123456789", data)

	data, err = download(3, 4)
	s.Require().NoError(err)
	s.Equal("3456", data)

	data, err = download(7, 0)
	s.Require().NoError(err)
	s.Equal("789", data)

	data, err = download(8, 100)
	s.Require().NoError(err)
	s.Equal("89", data)

	data, err = download(10, 0)
	s.Require().NoError(err)
	s.Empty(data)

	_, err = download(11, 0)
	s.Equal(syscall.ERANGE, err)

	err = s.lfs.CopyToFile(internal.CopyToFileOptions{Name: "missing", File: s.tempFile("")})
	s.Equal(syscall.ENOENT, err)
}

func (s *loopbackTestSuite) TestCopyFromFile() {
	// The upload reads the file from its start, wherever its offset is.
	upload := s.tempFile("uploaded content")
	s.Require().NoError(s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: "dir/file", File: upload}))
	s.Equal([]byte("uploaded content"), s.read("dir/file"))

	// The upload replaces the object at once and keeps its permissions.
	s.Require().NoError(os.Chmod(filepath.Join(s.path, "dir/file"), 0o640))
	before, err := os.Stat(filepath.Join(s.path, "dir/file"))
	s.Require().NoError(err)

	s.Require().NoError(s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: "dir/file", File: s.tempFile("new")}))
	after, err := os.Stat(filepath.Join(s.path, "dir/file"))
	s.Require().NoError(err)
	s.Equal([]byte("new"), s.read("dir/file"))
	s.Equal(os.FileMode(0o640), after.Mode().Perm())
	s.False(os.SameFile(before, after), "the object must be replaced, not rewritten in place")

	s.Require().NoError(os.Mkdir(filepath.Join(s.path, "adir"), 0o755))
	s.Equal(syscall.EISDIR, s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: "adir", File: upload}))
	s.Equal(syscall.EINVAL, s.lfs.CopyFromFile(internal.CopyFromFileOptions{Name: "file"}))

	entries, err := os.ReadDir(filepath.Join(s.path, uploadDir))
	s.Require().NoError(err)
	s.Empty(entries, "no upload file may be left behind")
}

func (s *loopbackTestSuite) TestDeleteFile() {
	s.write("file", []byte("data"))
	s.Require().NoError(os.Symlink("file", filepath.Join(s.path, "link")))

	s.Require().NoError(s.lfs.DeleteFile(internal.DeleteFileOptions{Name: "link"}))
	s.False(s.exists("link"))
	s.True(s.exists("file"), "deleting a link must not delete its target")

	s.Require().NoError(s.lfs.DeleteFile(internal.DeleteFileOptions{Name: "file"}))
	s.False(s.exists("file"))
}

func (s *loopbackTestSuite) TestRenameFile() {
	s.write("src", []byte("data"))
	s.write("dst", []byte("old"))
	past := time.Now().Add(-time.Hour)
	s.Require().NoError(os.Chtimes(filepath.Join(s.path, "src"), past, past))

	srcAttr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "src"})
	s.Require().NoError(err)
	oldETag := srcAttr.ETag

	s.Require().NoError(s.lfs.RenameFile(internal.RenameFileOptions{Src: "src", Dst: "dst", SrcAttr: srcAttr}))
	s.False(s.exists("src"))
	s.Equal([]byte("data"), s.read("dst"))

	// Like a copy of a blob, a rename gives the object a new modification time and ETag, which azstorage reports
	// through SrcAttr.
	dstAttr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "dst"})
	s.Require().NoError(err)
	s.True(dstAttr.Mtime.After(past.Add(time.Minute)))
	s.True(dstAttr.Mtime.Equal(srcAttr.Mtime))
	s.True(dstAttr.Mtime.Equal(srcAttr.Atime))
	s.True(dstAttr.Mtime.Equal(srcAttr.Ctime))
	s.Equal(dstAttr.ETag, srcAttr.ETag)
	s.NotEqual(oldETag, dstAttr.ETag)

	s.Require().NoError(s.lfs.RenameFile(internal.RenameFileOptions{Src: "dst", Dst: "new/parent/file"}))
	s.Equal([]byte("data"), s.read("new/parent/file"))
	s.Require().NoError(s.lfs.RenameFile(internal.RenameFileOptions{Src: "new/parent/file", Dst: "new/parent/file"}))
	s.Equal([]byte("data"), s.read("new/parent/file"))
}

// ------------------------- Symlinks -------------------------

func (s *loopbackTestSuite) TestCreateAndReadLink() {
	// A target is returned as it was given, even one that starts with the backing directory.
	targets := []string{"relative/target", "/absolute/target", s.path + "/inside", s.path + "x/sibling"}
	for i, target := range targets {
		name := fmt.Sprintf("dir/link%d", i)
		s.Require().NoError(s.lfs.CreateLink(internal.CreateLinkOptions{Name: name, Target: target}))

		attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: name})
		s.Require().NoError(err)
		s.True(attr.IsSymlink())

		got, err := s.lfs.ReadLink(internal.ReadLinkOptions{Name: name, Size: attr.Size})
		s.Require().NoError(err)
		s.Equal(target, got)
	}

	// Like azstorage, ReadLink reads no more than the size it is given.
	got, err := s.lfs.ReadLink(internal.ReadLinkOptions{Name: "dir/link0", Size: 8})
	s.Require().NoError(err)
	s.Equal("relative", got)

	got, err = s.lfs.ReadLink(internal.ReadLinkOptions{Name: "dir/link0"})
	s.Require().NoError(err)
	s.Empty(got)

	// Like an upload of a symlink blob, a link replaces an object of the same name.
	s.write("file", []byte("data"))
	s.Require().NoError(s.lfs.CreateLink(internal.CreateLinkOptions{Name: "file", Target: "elsewhere"}))
	got, err = s.lfs.ReadLink(internal.ReadLinkOptions{Name: "file", Size: 100})
	s.Require().NoError(err)
	s.Equal("elsewhere", got)

	_, err = s.lfs.ReadLink(internal.ReadLinkOptions{Name: "dir", Size: 100})
	s.Equal(syscall.EINVAL, err, "a directory is not a link")
	s.Equal(syscall.EEXIST, s.lfs.CreateLink(internal.CreateLinkOptions{Name: "dir", Target: "x"}))
}

// ------------------------- Permissions -------------------------

func (s *loopbackTestSuite) TestChmod() {
	s.write("file", []byte("data"))
	s.Require().NoError(os.Mkdir(filepath.Join(s.path, "dir"), 0o755))
	s.Require().NoError(os.Symlink("file", filepath.Join(s.path, "link")))

	mode := func(name string) os.FileMode {
		info, err := os.Lstat(filepath.Join(s.path, name))
		s.Require().NoError(err)
		return info.Mode().Perm()
	}

	s.Require().NoError(s.lfs.Chmod(internal.ChmodOptions{Name: "file", Mode: 0o640}))
	s.Equal(os.FileMode(0o640), mode("file"))

	attr, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: "file"})
	s.Require().NoError(err)
	s.Equal(os.FileMode(0o640), attr.Mode.Perm())

	// The owner keeps the right to read and write, so the component can still serve the object.
	s.Require().NoError(s.lfs.Chmod(internal.ChmodOptions{Name: "file", Mode: 0}))
	s.Equal(os.FileMode(0o600), mode("file"))
	n, err := s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Path: "file", Size: 4, Data: make([]byte, 4)})
	s.Require().NoError(err)
	s.Equal(4, n)

	s.Require().NoError(s.lfs.Chmod(internal.ChmodOptions{Name: "dir", Mode: 0o500}))
	s.Equal(os.FileMode(0o700), mode("dir"))

	// A symbolic link has no mode of its own, and its target must not change.
	s.Require().NoError(s.lfs.Chmod(internal.ChmodOptions{Name: "link", Mode: 0o777}))
	s.Equal(os.FileMode(0o600), mode("file"))

	s.Equal(syscall.ENOENT, s.lfs.Chmod(internal.ChmodOptions{Name: "missing", Mode: 0o644}))
}

func (s *loopbackTestSuite) TestChown() {
	s.write("file", []byte("data"))

	s.Require().NoError(s.lfs.Chown(internal.ChownOptions{Name: "file", Owner: os.Getuid(), Group: os.Getgid()}))

	// Azure Storage does not fail a change of owner, even one the process is not allowed to make.
	s.NoError(s.lfs.Chown(internal.ChownOptions{Name: "file", Owner: os.Getuid() + 1, Group: os.Getgid() + 1}))

	s.Equal(syscall.ENOENT, s.lfs.Chown(internal.ChownOptions{Name: "missing", Owner: os.Getuid(), Group: os.Getgid()}))
}

// ------------------------- Concurrency -------------------------

// Operations on the same objects run concurrently, as they do below libfuse. Run with -race.
func (s *loopbackTestSuite) TestConcurrentOperations() {
	const workers = 8
	const rounds = 20

	s.write("shared", make([]byte, 64))
	errs := make(chan error, workers*rounds*4)
	done := make(chan struct{})

	for w := range workers {
		go func() {
			defer func() { done <- struct{}{} }()

			name := fmt.Sprintf("dir%d/file", w%2)
			h := handlemap.NewHandle("shared")
			for range rounds {
				if _, err := s.lfs.WriteFile(&internal.WriteFileOptions{Handle: h, Offset: int64(w), Data: []byte{byte(w)}}); err != nil {
					errs <- err
				}
				if _, err := s.lfs.ReadInBuffer(&internal.ReadInBufferOptions{Handle: h, Data: make([]byte, 64)}); err != nil {
					errs <- err
				}
				if _, _, err := s.lfs.StreamDir(internal.StreamDirOptions{Name: ""}); err != nil {
					errs <- err
				}
				if _, err := s.lfs.GetAttr(internal.GetAttrOptions{Name: name}); err != nil && err != syscall.ENOENT {
					errs <- err
				}
			}
		}()
	}

	for range workers {
		<-done
	}
	close(errs)

	for err := range errs {
		s.NoError(err)
	}
}

// ------------------------- Helpers -------------------------

// blockID returns a valid block ID for n, 16 bytes long before encoding like the IDs of block_cache.
func blockID(n int) string {
	return encodeID(fmt.Sprintf("block-%010d", n))
}

func encodeID(raw string) string {
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func TestObjectName(t *testing.T) {
	testCases := []struct {
		in, want string
		err      error
	}{
		{"", ".", nil},
		{"/", ".", nil},
		{"a", "a", nil},
		{"/a/b/", "a/b", nil},
		{"a//b", "a/b", nil},
		{"a/./b", "a/b", nil},
		{"a/../b", "b", nil},
		{"..", "", syscall.EINVAL},
		{"a/../..", "", syscall.EINVAL},
		{`a\b`, "", syscall.EINVAL},
		{stateDir, "", syscall.EINVAL},
		{stateDir + "/x", "", syscall.EINVAL},
		{"dir/" + stateDir, "dir/" + stateDir, nil},
		{stateDir + "x", stateDir + "x", nil},
	}

	for _, tc := range testCases {
		got, err := objectName(tc.in)
		assert.Equal(t, tc.err, err, "objectName(%q)", tc.in)
		assert.Equal(t, tc.want, got, "objectName(%q)", tc.in)
	}
}

func TestCopyDataEmpty(t *testing.T) {
	var out strings.Builder
	n, err := copyData(&out, strings.NewReader(""), 0)
	require.NoError(t, err)
	assert.Zero(t, n)

	n, err = copyData(&out, io.LimitReader(strings.NewReader("abcdef"), 3), 3)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.Equal(t, "abc", out.String())
}
