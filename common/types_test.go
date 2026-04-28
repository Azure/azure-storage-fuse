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

package common

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type typesTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *typesTestSuite) SetupTest() {
	suite.assert = assert.New(suite.T())
}

func TestGenerateConfig(t *testing.T) {
	suite.Run(t, new(typesTestSuite))
}

func (suite *typesTestSuite) TestBinarySearch() {
	blocksList := []*Block{
		{StartIndex: 0, EndIndex: 4},
		{StartIndex: 4, EndIndex: 7},
		{StartIndex: 7, EndIndex: 12},
	}
	bol := BlockOffsetList{
		BlockList: blocksList,
	}
	found, startingIndex := bol.BinarySearch(5)
	suite.assert.True(found)
	suite.assert.Equal(1, startingIndex)

	found, startingIndex = bol.BinarySearch(20)
	suite.assert.False(found)
	suite.assert.Equal(3, startingIndex)
}

func (suite *typesTestSuite) TestFindBlocksToModify() {
	blocksList := []*Block{
		{StartIndex: 0, EndIndex: 4},
		{StartIndex: 4, EndIndex: 7},
		{StartIndex: 7, EndIndex: 12},
	}
	bol := BlockOffsetList{
		BlockList: blocksList,
	}
	index, size, largerThanFile, _ := bol.FindBlocksToModify(3, 7)
	suite.assert.Equal(0, index)
	suite.assert.Equal(int64(12), size)
	suite.assert.False(largerThanFile)

	index, size, largerThanFile, _ = bol.FindBlocksToModify(8, 10)
	suite.assert.Equal(2, index)
	suite.assert.Equal(int64(5), size)
	suite.assert.True(largerThanFile)

	_, size, largerThanFile, appendOnly := bol.FindBlocksToModify(20, 20)
	suite.assert.Equal(int64(0), size)
	suite.assert.True(largerThanFile)
	suite.assert.True(appendOnly)
}

func (suite *typesTestSuite) TestDefaultWorkDir() {
	val, err := os.UserHomeDir()
	suite.assert.NoError(err)
	suite.assert.Equal(DefaultWorkDir, filepath.Join(val, ".blobfuse2"))
	suite.assert.Equal(DefaultLogFilePath, filepath.Join(val, ".blobfuse2/blobfuse2.log"))
	suite.assert.Equal(StatsConfigFilePath, filepath.Join(val, ".blobfuse2/stats_monitor.cfg"))
}

type layoutTestSuite struct {
	suite.Suite
}

// newLayout creates a Layout with the given ranges and marks it freshly fetched (i.e. valid).
func newLayout(ranges []LayoutRange) *Layout {
	l := &Layout{LayoutRanges: ranges}
	l.LastFetchedTime.Store(time.Now().Unix())
	return l
}

func (s *layoutTestSuite) TestIsValid_FreshlyFetched() {
	assert := assert.New(s.T())

	l := &Layout{}
	l.LastFetchedTime.Store(time.Now().Unix())
	assert.True(l.IsValid())
}

func (s *layoutTestSuite) TestIsValid_NeverFetched() {
	assert := assert.New(s.T())

	// Zero value: LastFetchedTime == 0 -> 1970-01-01, way past validity window.
	l := &Layout{}
	assert.False(l.IsValid())
}

func (s *layoutTestSuite) TestIsValid_Expired() {
	assert := assert.New(s.T())

	l := &Layout{}
	// Set fetched time to well outside the validity duration.
	l.LastFetchedTime.Store(time.Now().Add(-2 * layoutValidityDuration).Unix())
	assert.False(l.IsValid())
}

func (s *layoutTestSuite) TestIsValid_JustWithinWindow() {
	assert := assert.New(s.T())

	l := &Layout{}
	// 1 second inside the validity window.
	l.LastFetchedTime.Store(time.Now().Add(-(layoutValidityDuration - time.Second)).Unix())
	assert.True(l.IsValid())
}

func (s *layoutTestSuite) TestInvalidate() {
	assert := assert.New(s.T())

	l := &Layout{}
	l.LastFetchedTime.Store(time.Now().Unix())
	assert.True(l.IsValid())

	l.Invalidate()
	assert.False(l.IsValid())
	assert.EqualValues(0, l.LastFetchedTime.Load())
}

func (s *layoutTestSuite) TestGetIdealEndpoint_NilLayout() {
	assert := assert.New(s.T())

	var l *Layout
	assert.Equal("", l.GetIdealEndpoint(0))
	assert.Equal("", l.GetIdealEndpoint(1024))
}

func (s *layoutTestSuite) TestGetIdealEndpoint_EmptyRanges() {
	assert := assert.New(s.T())

	l := newLayout(nil)
	assert.Equal("", l.GetIdealEndpoint(0))

	l = newLayout([]LayoutRange{})
	assert.Equal("", l.GetIdealEndpoint(100))
}

func (s *layoutTestSuite) TestGetIdealEndpoint_ExpiredLayout() {
	assert := assert.New(s.T())

	l := &Layout{
		LayoutRanges: []LayoutRange{
			{Start: 0, End: 100, Endpoint: "ep1"},
		},
	}
	// LastFetchedTime not set -> invalid.
	assert.Equal("", l.GetIdealEndpoint(50))
}

func (s *layoutTestSuite) TestGetIdealEndpoint_SingleRange() {
	assert := assert.New(s.T())

	l := newLayout([]LayoutRange{
		{Start: 0, End: 100, Endpoint: "ep1"},
	})

	assert.Equal("ep1", l.GetIdealEndpoint(0))
	assert.Equal("ep1", l.GetIdealEndpoint(50))
	assert.Equal("ep1", l.GetIdealEndpoint(100))
	// Offset beyond last range still returns the last (only) range's endpoint.
	assert.Equal("ep1", l.GetIdealEndpoint(500))
}

func (s *layoutTestSuite) TestGetIdealEndpoint_MultipleRanges() {
	assert := assert.New(s.T())

	l := newLayout([]LayoutRange{
		{Start: 0, End: 99, Endpoint: "ep1"},
		{Start: 100, End: 199, Endpoint: "ep2"},
		{Start: 200, End: 299, Endpoint: "ep3"},
	})

	// Within first range.
	assert.Equal("ep1", l.GetIdealEndpoint(0))
	assert.Equal("ep1", l.GetIdealEndpoint(50))
	assert.Equal("ep1", l.GetIdealEndpoint(99))

	// Boundary: offset == 100 means first range with End >= 100 is range[1].
	assert.Equal("ep2", l.GetIdealEndpoint(100))
	assert.Equal("ep2", l.GetIdealEndpoint(150))
	assert.Equal("ep2", l.GetIdealEndpoint(199))

	// Within third range.
	assert.Equal("ep3", l.GetIdealEndpoint(200))
	assert.Equal("ep3", l.GetIdealEndpoint(250))
	assert.Equal("ep3", l.GetIdealEndpoint(299))

	// Offset past last range -> falls back to last range's endpoint.
	assert.Equal("ep3", l.GetIdealEndpoint(1000))
}

func (s *layoutTestSuite) TestGetIdealEndpoint_NegativeOffset() {
	assert := assert.New(s.T())

	l := newLayout([]LayoutRange{
		{Start: 0, End: 99, Endpoint: "ep1"},
		{Start: 100, End: 199, Endpoint: "ep2"},
	})

	// Negative offset is <= every range's End, so binary search returns the first range.
	assert.Equal("ep1", l.GetIdealEndpoint(-1))
	assert.Equal("ep1", l.GetIdealEndpoint(-1000))
}

func (s *layoutTestSuite) TestGetIdealEndpoint_ManyRanges() {
	assert := assert.New(s.T())

	const n = 50
	const blockSize = int64(1024)
	ranges := make([]LayoutRange, 0, n)
	for i := 0; i < n; i++ {
		ranges = append(ranges, LayoutRange{
			Start:    int64(i) * blockSize,
			End:      int64(i+1)*blockSize - 1,
			Endpoint: "ep" + strconv.Itoa(i),
		})
	}
	l := newLayout(ranges)

	// Spot-check each range start, middle, and end.
	for i := 0; i < n; i++ {
		want := "ep" + strconv.Itoa(i)
		assert.Equal(want, l.GetIdealEndpoint(int64(i)*blockSize))
		assert.Equal(want, l.GetIdealEndpoint(int64(i)*blockSize+blockSize/2))
		assert.Equal(want, l.GetIdealEndpoint(int64(i+1)*blockSize-1))
	}

	// Offset past the end -> last range.
	assert.Equal("ep"+strconv.Itoa(n-1), l.GetIdealEndpoint(int64(n)*blockSize+1))
}

func TestLayoutTestSuite(t *testing.T) {
	suite.Run(t, new(layoutTestSuite))
}
