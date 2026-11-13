// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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

func (suite *HandleMapSuite) TestHandleDirtyWhileFlushing() {
	h := NewHandle("abc")
	suite.assert.False(h.Dirty())

	// Changes being uploaded are not persisted yet, so the handle is still dirty
	h.Flags.Set(HandleFlagFlushing)
	suite.assert.True(h.Dirty())

	h.Flags.Set(HandleFlagDirty)
	h.Flags.Clear(HandleFlagFlushing)
	suite.assert.True(h.Dirty())

	h.Flags.Clear(HandleFlagDirty)
	suite.assert.False(h.Dirty())
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
