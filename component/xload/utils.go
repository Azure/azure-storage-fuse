/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
	"math"
	"os"
	"reflect"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/JeffreyRichter/enum/enum"
)

// One workitem to be processed
type workItem struct {
	compName        string         // Name of the component
	path            string         // Name of the file being processed
	dataLen         uint64         // Length of the data to be processed
	block           *Block         // Block to hold data for
	fileHandle      *os.File       // File handle to the file being processed
	err             error          // Error if any
	responseChannel chan *workItem // Channel to send the response
	download        bool           // boolean variable to decide upload or download
	priority        bool           // boolean flag to decide if this item needs to be processed on priority
}

// xload mode enum
type Mode int

var EMode = Mode(0).INVALID_MODE()

func (Mode) INVALID_MODE() Mode {
	return Mode(0)
}

func (Mode) CHECKPOINT() Mode {
	return Mode(1)
}

func (Mode) DOWNLOAD() Mode {
	return Mode(2)
}

func (Mode) UPLOAD() Mode {
	return Mode(3)
}

func (Mode) SYNC() Mode {
	return Mode(4)
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

func roundFloat(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func isFilePresent(localPath string) (bool, int64) {
	filePresent := true
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		log.Debug("utils::isDownloadRequired : %s is not present in local path [%v]", localPath, err.Error())
		filePresent = false
	}
	return filePresent, fileInfo.Size()
}
