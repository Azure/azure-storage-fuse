/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
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

func (s *utilsTestSuite) TestPrefixPathRemoval() {
	assert := assert.New(s.T())

	type PrefixPath struct {
		prefix string
		path   string
		result string
	}

	var inputs = []PrefixPath{
		{prefix: "", path: "abc.txt", result: "abc.txt"},
		{prefix: "", path: "ABC", result: "ABC"},
		{prefix: "", path: "ABC/DEF.txt", result: "ABC/DEF.txt"},
		{prefix: "", path: "ABC/DEF/1.txt", result: "ABC/DEF/1.txt"},

		{prefix: "ABC", path: "ABC/DEF/1.txt", result: "DEF/1.txt"},
		{prefix: "ABC/", path: "ABC/DEF/1.txt", result: "DEF/1.txt"},
		{prefix: "ABC", path: "ABC/DEF", result: "DEF"},
		{prefix: "ABC/", path: "ABC/DEF", result: "DEF"},
		{prefix: "ABC/", path: "ABC/DEF/G/H/1.txt", result: "DEF/G/H/1.txt"},

		{prefix: "ABC/DEF", path: "ABC/DEF/1.txt", result: "1.txt"},
		{prefix: "ABC/DEF/", path: "ABC/DEF/1.txt", result: "1.txt"},
		{prefix: "ABC/DEF", path: "ABC/DEF/A/B/c.txt", result: "A/B/c.txt"},
		{prefix: "ABC/DEF/", path: "ABC/DEF/A/B/c.txt", result: "A/B/c.txt"},

		{prefix: "A/B/C/D/E", path: "A/B/C/D/E/F/G/H/I/j.txt", result: "F/G/H/I/j.txt"},
		{prefix: "A/B/C/D/E/", path: "A/B/C/D/E/F/G/H/I/j.txt", result: "F/G/H/I/j.txt"},
	}

	for _, i := range inputs {
		s.Run(filepath.Join(i.prefix, i.path), func() {
			output := split(i.prefix, i.path)
			assert.EqualValues(i.result, output)
		})
	}

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
		{val: "a.cSS", result: "text/css"},
		{val: "a.Mp4", result: "video/mp4"},
		{val: "a.JPG", result: "image/jpeg"},
		{val: "a.usdz", result: "application/zip"},
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

	_ = log.SetDefaultLogger("silent", common.LogConfig{})

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

func (s *utilsTestSuite) TestGetFileModeFromACL() {
	assert := assert.New(s.T())

	type blobACLs struct {
		acl    string
		owner  string
		mode   os.FileMode
		errstr string
	}

	objid := "tmp-obj-id"
	var inputs = []blobACLs{
		// acl, owner, mode, error string
		{"", "", 0, "empty permissions from the service"},
		{"user::rwx,user:tmp-obj-1:r--,user:tmp-obj-id:r-x,group::r--,mask::r-x,other::rwx", "", 0547, ""},
		{"user::rwx,user:tmp-obj-1:r--,user:tmp-obj-id:rwx,group::r--,mask::r--,other::rwx", "", 0447, ""},
		{"user::rwx,user:tmp-obj-1:r--,user:tmp-obj-id:rwx,group::rw-,mask::r--,other::rwx", "tmp-obj-id", 0767, ""},
		{"user::rwx,user:tmp-obj-1:r--,group::rw-,mask::r--,other::rwx", "tmp-obj-id", 0767, ""},
		{"user::rwx,user:tmp-obj-1:r--,group::rw-,mask::r--,other::rwx", "0", 0067, ""},
	}

	_ = log.SetDefaultLogger("silent", common.LogConfig{})

	for _, i := range inputs {
		s.Run(i.acl, func() {
			m, err := getFileModeFromACL(objid, i.acl, i.owner)
			if i.errstr == "" {
				assert.Nil(err)
				assert.EqualValues(i.mode, m)
			} else {
				assert.NotNil(err)
				assert.Contains(err.Error(), i.errstr)
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

type endpointAccountType struct {
	endpoint string
	account  AccountType
	result   string
}

func (s *utilsTestSuite) TestFormatEndpointAccountType() {
	assert := assert.New(s.T())
	var inputs = []endpointAccountType{
		{endpoint: "https://account.blob.core.windows.net", account: EAccountType.BLOCK(), result: "https://account.blob.core.windows.net"},
		{endpoint: "https://blobaccount.blob.core.windows.net", account: EAccountType.BLOCK(), result: "https://blobaccount.blob.core.windows.net"},
		{endpoint: "https://accountblob.blob.core.windows.net", account: EAccountType.BLOCK(), result: "https://accountblob.blob.core.windows.net"},
		{endpoint: "https://dfsaccount.blob.core.windows.net", account: EAccountType.BLOCK(), result: "https://dfsaccount.blob.core.windows.net"},
		{endpoint: "https://accountdfs.blob.core.windows.net", account: EAccountType.BLOCK(), result: "https://accountdfs.blob.core.windows.net"},

		{endpoint: "https://account.dfs.core.windows.net", account: EAccountType.BLOCK(), result: "https://account.blob.core.windows.net"},
		{endpoint: "https://dfsaccount.dfs.core.windows.net", account: EAccountType.BLOCK(), result: "https://dfsaccount.blob.core.windows.net"},
		{endpoint: "https://accountdfs.dfs.core.windows.net", account: EAccountType.BLOCK(), result: "https://accountdfs.blob.core.windows.net"},
		{endpoint: "https://blobaccount.dfs.core.windows.net", account: EAccountType.BLOCK(), result: "https://blobaccount.blob.core.windows.net"},
		{endpoint: "https://accountblob.dfs.core.windows.net", account: EAccountType.BLOCK(), result: "https://accountblob.blob.core.windows.net"},

		{endpoint: "https://account.blob.core.windows.net", account: EAccountType.ADLS(), result: "https://account.dfs.core.windows.net"},
		{endpoint: "https://blobaccount.blob.core.windows.net", account: EAccountType.ADLS(), result: "https://blobaccount.dfs.core.windows.net"},
		{endpoint: "https://accountblob.blob.core.windows.net", account: EAccountType.ADLS(), result: "https://accountblob.dfs.core.windows.net"},
		{endpoint: "https://dfsaccount.blob.core.windows.net", account: EAccountType.ADLS(), result: "https://dfsaccount.dfs.core.windows.net"},
		{endpoint: "https://accountdfs.blob.core.windows.net", account: EAccountType.ADLS(), result: "https://accountdfs.dfs.core.windows.net"},

		{endpoint: "https://account.dfs.core.windows.net", account: EAccountType.ADLS(), result: "https://account.dfs.core.windows.net"},
		{endpoint: "https://dfsaccount.dfs.core.windows.net", account: EAccountType.ADLS(), result: "https://dfsaccount.dfs.core.windows.net"},
		{endpoint: "https://accountdfs.dfs.core.windows.net", account: EAccountType.ADLS(), result: "https://accountdfs.dfs.core.windows.net"},
		{endpoint: "https://blobaccount.dfs.core.windows.net", account: EAccountType.ADLS(), result: "https://blobaccount.dfs.core.windows.net"},
		{endpoint: "https://accountblob.dfs.core.windows.net", account: EAccountType.ADLS(), result: "https://accountblob.dfs.core.windows.net"},

		// Private Endpoint
		{endpoint: "https://myprivateendpoint.net", account: EAccountType.BLOCK(), result: "https://myprivateendpoint.net"},
		{endpoint: "https://myprivateendpoint.net", account: EAccountType.ADLS(), result: "https://myprivateendpoint.net"},

		// Zonal DNS endpoint
		{endpoint: "https://account.z99.blob.storage.azure.net", account: EAccountType.BLOCK(), result: "https://account.z99.blob.storage.azure.net"},
		{endpoint: "https://account.z99.blob.storage.azure.net", account: EAccountType.ADLS(), result: "https://account.z99.dfs.storage.azure.net"},
		{endpoint: "https://account.z99.dfs.storage.azure.net", account: EAccountType.BLOCK(), result: "https://account.z99.blob.storage.azure.net"},
		{endpoint: "https://account.z99.dfs.storage.azure.net", account: EAccountType.ADLS(), result: "https://account.z99.dfs.storage.azure.net"},

		// China Cloud endpoint
		{endpoint: "https://account.z99.blob.core.chinacloudapi.cn", account: EAccountType.BLOCK(), result: "https://account.z99.blob.core.chinacloudapi.cn"},
		{endpoint: "https://account.z99.blob.core.chinacloudapi.cn", account: EAccountType.ADLS(), result: "https://account.z99.dfs.core.chinacloudapi.cn"},
		{endpoint: "https://account.z99.dfs.core.chinacloudapi.cn", account: EAccountType.BLOCK(), result: "https://account.z99.blob.core.chinacloudapi.cn"},
		{endpoint: "https://account.z99.dfs.core.chinacloudapi.cn", account: EAccountType.ADLS(), result: "https://account.z99.dfs.core.chinacloudapi.cn"},

		// Germany endpoint
		{endpoint: "https://account.z99.blob.core.cloudapi.de", account: EAccountType.BLOCK(), result: "https://account.z99.blob.core.cloudapi.de"},
		{endpoint: "https://account.z99.blob.core.cloudapi.de", account: EAccountType.ADLS(), result: "https://account.z99.dfs.core.cloudapi.de"},
		{endpoint: "https://account.z99.dfs.core.cloudapi.de", account: EAccountType.BLOCK(), result: "https://account.z99.blob.core.cloudapi.de"},
		{endpoint: "https://account.z99.dfs.core.cloudapi.de", account: EAccountType.ADLS(), result: "https://account.z99.dfs.core.cloudapi.de"},

		// Government endpoint
		{endpoint: "https://account.z99.blob.core.usgovcloudapi.net", account: EAccountType.BLOCK(), result: "https://account.z99.blob.core.usgovcloudapi.net"},
		{endpoint: "https://account.z99.blob.core.usgovcloudapi.net", account: EAccountType.ADLS(), result: "https://account.z99.dfs.core.usgovcloudapi.net"},
		{endpoint: "https://account.z99.dfs.core.usgovcloudapi.net", account: EAccountType.BLOCK(), result: "https://account.z99.blob.core.usgovcloudapi.net"},
		{endpoint: "https://account.z99.dfs.core.usgovcloudapi.net", account: EAccountType.ADLS(), result: "https://account.z99.dfs.core.usgovcloudapi.net"},
	}
	for _, i := range inputs {
		s.Run(i.endpoint+","+i.account.String(), func() {
			output := formatEndpointAccountType(i.endpoint, i.account)
			assert.EqualValues(i.result, output)
		})
	}
}

type endpointProtocol struct {
	endpoint string
	ustHttp  bool
	result   string
}

func (s *utilsTestSuite) TestFormatEndpointProtocol() {
	assert := assert.New(s.T())
	var inputs = []endpointProtocol{
		{endpoint: "https://account.blob.core.windows.net", result: "https://account.blob.core.windows.net/", ustHttp: true},
		{endpoint: "http://account.blob.core.windows.net", result: "http://account.blob.core.windows.net/", ustHttp: false},
		{endpoint: "account.blob.core.windows.net", result: "http://account.blob.core.windows.net/", ustHttp: true},
		{endpoint: "account.blob.core.windows.net", result: "https://account.blob.core.windows.net/", ustHttp: false},
		{endpoint: "account.bl://ob.core.windows.net", result: "https://account.bl://ob.core.windows.net/", ustHttp: false},
		{endpoint: "account.bl://ob.core.windows.net", result: "http://account.bl://ob.core.windows.net/", ustHttp: true},
		{endpoint: "https://account.blob.core.windows.net/", result: "https://account.blob.core.windows.net/", ustHttp: true},
		{endpoint: "https://account.blob.core.windows.net/abc", result: "https://account.blob.core.windows.net/abc/", ustHttp: true},

		// These are false positive test cases where we are forming the wrong URI and it shall fail for user when used in blobfuse
		{endpoint: "://account.blob.core.windows.net", result: "https://://account.blob.core.windows.net/", ustHttp: false},
		{endpoint: "://account.blob.core.windows.net", result: "http://://account.blob.core.windows.net/", ustHttp: true},
		{endpoint: "https://://./account.blob.core.windows.net", result: "https://://./account.blob.core.windows.net/", ustHttp: true},
	}

	for _, i := range inputs {
		s.Run(i.endpoint+","+strconv.FormatBool(i.ustHttp), func() {
			output := formatEndpointProtocol(i.endpoint, i.ustHttp)
			assert.EqualValues(i.result, output)
		})
	}
}

func (s *utilsTestSuite) TestAutoDetectAuthMode() {
	assert := assert.New(s.T())

	var authType string
	authType = autoDetectAuthMode(AzStorageOptions{})
	assert.Equal(authType, "msi")

	var authType_ AuthType
	err := authType_.Parse(authType)
	assert.Nil(err)
	assert.Equal(authType_, EAuthType.MSI())

	authType = autoDetectAuthMode(AzStorageOptions{AccountKey: "abc"})
	assert.Equal(authType, "key")

	authType = autoDetectAuthMode(AzStorageOptions{SaSKey: "abc"})
	assert.Equal(authType, "sas")

	authType = autoDetectAuthMode(AzStorageOptions{ApplicationID: "abc"})
	assert.Equal(authType, "msi")

	authType = autoDetectAuthMode(AzStorageOptions{ResourceID: "abc"})
	assert.Equal(authType, "msi")

	authType = autoDetectAuthMode(AzStorageOptions{ClientID: "abc"})
	assert.Equal(authType, "spn")

	authType = autoDetectAuthMode(AzStorageOptions{ClientSecret: "abc"})
	assert.Equal(authType, "spn")

	authType = autoDetectAuthMode(AzStorageOptions{TenantID: "abc"})
	assert.Equal(authType, "spn")

	authType = autoDetectAuthMode(AzStorageOptions{ApplicationID: "abc", AccountKey: "abc", SaSKey: "abc", ClientID: "abc"})
	assert.Equal(authType, "msi")

	authType = autoDetectAuthMode(AzStorageOptions{AccountKey: "abc", SaSKey: "abc", ClientID: "abc"})
	assert.Equal(authType, "key")

	authType = autoDetectAuthMode(AzStorageOptions{SaSKey: "abc", ClientID: "abc"})
	assert.Equal(authType, "sas")
}

func (s *utilsTestSuite) TestRemoveLeadingSlashes() {
	assert := assert.New(s.T())
	var inputs = []struct {
		subdirectory string
		result       string
	}{
		{subdirectory: "/abc/def", result: "abc/def"},
		{subdirectory: "////abc/def/", result: "abc/def/"},
		{subdirectory: "abc/def/", result: "abc/def/"},
		{subdirectory: "", result: ""},
	}

	for _, i := range inputs {
		assert.Equal(i.result, removeLeadingSlashes(i.subdirectory))
	}
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
