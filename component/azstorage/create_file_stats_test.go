// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

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
