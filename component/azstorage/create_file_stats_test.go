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

package azstorage

import (
	"errors"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
	"github.com/stretchr/testify/assert"
)

// fakeCreateFileConn records CreateFile calls and fails them with createErr when it is set.
type fakeCreateFileConn struct {
	AzConnection
	createErr error
	created   []string
}

func (f *fakeCreateFileConn) CreateFile(name string, mode os.FileMode) error {
	f.created = append(f.created, name)
	return f.createErr
}

func TestCreateFileUpdatesStats(t *testing.T) {
	tests := []struct {
		name      string
		createErr error
		wantCount any
	}{
		{name: "success", wantCount: int64(1)},
		{name: "create error", createErr: errors.New("failed to create file")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats_manager.EnableForTest(t)
			prev := azStatsCollector
			azStatsCollector = stats_manager.NewStatsCollector(compName)
			t.Cleanup(func() { azStatsCollector = prev })

			conn := &fakeCreateFileConn{createErr: tt.createErr}
			az := &AzStorage{storage: conn}

			handle, err := az.CreateFile(internal.CreateFileOptions{Name: "file", Mode: 0644})
			if tt.createErr != nil {
				assert.ErrorIs(t, err, tt.createErr)
				assert.Nil(t, handle)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, handle)
			}
			assert.Equal(t, []string{"file"}, conn.created)

			stats := azStatsCollector.StopAndGetStats()
			assert.Equal(t, tt.wantCount, stats[createFile])
		})
	}
}
