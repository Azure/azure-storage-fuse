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

package contract

import (
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/xload/core"
)

type Xcomponent interface {
	Init()
	Start()
	Stop()
	Process(item *core.WorkItem) (int, error)
	GetNext() Xcomponent
	SetNext(s Xcomponent)
	GetThreadPool() *core.ThreadPool
	GetRemote() internal.Component
	GetName() string
	SetName(s string)
	SetRemote(remote internal.Component)
	SetThreadPool(pool *core.ThreadPool)
}

type Xbase struct {
	name   string
	pool   *core.ThreadPool
	remote internal.Component
	next   Xcomponent
}

// SetThreadPool implements Xcomponent.
func (xb *Xbase) SetThreadPool(pool *core.ThreadPool) {
	xb.pool = pool
}

// SetRemote implements Xcomponent.
func (xb *Xbase) SetRemote(remote internal.Component) {
	xb.remote = remote
}

var _ Xcomponent = &Xbase{}

func (xb *Xbase) Init() {
}

func (xb *Xbase) Start() {
}

func (xb *Xbase) Stop() {
}

func (xb *Xbase) Process(item *core.WorkItem) (int, error) {
	return 0, nil
}

func (xb *Xbase) GetNext() Xcomponent {
	return xb.next
}

func (xb *Xbase) SetNext(s Xcomponent) {
	xb.next = s
}

func (xb *Xbase) GetThreadPool() *core.ThreadPool {
	return xb.pool
}

func (xb *Xbase) GetRemote() internal.Component {
	return xb.remote
}

func (xb *Xbase) GetName() string {
	return xb.name
}

func (xb *Xbase) SetName(s string) {
	xb.name = s
}
