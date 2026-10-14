//go:build !unittest
// +build !unittest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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
   SOFTWARE.
*/

package dcache_e2e

import (
	"slices"
	"testing"
)

func TestParsePodState(t *testing.T) {
	data := []byte(`{
		"items": [
			{
				"metadata": {"name": "ready"},
				"status": {"phase": "Running"}
			},
			{
				"metadata": {
					"name": "terminating",
					"deletionTimestamp": "2026-10-04T15:48:30Z"
				},
				"status": {"phase": "Running"}
			},
			{
				"metadata": {"name": "pending"},
				"status": {"phase": "Pending"}
			},
			{
				"metadata": {"name": "historical-failure"},
				"status": {"phase": "Failed"}
			}
		]
	}`)

	live, total, err := parsePodState(data)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total pods = %d, want 3", total)
	}
	if !slices.Equal(live, []string{"ready"}) {
		t.Fatalf("live pods = %v, want [ready]", live)
	}
}

func TestParsePodStateInvalidJSON(t *testing.T) {
	if _, _, err := parsePodState([]byte(`{"items":`)); err == nil {
		t.Fatal("parsePodState accepted invalid JSON")
	}
}
