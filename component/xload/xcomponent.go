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

package xload

import "github.com/Azure/azure-storage-fuse/v2/internal"

type xcomponent interface {
	init()
	start()
	stop()
	process(item *workItem) (int, error)
	getNext() xcomponent
	setNext(s xcomponent)
	getThreadPool() *ThreadPool
	getRemote() internal.Component
	getName() string
	setName(s string)
}

type xbase struct {
	name     string
	pool     *ThreadPool
	remote   internal.Component
	next     xcomponent
	statsMgr *statsManager
}

var _ xcomponent = &xbase{}

func (xb *xbase) init() {
}

func (xb *xbase) start() {
}

func (xb *xbase) stop() {
}

func (xb *xbase) process(item *workItem) (int, error) {
	return 0, nil
}

func (xb *xbase) getNext() xcomponent {
	return xb.next
}

func (xb *xbase) setNext(s xcomponent) {
	xb.next = s
}

func (xb *xbase) getThreadPool() *ThreadPool {
	return xb.pool
}

func (xb *xbase) getRemote() internal.Component {
	return xb.remote
}

func (xb *xbase) getName() string {
	return xb.name
}

func (xb *xbase) setName(s string) {
	xb.name = s
}

func (xb *xbase) getStatsManager() *statsManager {
	return xb.statsMgr
}
