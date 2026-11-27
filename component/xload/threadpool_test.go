// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package xload

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type threadPoolTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *threadPoolTestSuite) TestThreadPoolCreate() {
	suite.assert = assert.New(suite.T())

	tp := NewThreadPool(0, nil)
	suite.assert.Nil(tp)

	tp = NewThreadPool(1, nil)
	suite.assert.Nil(tp)

	tp = NewThreadPool(1, func(*WorkItem) (int, error) {
		return 0, nil
	})
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(1), tp.worker)
}

func (suite *threadPoolTestSuite) TestThreadPoolStartStop() {
	suite.assert = assert.New(suite.T())

	r := func(i *WorkItem) (int, error) {
		return 0, nil
	}

	tp := NewThreadPool(2, r)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(2), tp.worker)

	tp.Start(context.TODO())
	suite.assert.NotNil(tp.priorityItems)
	suite.assert.NotNil(tp.workItems)

	tp.Stop()
}

func (suite *threadPoolTestSuite) TestThreadPoolSchedule() {
	suite.assert = assert.New(suite.T())

	r := func(i *WorkItem) (int, error) {
		return 0, nil
	}

	tp := NewThreadPool(2, r)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(2), tp.worker)

	tp.Start(context.TODO())
	suite.assert.NotNil(tp.priorityItems)
	suite.assert.NotNil(tp.workItems)

	err := tp.Schedule(&WorkItem{Priority: true})
	suite.assert.NoError(err)
	err = tp.Schedule(&WorkItem{})
	suite.assert.NoError(err)

	time.Sleep(1 * time.Second)
	tp.Stop()
}

func (suite *threadPoolTestSuite) TestPrioritySchedule() {
	suite.assert = assert.New(suite.T())

	callbackCnt := int32(0)
	r := func(i *WorkItem) (int, error) {
		atomic.AddInt32(&callbackCnt, 1)
		return 0, nil
	}

	tp := NewThreadPool(10, r)
	suite.assert.NotNil(tp)
	suite.assert.Equal(uint32(10), tp.worker)

	tp.Start(context.TODO())
	suite.assert.NotNil(tp.priorityItems)
	suite.assert.NotNil(tp.workItems)

	for i := range 100 {
		if i < 20 {
			err := tp.Schedule(&WorkItem{Priority: true})
			suite.assert.NoError(err)
		} else {
			err := tp.Schedule(&WorkItem{})
			suite.assert.NoError(err)
		}

	}

	time.Sleep(1 * time.Second)
	suite.assert.Equal(int32(100), callbackCnt)
	tp.Stop()
}

func TestThreadPoolSuite(t *testing.T) {
	suite.Run(t, new(threadPoolTestSuite))
}
