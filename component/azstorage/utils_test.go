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

func (s *utilsTestSuite) TestSanitizeSASKey() {
	assert := assert.New(s.T())

	key := sanitizeSASKey("")
	assert.EqualValues("", key)

	key = sanitizeSASKey("?abcd")
	assert.EqualValues("?abcd", key)

	key = sanitizeSASKey("abcd")
	assert.EqualValues("?abcd", key)
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
