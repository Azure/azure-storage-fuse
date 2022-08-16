/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package azstorage

import (
	"os"
	"testing"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type utilsTestSuite struct {
	suite.Suite
}

func (s *utilsTestSuite) TestContentType() {
	assert := assert.New(s.T())

	val := getContentType("a.tst")
	assert.EqualValues("application/octet-stream", val, "Content-type mismatch")

	newSet := `{
		".tst": "application/test",
		".dum": "dummy/test"
		}`
	err := populateContentType(newSet)
	assert.Nil(err, "Failed to populate new config")

	val = getContentType("a.tst")
	assert.EqualValues("application/test", val, "Content-type mismatch")

	// assert mp4 content type would get deserialized correctly
	val = getContentType("file.mp4")
	assert.EqualValues(val, "video/mp4")
}

type contentTypeVal struct {
	val    string
	result string
}

func (s *utilsTestSuite) TestGetContentType() {
	assert := assert.New(s.T())
	var inputs = []contentTypeVal{
		{val: "a.css", result: "text/css"},
		{val: "a.pdf", result: "application/pdf"},
		{val: "a.xml", result: "text/xml"},
		{val: "a.csv", result: "text/csv"},
		{val: "a.json", result: "application/json"},
		{val: "a.rtf", result: "application/rtf"},
		{val: "a.txt", result: "text/plain"},
		{val: "a.java", result: "text/plain"},
		{val: "a.dat", result: "text/plain"},
		{val: "a.htm", result: "text/html"},
		{val: "a.html", result: "text/html"},
		{val: "a.gif", result: "image/gif"},
		{val: "a.jpeg", result: "image/jpeg"},
		{val: "a.jpg", result: "image/jpeg"},
		{val: "a.png", result: "image/png"},
		{val: "a.bmp", result: "image/bmp"},
		{val: "a.js", result: "application/javascript"},
		{val: "a.mjs", result: "application/javascript"},
		{val: "a.svg", result: "image/svg+xml"},
		{val: "a.wasm", result: "application/wasm"},
		{val: "a.webp", result: "image/webp"},
		{val: "a.wav", result: "audio/wav"},
		{val: "a.mp3", result: "audio/mpeg"},
		{val: "a.mpeg", result: "video/mpeg"},
		{val: "a.aac", result: "audio/aac"},
		{val: "a.avi", result: "video/x-msvideo"},
		{val: "a.m3u8", result: "application/x-mpegURL"},
		{val: "a.ts", result: "video/MP2T"},
		{val: "a.mid", result: "audio/midiaudio/x-midi"},
		{val: "a.3gp", result: "video/3gpp"},
		{val: "a.mp4", result: "video/mp4"},
		{val: "a.doc", result: "application/msword"},
		{val: "a.docx", result: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{val: "a.ppt", result: "application/vnd.ms-powerpoint"},
		{val: "a.pptx", result: "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		{val: "a.xls", result: "application/vnd.ms-excel"},
		{val: "a.xlsx", result: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{val: "a.gz", result: "application/x-gzip"},
		{val: "a.jar", result: "application/java-archive"},
		{val: "a.rar", result: "application/vnd.rar"},
		{val: "a.tar", result: "application/x-tar"},
		{val: "a.zip", result: "application/x-zip-compressed"},
		{val: "a.7z", result: "application/x-7z-compressed"},
		{val: "a.3g2", result: "video/3gpp2"},
		{val: "a.sh", result: "application/x-sh"},
		{val: "a.exe", result: "application/x-msdownload"},
		{val: "a.dll", result: "application/x-msdownload"},
	}
	for _, i := range inputs {
		s.Run(i.val, func() {
			output := getContentType(i.val)
			assert.EqualValues(i.result, output)
		})
	}
}

type accesTierVal struct {
	val    string
	result azblob.AccessTierType
}

func (s *utilsTestSuite) TestGetAccessTierType() {
	assert := assert.New(s.T())
	var inputs = []accesTierVal{
		{val: "", result: azblob.AccessTierNone},
		{val: "none", result: azblob.AccessTierNone},
		{val: "hot", result: azblob.AccessTierHot},
		{val: "cool", result: azblob.AccessTierCool},
		{val: "archive", result: azblob.AccessTierArchive},
		{val: "p4", result: azblob.AccessTierP4},
		{val: "p6", result: azblob.AccessTierP6},
		{val: "p10", result: azblob.AccessTierP10},
		{val: "p15", result: azblob.AccessTierP15},
		{val: "p20", result: azblob.AccessTierP20},
		{val: "p30", result: azblob.AccessTierP30},
		{val: "p40", result: azblob.AccessTierP40},
		{val: "p50", result: azblob.AccessTierP50},
		{val: "p60", result: azblob.AccessTierP60},
		{val: "p70", result: azblob.AccessTierP70},
		{val: "p80", result: azblob.AccessTierP80},
		{val: "random", result: azblob.AccessTierNone},
	}
	for _, i := range inputs {
		s.Run(i.val, func() {
			output := getAccessTierType(i.val)
			assert.EqualValues(i.result, output)
		})
	}
}

type fileMode struct {
	val  string
	mode os.FileMode
	str  string
}

func (s *utilsTestSuite) TestGetFileMode() {
	assert := assert.New(s.T())
	var inputs = []fileMode{
		{"", 0, ""},
		{"rwx", 0, "unexpected length of permissions from the service"},
		{"rw-rw-rw-", 0x1b6, ""},
		{"rwxrwxrwx+", 0x1ff, ""},
	}

	for _, i := range inputs {
		s.Run(i.val, func() {
			m, err := getFileMode(i.val)
			if i.str == "" {
				assert.Nil(err)
			}

			assert.EqualValues(i.mode, m)
			if err != nil {
				assert.Contains(err.Error(), i.str)
			}

		})
	}
}

func (s *utilsTestSuite) TestGetMD5() {
	assert := assert.New(s.T())

	f, err := os.Create("abc.txt")
	assert.Nil(err)

	_, err = f.Write([]byte(randomString(50)))
	assert.Nil(err)

	f.Close()

	f, err = os.Open("abc.txt")
	assert.Nil(err)

	md5Sum, err := getMD5(f)
	assert.Nil(err)
	assert.NotZero(md5Sum)

	f.Close()
	os.Remove("abc.txt")
}

func (s *utilsTestSuite) TestSanitizeSASKey() {
	assert := assert.New(s.T())

	key := sanitizeSASKey("")
	assert.EqualValues("", key)

	key = sanitizeSASKey("?abcd")
	assert.EqualValues("?abcd", key)

	key = sanitizeSASKey("abcd")
	assert.EqualValues("?abcd", key)
}

func (s *utilsTestSuite) TestBlockNonProxyOptions() {
	assert := assert.New(s.T())
	po, ro := getAzBlobPipelineOptions(AzStorageConfig{})
	assert.EqualValues(ro.MaxTries, int(0))
	assert.NotEqual(po.RequestLog.SyslogDisabled, true)
}

func (s *utilsTestSuite) TestBlockProxyOptions() {
	assert := assert.New(s.T())
	po, ro := getAzBlobPipelineOptions(AzStorageConfig{proxyAddress: "127.0.0.1", maxRetries: 3})
	assert.EqualValues(ro.MaxTries, 3)
	assert.NotEqual(po.RequestLog.SyslogDisabled, true)
}

func (s *utilsTestSuite) TestBfsNonProxyOptions() {
	assert := assert.New(s.T())
	po, ro := getAzBfsPipelineOptions(AzStorageConfig{})
	assert.EqualValues(ro.MaxTries, int(0))
	assert.NotEqual(po.RequestLog.SyslogDisabled, true)
}

func (s *utilsTestSuite) TestBfsProxyOptions() {
	assert := assert.New(s.T())
	po, ro := getAzBfsPipelineOptions(AzStorageConfig{proxyAddress: "127.0.0.1", maxRetries: 3})
	assert.EqualValues(ro.MaxTries, 3)
	assert.NotEqual(po.RequestLog.SyslogDisabled, true)
}
func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
