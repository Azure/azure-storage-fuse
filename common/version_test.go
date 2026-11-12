// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
	assert.Equal(0, v1.compare(*v2))

	v1, _ = ParseVersion("10.0.0-preview.1")
	v2, _ = ParseVersion("10.0.0-preview.1")
	assert.Equal(0, v1.compare(*v2))

	v1, _ = ParseVersion("10.0.0-beta.5")
	v2, _ = ParseVersion("10.0.0-beta.5")
	assert.Equal(0, v1.compare(*v2))

	v1, _ = ParseVersion("10.0.0~preview.1")
	v2, _ = ParseVersion("10.0.0~preview.1")
	assert.Equal(0, v1.compare(*v2))

	v1, _ = ParseVersion("10.0.0~beta.5")
	v2, _ = ParseVersion("10.0.0~beta.5")
	assert.Equal(0, v1.compare(*v2))
}

func (vSuite *versionTestSuite) TestVersionPreview() {
	assert := assert.New(vSuite.T())

	stable, err := ParseVersion("10.0.0")
	assert.NoError(err)
	assert.False(stable.IsPreview())

	hyphenPreview, err := ParseVersion("10.0.0-preview.1")
	assert.NoError(err)
	assert.True(hyphenPreview.IsPreview())

	tildePreview, err := ParseVersion("10.0.0~preview.1")
	assert.NoError(err)
	assert.True(tildePreview.IsPreview())
}

func (vSuite *versionTestSuite) TestVersionSuperiority() {
	assert := assert.New(vSuite.T())

	v1, _ := ParseVersion("11.3.0")
	v2, _ := ParseVersion("10.8.3")
	assert.Equal(1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.6")
	v2, _ = ParseVersion("15.3.5")
	assert.Equal(1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.6")
	v2, _ = ParseVersion("15.5.5")
	assert.Equal(1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.5")
	v2, _ = ParseVersion("15.5.5-preview.3")
	assert.Equal(1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.5-preview.6")
	v2, _ = ParseVersion("15.5.5-preview.3")
	assert.Equal(1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.6")
	v2, _ = ParseVersion("15.5.6~preview.3")
	assert.Equal(1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.6~preview.6")
	v2, _ = ParseVersion("15.5.6~preview.3")
	assert.Equal(1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.7~preview.6")
	v2, _ = ParseVersion("15.5.7-preview.3")
	assert.Equal(1, v1.compare(*v2))
}

func (vSuite *versionTestSuite) TestVersionInferiority() {
	assert := assert.New(vSuite.T())

	v1, _ := ParseVersion("10.5.6")
	v2, _ := ParseVersion("11.8.3")
	assert.Equal(-1, v1.compare(*v2))

	v1, _ = ParseVersion("15.3.6")
	v2, _ = ParseVersion("15.5.5")
	assert.Equal(-1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.5")
	v2, _ = ParseVersion("15.5.6")
	assert.Equal(-1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.5-preview.6")
	v2, _ = ParseVersion("15.5.5")
	assert.Equal(-1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.5-preview.3")
	v2, _ = ParseVersion("15.5.5-preview.6")
	assert.Equal(-1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.6~preview.6")
	v2, _ = ParseVersion("15.5.6")
	assert.Equal(-1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.6~preview.3")
	v2, _ = ParseVersion("15.5.6~preview.6")
	assert.Equal(-1, v1.compare(*v2))

	v1, _ = ParseVersion("15.5.7-preview.3")
	v2, _ = ParseVersion("15.5.7~preview.6")
	assert.Equal(-1, v1.compare(*v2))
}

func TestVersionTestSuite(t *testing.T) {
	suite.Run(t, new(versionTestSuite))
}
