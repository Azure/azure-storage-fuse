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

package handlemap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type HandleMapSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *HandleMapSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

// mount failure test where the mount directory does not exist
func (suite *HandleMapSuite) TestNewHandle() {
	h := NewHandle("abc")
	suite.assert.NotNil(h)
	suite.assert.Equal(InvalidHandleID, h.ID)
	suite.assert.Equal("abc", h.Path)
}

func (suite *HandleMapSuite) TestHandleFlags() {
	h := NewHandle("abc")
	suite.assert.NotNil(h)
	suite.assert.Equal(InvalidHandleID, h.ID)
	suite.assert.Equal("abc", h.Path)

	suite.assert.False(h.Dirty())
	suite.assert.False(h.Fsynced())
	suite.assert.False(h.Cached())
	suite.assert.Nil(h.GetFileObject())

	h.Flags.Set(HandleFlagDirty)
	suite.assert.True(h.Dirty())

	h.Flags.Set(HandleFlagFSynced)
	suite.assert.True(h.Fsynced())

	h.Flags.Set(HandleFlagCached)
	suite.assert.True(h.Cached())

	var f os.File
	h.SetFileObject(&f)
	suite.assert.NotNil(h.GetFileObject())
	suite.assert.Equal(h.GetFileObject(), &f)

	val, found := h.GetValue("123")
	suite.assert.False(found)
	suite.assert.Empty(val)

	h.SetValue("123", 1)
	val, found = h.GetValue("123")
	suite.assert.True(found)
	suite.assert.Equal(1, val)

	h.RemoveValue("123")
	val, found = h.GetValue("123")
	suite.assert.False(found)
	suite.assert.Empty(val)

	h.SetValue("123", 1)
	h.SetValue("456", 1)
	h.SetValue("789", 1)
	val, found = h.GetValue("123")
	suite.assert.True(found)
	suite.assert.Equal(1, val)
	h.Cleanup()

	val, found = h.GetValue("123")
	suite.assert.False(found)
	suite.assert.Empty(val)
}

func (suite *HandleMapSuite) TestHandleMap() {
	h := NewHandle("abc")
	suite.assert.NotNil(h)
	suite.assert.Equal(InvalidHandleID, h.ID)
	suite.assert.Equal("abc", h.Path)

	hmap := GetHandles()
	suite.assert.NotNil(hmap)

	hid := Add(h)
	suite.assert.NotZero(hid)
	suite.assert.Equal(h.ID, hid)

	nh, found := Load(hid)
	suite.assert.True(found)
	suite.assert.Equal(nh, h)

	nh, found = Load(123)
	suite.assert.False(found)
	suite.assert.Nil(nh)

	Delete(hid)
	nh, found = Load(hid)
	suite.assert.False(found)
	suite.assert.Nil(nh)

	suite.assert.Nil(h.CacheObj)
	CreateCacheObject(1, h)
	suite.assert.NotNil(h.CacheObj)

	nh = Store(123, "abc", 0)
	suite.assert.NotNil(nh)
	suite.assert.Nil(nh.CacheObj)
}
func TestUnMountCommand(t *testing.T) {
	suite.Run(t, new(HandleMapSuite))
}
