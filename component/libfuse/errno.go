// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

import (
	"errors"
	"io/fs"
	"syscall"
)

// fsErrors are the io/fs errors that stand for an errno, the way os.IsNotExist, os.IsPermission and os.IsExist
// interpret them. EPERM thus counts as EACCES, and ENOTEMPTY as EEXIST.
var fsErrors = map[syscall.Errno]error{
	syscall.ENOENT: fs.ErrNotExist,
	syscall.EACCES: fs.ErrPermission,
	syscall.EEXIST: fs.ErrExist,
}

// errnoOf returns the errno to report err to libfuse with: the first errno in reported that err is or stands for (see
// fsErrors), and EIO if it is none of them. Errors are matched with errors.Is, so wrapped and joined errors match too.
func errnoOf(err error, reported ...syscall.Errno) syscall.Errno {
	for _, errno := range reported {
		if errors.Is(err, errno) {
			return errno
		}
		if fsErr, ok := fsErrors[errno]; ok && errors.Is(err, fsErr) {
			return errno
		}
	}
	return syscall.EIO
}
