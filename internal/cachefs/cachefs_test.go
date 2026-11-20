// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package cachefs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"
)

type cacheFSSuite struct {
	suite.Suite
	dir    string
	root   *os.Root
	victim string
}

func (s *cacheFSSuite) SetupTest() {
	s.dir = filepath.Join(s.T().TempDir(), "cache")
	s.Require().NoError(os.Mkdir(s.dir, 0777))
	root, err := OpenRoot(s.dir)
	s.Require().NoError(err)
	s.root = root

	s.victim = filepath.Join(s.T().TempDir(), "victim")
	s.Require().NoError(os.WriteFile(s.victim, []byte("keep"), 0600))
}

func (s *cacheFSSuite) TearDownTest() {
	_ = s.root.Close()
}

func (s *cacheFSSuite) assertVictimUntouched() {
	content, err := os.ReadFile(s.victim)
	s.Require().NoError(err)
	s.Equal("keep", string(content))
	info, err := os.Stat(s.victim)
	s.Require().NoError(err)
	s.Equal(os.FileMode(0600), info.Mode().Perm())
}

func (s *cacheFSSuite) TestOpenRootMakesCacheDirPrivate() {
	info, err := os.Stat(s.dir)
	s.Require().NoError(err)
	s.Equal(DirPerm, info.Mode().Perm())
}

func (s *cacheFSSuite) TestOpenFileRefusesSymlinks() {
	s.Require().NoError(os.Symlink(s.victim, filepath.Join(s.dir, "abs")))
	s.Require().NoError(os.Symlink("missing", filepath.Join(s.dir, "dangling")))
	s.Require().NoError(os.Mkdir(filepath.Join(s.dir, "real"), 0700))
	s.Require().NoError(os.Symlink("real", filepath.Join(s.dir, "dirlink")))

	for _, name := range []string{"abs", "dangling", "dirlink/file"} {
		_, err := OpenFile(s.root, name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		s.Error(err, name)
	}

	_, err := os.Lstat(filepath.Join(s.dir, "missing"))
	s.True(os.IsNotExist(err))
	_, err = os.Lstat(filepath.Join(s.dir, "real", "file"))
	s.True(os.IsNotExist(err))
	s.assertVictimUntouched()
}

func (s *cacheFSSuite) TestOpenFileRefusesHardLinkBeforeTruncate() {
	if err := os.Link(s.victim, filepath.Join(s.dir, "hard")); err != nil {
		s.T().Skipf("hard link not possible: %v", err)
	}

	_, err := OpenFile(s.root, "hard", os.O_RDWR|os.O_TRUNC, 0)
	s.ErrorIs(err, ErrUnsafeEntry)
	s.assertVictimUntouched()
}

func (s *cacheFSSuite) TestOpenFileRefusesFifoWithoutBlocking() {
	s.Require().NoError(unix.Mkfifo(filepath.Join(s.dir, "fifo"), 0600))

	_, err := OpenFile(s.root, "fifo", os.O_RDONLY, 0)
	s.ErrorIs(err, ErrUnsafeEntry)
}

func (s *cacheFSSuite) TestOpenFileLimitsModeToPermissionBits() {
	f, err := OpenFile(s.root, "file", os.O_RDWR|os.O_CREATE, os.ModeSetuid|os.ModeSticky|0755)
	s.Require().NoError(err)
	s.NoError(f.Close())

	info, err := os.Lstat(filepath.Join(s.dir, "file"))
	s.Require().NoError(err)
	s.Zero(info.Mode() & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky))
}

func (s *cacheFSSuite) TestMetadataChangesRefuseSymlinks() {
	link := "link"
	s.Require().NoError(os.Symlink(s.victim, filepath.Join(s.dir, link)))

	s.ErrorIs(Chmod(s.root, link, 0777), ErrUnsafeEntry)
	s.ErrorIs(Chown(s.root, link, os.Geteuid(), os.Getegid()), ErrUnsafeEntry)
	s.Error(Truncate(s.root, link, 0))
	s.assertVictimUntouched()

	info, err := Lstat(s.root, link)
	s.Require().NoError(err)
	s.NotZero(info.Mode() & os.ModeSymlink)
}

func (s *cacheFSSuite) TestChmodChangesRegularFile() {
	s.Require().NoError(os.WriteFile(filepath.Join(s.dir, "file"), nil, 0600))

	s.Require().NoError(Chmod(s.root, "file", os.ModeSetuid|0640))
	info, err := os.Lstat(filepath.Join(s.dir, "file"))
	s.Require().NoError(err)
	s.Equal(os.FileMode(0640), info.Mode())
}

func (s *cacheFSSuite) TestMkdirAllRefusesSymlinkAndUsesPrivateMode() {
	outside := s.T().TempDir()
	s.Require().NoError(os.Symlink(outside, filepath.Join(s.dir, "link")))

	s.Error(MkdirAll(s.root, "link/sub"))
	_, err := os.Lstat(filepath.Join(outside, "sub"))
	s.True(os.IsNotExist(err))

	s.Require().NoError(MkdirAll(s.root, "a/b"))
	info, err := os.Lstat(filepath.Join(s.dir, "a", "b"))
	s.Require().NoError(err)
	s.Equal(DirPerm, info.Mode().Perm())
}

func (s *cacheFSSuite) TestRemoveAllDoesNotFollowSymlinks() {
	outside := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(outside, "file"), nil, 0600))
	s.Require().NoError(MkdirAll(s.root, "dir"))
	s.Require().NoError(os.Symlink(outside, filepath.Join(s.dir, "dir", "link")))

	s.Require().NoError(RemoveAll(s.root, "dir"))
	_, err := os.Lstat(filepath.Join(s.dir, "dir"))
	s.True(os.IsNotExist(err))
	_, err = os.Lstat(filepath.Join(outside, "file"))
	s.NoError(err)
}

func (s *cacheFSSuite) TestNamesOutsideRootAreRejected() {
	for _, name := range []string{"../victim", "/etc/passwd", ""} {
		_, err := OpenFile(s.root, name, os.O_RDONLY, 0)
		s.True(errors.Is(err, ErrUnsafeEntry), name)
	}
}

func TestCacheFS(t *testing.T) {
	suite.Run(t, new(cacheFSSuite))
}
