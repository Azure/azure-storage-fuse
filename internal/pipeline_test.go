// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package internal

import (
	"context"
	"testing"

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

func (s *pipelineTestSuite) SetupTest() {
	AddComponent("ComponentA", NewComponentA)
	AddComponent("ComponentB", NewComponentB)
	AddComponent("ComponentC", NewComponentC)
	AddComponent("stream", NewComponentStream)
	AddComponent("block_cache", NewComponentBlockCache)
	s.assert = assert.New(s.T())
}

func (s *pipelineTestSuite) TestCreatePipeline() {
	_, err := NewPipeline([]string{"ComponentA", "ComponentB"}, false)
	s.assert.NoError(err)
}

func (s *pipelineTestSuite) TestCreateInvalidPipeline() {
	_, err := NewPipeline([]string{"ComponentC", "ComponentA"}, false)
	s.assert.Error(err)
	s.assert.Contains(err.Error(), "is out of order")

}

func (s *pipelineTestSuite) TestInvalidComponent() {
	_, err := NewPipeline([]string{"ComponentD"}, false)
	s.assert.Error(err)
}

func (s *pipelineTestSuite) TestStartStopCreateNewPipeline() {
	p, err := NewPipeline([]string{"ComponentA", "ComponentB"}, false)
	s.assert.NoError(err)
	print(p.components[0].Name())
	err = p.Start(context.Background())
	s.assert.NoError(err)

	err = p.Stop()
	s.assert.NoError(err)
}

func (s *pipelineTestSuite) TestStreamToBlockCacheConfig() {
	p, err := NewPipeline([]string{"stream"}, false)
	s.assert.NoError(err)
	s.assert.Equal("block_cache", p.components[0].Name())
}

func TestPipelineTestSuite(t *testing.T) {
	suite.Run(t, new(pipelineTestSuite))
}
