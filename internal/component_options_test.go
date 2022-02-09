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
