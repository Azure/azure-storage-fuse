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

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type versionTestSuite struct {
	suite.Suite
}

func (vSuite *versionTestSuite) TestVersionEquality() {
	assert := assert.New(vSuite.T())

	v1, _ := ParseVersion("10.0.0")
	v2, _ := ParseVersion("10.0.0")
	assert.Equal(v1.compare(*v2), 0)

	v1, _ = ParseVersion("10.0.0-preview.1")
	v2, _ = ParseVersion("10.0.0-preview.1")
	assert.Equal(v1.compare(*v2), 0)

	v1, _ = ParseVersion("10.0.0-beta.5")
	v2, _ = ParseVersion("10.0.0-beta.5")
	assert.Equal(v1.compare(*v2), 0)

	v1, _ = ParseVersion("10.0.0~preview.1")
	v2, _ = ParseVersion("10.0.0~preview.1")
	assert.Equal(v1.compare(*v2), 0)

	v1, _ = ParseVersion("10.0.0~beta.5")
	v2, _ = ParseVersion("10.0.0~beta.5")
	assert.Equal(v1.compare(*v2), 0)
}

func (vSuite *versionTestSuite) TestVersionSuperiority() {
	assert := assert.New(vSuite.T())

	v1, _ := ParseVersion("11.3.0")
	v2, _ := ParseVersion("10.8.3")
	assert.Equal(v1.compare(*v2), 1)

	v1, _ = ParseVersion("15.5.6")
	v2, _ = ParseVersion("15.3.5")
	assert.Equal(v1.compare(*v2), 1)

	v1, _ = ParseVersion("15.5.6")
	v2, _ = ParseVersion("15.5.5")
	assert.Equal(v1.compare(*v2), 1)

	v1, _ = ParseVersion("15.5.5")
	v2, _ = ParseVersion("15.5.5-preview.3")
	assert.Equal(v1.compare(*v2), 1)

	v1, _ = ParseVersion("15.5.5-preview.6")
	v2, _ = ParseVersion("15.5.5-preview.3")
	assert.Equal(v1.compare(*v2), 1)

	v1, _ = ParseVersion("15.5.6")
	v2, _ = ParseVersion("15.5.6~preview.3")
	assert.Equal(v1.compare(*v2), 1)

	v1, _ = ParseVersion("15.5.6~preview.6")
	v2, _ = ParseVersion("15.5.6~preview.3")
	assert.Equal(v1.compare(*v2), 1)

	v1, _ = ParseVersion("15.5.7~preview.6")
	v2, _ = ParseVersion("15.5.7-preview.3")
	assert.Equal(v1.compare(*v2), 1)
}

func (vSuite *versionTestSuite) TestVersionInferiority() {
	assert := assert.New(vSuite.T())

	v1, _ := ParseVersion("10.5.6")
	v2, _ := ParseVersion("11.8.3")
	assert.Equal(v1.compare(*v2), -1)

	v1, _ = ParseVersion("15.3.6")
	v2, _ = ParseVersion("15.5.5")
	assert.Equal(v1.compare(*v2), -1)

	v1, _ = ParseVersion("15.5.5")
	v2, _ = ParseVersion("15.5.6")
	assert.Equal(v1.compare(*v2), -1)

	v1, _ = ParseVersion("15.5.5-preview.6")
	v2, _ = ParseVersion("15.5.5")
	assert.Equal(v1.compare(*v2), -1)

	v1, _ = ParseVersion("15.5.5-preview.3")
	v2, _ = ParseVersion("15.5.5-preview.6")
	assert.Equal(v1.compare(*v2), -1)

	v1, _ = ParseVersion("15.5.6~preview.6")
	v2, _ = ParseVersion("15.5.6")
	assert.Equal(v1.compare(*v2), -1)

	v1, _ = ParseVersion("15.5.6~preview.3")
	v2, _ = ParseVersion("15.5.6~preview.6")
	assert.Equal(v1.compare(*v2), -1)

	v1, _ = ParseVersion("15.5.7-preview.3")
	v2, _ = ParseVersion("15.5.7~preview.6")
	assert.Equal(v1.compare(*v2), -1)
}

func TestVersionTestSuite(t *testing.T) {
	suite.Run(t, new(versionTestSuite))
}
