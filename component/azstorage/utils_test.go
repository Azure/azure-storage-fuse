package azstorage

import (
	"testing"

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

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}
