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
   SOFTWARE
*/

package stats_manager

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
)

// EnableForTest turns on blobfuse stats collection for the duration of a unit test.
// Events are written to a regular file in a temporary directory instead of the health-monitor pipe,
// and the polling loop that waits for the health monitor is not started.
// The previous monitoring settings are restored when the test finishes.
func EnableForTest(t testing.TB) {
	t.Helper()

	transferFile := filepath.Join(t.TempDir(), "transferPipe")
	f, err := os.Create(transferFile)
	if err != nil {
		t.Fatalf("stats_manager::EnableForTest : unable to create transfer file [%v]", err)
	}
	_ = f.Close()

	stMgrOpt.pollMtx.Lock()
	pollStarted := stMgrOpt.pollStarted
	stMgrOpt.pollStarted = true
	stMgrOpt.pollMtx.Unlock()

	enableMonitoring, bfsDisabled, transferPipe := common.EnableMonitoring, common.BfsDisabled, common.TransferPipe
	common.EnableMonitoring, common.BfsDisabled, common.TransferPipe = true, false, transferFile

	t.Cleanup(func() {
		common.EnableMonitoring, common.BfsDisabled, common.TransferPipe = enableMonitoring, bfsDisabled, transferPipe

		stMgrOpt.pollMtx.Lock()
		stMgrOpt.pollStarted = pollStarted
		stMgrOpt.pollMtx.Unlock()
	})
}

// StopAndGetStats stops the collector once it has processed every queued update and returns a copy of its
// aggregate stats. It returns nil if the collector was created while monitoring was disabled.
// The collector must not be used after this call.
func (sc *StatsCollector) StopAndGetStats() map[string]any {
	if sc == nil || sc.channel == nil {
		return nil
	}

	sc.Destroy()

	stMgrOpt.statsMtx.Lock()
	defer stMgrOpt.statsMtx.Unlock()
	return maps.Clone(stMgrOpt.statsList[sc.compIdx].Value)
}
