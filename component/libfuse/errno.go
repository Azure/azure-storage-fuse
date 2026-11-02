/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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
