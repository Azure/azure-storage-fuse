/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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
	suite.assert.Equal(tp.worker, uint32(1))
}

func (suite *threadPoolTestSuite) TestThreadPoolStartStop() {
	suite.assert = assert.New(suite.T())

	r := func(i *WorkItem) (int, error) {
		return 0, nil
	}

	tp := NewThreadPool(2, r)
	suite.assert.NotNil(tp)
	suite.assert.Equal(tp.worker, uint32(2))

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
	suite.assert.Equal(tp.worker, uint32(2))

	tp.Start(context.TODO())
	suite.assert.NotNil(tp.priorityItems)
	suite.assert.NotNil(tp.workItems)

	tp.Schedule(&WorkItem{Priority: true})
	tp.Schedule(&WorkItem{})

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
	suite.assert.Equal(tp.worker, uint32(10))

	tp.Start(context.TODO())
	suite.assert.NotNil(tp.priorityItems)
	suite.assert.NotNil(tp.workItems)

	for i := range 100 {
		if i < 20 {
			tp.Schedule(&WorkItem{Priority: true})
		} else {
			tp.Schedule(&WorkItem{})
		}

	}

	time.Sleep(1 * time.Second)
	suite.assert.Equal(callbackCnt, int32(100))
	tp.Stop()
}

func TestThreadPoolSuite(t *testing.T) {
	suite.Run(t, new(threadPoolTestSuite))
}
