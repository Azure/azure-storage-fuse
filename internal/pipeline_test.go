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

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Test components
type ComponentA struct {
	BaseComponent
}

func (ac *ComponentA) Priority() ComponentPriority {
	return EComponentPriority.Producer()
}

func NewComponentA() Component {
	return &ComponentA{}
}

type ComponentB struct {
	BaseComponent
}

func (ac *ComponentB) Priority() ComponentPriority {
	return EComponentPriority.LevelMid()
}

func NewComponentB() Component {
	return &ComponentB{}
}

type ComponentC struct {
	BaseComponent
}

func (ac *ComponentC) Priority() ComponentPriority {
	return EComponentPriority.Consumer()
}

func NewComponentC() Component {
	return &ComponentC{}
}

type ComponentStream struct {
	BaseComponent
}

func NewComponentStream() Component {
	comp := &ComponentStream{}
	comp.SetName("stream")
	return comp
}

type ComponentBlockCache struct {
	BaseComponent
}

func NewComponentBlockCache() Component {
	comp := &ComponentBlockCache{}
	comp.SetName("block_cache")
	return comp
}

/////////////////////////////////////////

type pipelineTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *pipelineTestSuite) SetupTest() {
	AddComponent("ComponentA", NewComponentA)
	AddComponent("ComponentB", NewComponentB)
	AddComponent("ComponentC", NewComponentC)
	AddComponent("stream", NewComponentStream)
	AddComponent("block_cache", NewComponentBlockCache)
	suite.assert = assert.New(suite.T())
}

func (s *pipelineTestSuite) TestCreatePipeline() {
	_, err := NewPipeline([]string{"ComponentA", "ComponentB"}, false)
	s.assert.Nil(err)
}

func (s *pipelineTestSuite) TestCreateInvalidPipeline() {
	_, err := NewPipeline([]string{"ComponentC", "ComponentA"}, false)
	s.assert.NotNil(err)
	s.assert.Contains(err.Error(), "is out of order")

}

func (s *pipelineTestSuite) TestInvalidComponent() {
	_, err := NewPipeline([]string{"ComponentD"}, false)
	s.assert.NotNil(err)
}

func (s *pipelineTestSuite) TestStartStopCreateNewPipeline() {
	p, err := NewPipeline([]string{"ComponentA", "ComponentB"}, false)
	s.assert.Nil(err)
	print(p.components[0].Name())
	err = p.Start(nil)
	s.assert.Nil(err)

	err = p.Stop()
	s.assert.Nil(err)
}

func (s *pipelineTestSuite) TestNewPipelineWithGenConfig() {
	common.GenConfig = true
	defer func() { common.GenConfig = false }()
	p, err := NewPipeline([]string{"ComponentA", "ComponentB", "ComponentC"}, false)
	s.assert.Nil(err)
	s.assert.Equal(0, len(p.components))
}

func (s *pipelineTestSuite) TestStreamToBlockCacheConfig() {
	p, err := NewPipeline([]string{"stream"}, false)
	s.assert.Nil(err)
	s.assert.Equal(p.components[0].Name(), "block_cache")
}

func TestPipelineTestSuite(t *testing.T) {
	suite.Run(t, new(pipelineTestSuite))
}
