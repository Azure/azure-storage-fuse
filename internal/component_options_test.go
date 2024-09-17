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

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type componentOptionsTestSuite struct {
	suite.Suite
}

func (s *componentOptionsTestSuite) TestExtendDirName() {
	assert := assert.New(s.T())
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{input: "dir", expectedOutput: "dir/"},
		{input: "dir/", expectedOutput: "dir/"},
		{input: "", expectedOutput: "/"},
	}
	for _, tt := range tests {
		s.Run(tt.input, func() {
			output := ExtendDirName(tt.input)
			assert.EqualValues(tt.expectedOutput, output)
		})
	}
}

func (s *componentOptionsTestSuite) TestTruncateDirName() {
	assert := assert.New(s.T())
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{input: "dir/", expectedOutput: "dir"},
		{input: "dir", expectedOutput: "dir"},
		{input: "/", expectedOutput: ""},
	}
	for _, tt := range tests {
		s.Run(tt.input, func() {
			output := TruncateDirName(tt.input)
			assert.EqualValues(tt.expectedOutput, output)
		})
	}
}

func TestComponentOptionsTestSuite(t *testing.T) {
	suite.Run(t, new(componentOptionsTestSuite))
}
