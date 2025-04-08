/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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

package xload

import (
	"context"
	"math"
	"os"
	"reflect"

	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/JeffreyRichter/enum/enum"
)

const (
	MAX_WORKER_COUNT         = 100
	MAX_DATA_SPLITTER        = 16
	MAX_LISTER               = 16
	MB                uint64 = (1024 * 1024)
	LISTER            string = "LISTER"
	SPLITTER          string = "SPLITTER"
	DATA_MANAGER      string = "DATA_MANAGER"
)

// One workitem to be processed
type WorkItem struct {
	CompName        string          // Name of the component
	Path            string          // Name of the file being processed
	DataLen         uint64          // Length of the data to be processed
	Mode            os.FileMode     // permissions in 0xxx format
	Atime           time.Time       // access time
	Mtime           time.Time       // modified time
	Block           *Block          // Block to hold data for
	FileHandle      *os.File        // File handle to the file being processed
	Err             error           // Error if any
	ResponseChannel chan *WorkItem  // Channel to send the response
	Download        bool            // boolean variable to decide upload or download
	Priority        bool            // boolean flag to decide if this item needs to be processed on priority
	Ctx             context.Context // context with cancellation method so that if download fails for one block, all other download operations will be cancelled
	MD5             []byte          // content md5 of the blob which can be used to check the consistency of the download
}

// xload mode enum
type Mode int

var EMode = Mode(0).INVALID_MODE()

func (Mode) INVALID_MODE() Mode {
	return Mode(0)
}

func (Mode) PRELOAD() Mode {
	return Mode(1)
}

func (Mode) UPLOAD() Mode {
	return Mode(2)
}

func (Mode) SYNC() Mode {
	return Mode(3)
}

func (m Mode) String() string {
	return enum.StringInt(m, reflect.TypeOf(m))
}

func (m *Mode) Parse(s string) error {
	enumVal, err := enum.ParseInt(reflect.TypeOf(m), s, true, false)
	if enumVal != nil {
		*m = enumVal.(Mode)
	}
	return err
}

func RoundFloat(val float64, precision int) float64 {
	ratio := math.Pow10(precision)
	return math.Round(val*ratio) / ratio
}

// returns if the given path is present, if its a directory and its size
func isFilePresent(localPath string) (bool, bool, int64) {
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		log.Debug("utils::isFilePresent : %s is not present in local path [%v]", localPath, err.Error())
		return false, false, 0
	} else {
		return true, fileInfo.IsDir(), fileInfo.Size()
	}
}
