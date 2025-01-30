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

package azstorage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
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
	result *blob.AccessTier
}

func (s *utilsTestSuite) TestGetAccessTierType() {
	assert := assert.New(s.T())
	var inputs = []accesTierVal{
		{val: "", result: nil},
		{val: "none", result: nil},
		{val: "hot", result: to.Ptr(blob.AccessTierHot)},
		{val: "cool", result: to.Ptr(blob.AccessTierCool)},
		{val: "cold", result: to.Ptr(blob.AccessTierCold)},
		{val: "archive", result: to.Ptr(blob.AccessTierArchive)},
		{val: "p4", result: to.Ptr(blob.AccessTierP4)},
		{val: "p6", result: to.Ptr(blob.AccessTierP6)},
		{val: "p10", result: to.Ptr(blob.AccessTierP10)},
		{val: "p15", result: to.Ptr(blob.AccessTierP15)},
		{val: "p20", result: to.Ptr(blob.AccessTierP20)},
		{val: "p30", result: to.Ptr(blob.AccessTierP30)},
		{val: "p40", result: to.Ptr(blob.AccessTierP40)},
		{val: "p50", result: to.Ptr(blob.AccessTierP50)},
		{val: "p60", result: to.Ptr(blob.AccessTierP60)},
		{val: "p70", result: to.Ptr(blob.AccessTierP70)},
		{val: "p80", result: to.Ptr(blob.AccessTierP80)},
		{val: "premium", result: to.Ptr(blob.AccessTierPremium)},
		{val: "random", result: nil},
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
	opt, err := getAzBlobServiceClientOptions(&AzStorageConfig{})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 0)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)
}

func (s *utilsTestSuite) TestBlockProxyOptions() {
	assert := assert.New(s.T())
	opt, err := getAzBlobServiceClientOptions(&AzStorageConfig{proxyAddress: "127.0.0.1", maxRetries: 3})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 3)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)

	opt, err = getAzBlobServiceClientOptions(&AzStorageConfig{proxyAddress: "http://127.0.0.1:8080", maxRetries: 3})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 3)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)

	opt, err = getAzBlobServiceClientOptions(&AzStorageConfig{proxyAddress: "https://128.0.0.1:8080", maxRetries: 3})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 3)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)
}

func (s *utilsTestSuite) TestBfsNonProxyOptions() {
	assert := assert.New(s.T())
	opt, err := getAzDatalakeServiceClientOptions(&AzStorageConfig{})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 0)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)
}

func (s *utilsTestSuite) TestBfsProxyOptions() {
	assert := assert.New(s.T())
	opt, err := getAzDatalakeServiceClientOptions(&AzStorageConfig{proxyAddress: "127.0.0.1", maxRetries: 3})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 3)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)

	opt, err = getAzDatalakeServiceClientOptions(&AzStorageConfig{proxyAddress: "http://127.0.0.1:8080", maxRetries: 3})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 3)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)

	opt, err = getAzDatalakeServiceClientOptions(&AzStorageConfig{proxyAddress: "https://128.0.0.1:8080", maxRetries: 3})
	assert.Nil(err)
	assert.EqualValues(opt.Retry.MaxRetries, 3)
	assert.GreaterOrEqual(len(opt.Logging.AllowedHeaders), 1)
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

func (suite *utilsTestSuite) TestRemovePrefixPath() {
	assert := assert.New(suite.T())

	var inputs = []struct {
		prefixPath string
		path       string
		result     string
	}{
		{prefixPath: "", path: "abc.txt", result: "abc.txt"},
		{prefixPath: "", path: "ABC/DEF/abc.txt", result: "ABC/DEF/abc.txt"},
		{prefixPath: "ABC", path: "ABC/DEF/1.txt", result: "DEF/1.txt"},
		{prefixPath: "ABC/", path: "ABC/DEF/1.txt", result: "DEF/1.txt"},
		{prefixPath: "ABC/DEF", path: "ABC/DEF/1.txt", result: "1.txt"},
		{prefixPath: "ABC/DEF/", path: "ABC/DEF/1.txt", result: "1.txt"},
		{prefixPath: "ABC", path: "ABC/ABC.txt", result: "ABC.txt"},
		{prefixPath: "A/B/C/D/E/", path: "A/B/C/D/E/F/G/H/I/j.txt", result: "F/G/H/I/j.txt"},
		{prefixPath: "A/B/C/D/E/", path: "A/B/C/D/E/F/G/H/I/j.txt", result: "F/G/H/I/j.txt"},
	}

	for _, i := range inputs {
		suite.Run(filepath.Join(i.prefixPath, i.path), func() {
			output := removePrefixPath(i.prefixPath, i.path)
			assert.EqualValues(i.result, output)
		})
	}
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
