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

package common

import (
	"log"
	"os"
)

var hostname string

// Assert can be used to assert any condition. It'll cause the program to terminate.
// Apart from the assertion condition it takes a variable number of items to print, which would mostly be a
// message and/or err variable and optionally one or more relevant variables.
// In non-debug builds it's a no-op.
//
// Examples:
//
//	Assert(err == nil, "Unexpected error return", err)
//	Assert(isValid == true)
//	Assert((value >= 0 && value <= 100), "Invalid percentage", value)

func Assert(cond bool, msg ...interface{}) {
	if !IsDebugBuild() {
		return
	}
	if !cond {
		//
		// Logging hostname helps when running multiple instances of blobfuse on different hosts
		// from one terminal.
		//
		if len(msg) != 0 {
			log.Panicf("[%s] Assertion failed: %v", hostname, msg)
		} else {
			log.Panicf("[%s] Assertion failed!", hostname)
		}
	}
}

// IsDebugBuild can be used to test if we are running in a debug environment.
// Note: Use this sparingly only to do stuff that we know for sure doesn't change the behavior of the program.
//
//	We need to be very careful in making sure debug build behaves same as prod builds for reliable testing.
var isDebugBuild bool

func IsDebugBuild() bool {
	return isDebugBuild
}

func init() {
	var err error

	hostname, err = os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	isDebugBuild = (os.Getenv("BLOBFUSE_DEBUG") == "1")
}
