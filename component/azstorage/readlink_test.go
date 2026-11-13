// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azstorage

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/stretchr/testify/assert"
)

// fakeReadLinkConn serves a symlink blob of blobSize bytes and records the calls it receives.
type fakeReadLinkConn struct {
	AzConnection
	blobSize      int64
	readErr       error
	getAttrCalls  int
	requestedRead []int64
}

func (f *fakeReadLinkConn) GetAttr(name string) (*internal.ObjAttr, error) {
	f.getAttrCalls++
	return &internal.ObjAttr{Size: f.blobSize}, nil
}

func (f *fakeReadLinkConn) ReadBuffer(name string, offset int64, length int64) ([]byte, error) {
	f.requestedRead = append(f.requestedRead, length)
	if f.readErr != nil {
		return nil, f.readErr
	}
	if length == 0 {
		length = f.blobSize - offset
	}
	if length > 1<<20 {
		return nil, fmt.Errorf("unbounded symlink read of %d bytes", length)
	}
	buf := make([]byte, length)
	for i := range buf {
		buf[i] = 'a'
	}
	return buf, nil
}

func TestReadLinkBoundsDownloadSize(t *testing.T) {
	const huge = int64(64 << 30)

	tests := []struct {
		name     string
		blobSize int64
		optSize  int64
		wantLen  int
		wantRead []int64
	}{
		{name: "small target", blobSize: 6, optSize: 6, wantLen: 6, wantRead: []int64{6}},
		{name: "max target", blobSize: common.MaxSymlinkTargetLen, optSize: common.MaxSymlinkTargetLen,
			wantLen: common.MaxSymlinkTargetLen, wantRead: []int64{common.MaxSymlinkTargetLen}},
		{name: "huge target", blobSize: huge, optSize: huge,
			wantLen: common.MaxSymlinkTargetLen, wantRead: []int64{common.MaxSymlinkTargetLen}},
		{name: "empty target", blobSize: 0, optSize: 0, wantLen: 0, wantRead: nil},
		// A stale size of 0 must not turn into a download of the whole blob.
		{name: "zero size for huge blob", blobSize: huge, optSize: 0, wantLen: 0, wantRead: nil},
		{name: "negative size", blobSize: 6, optSize: -1, wantLen: 0, wantRead: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &fakeReadLinkConn{blobSize: tt.blobSize}
			az := &AzStorage{storage: conn}

			target, err := az.ReadLink(internal.ReadLinkOptions{Name: "link", Size: tt.optSize})
			assert.NoError(t, err)
			assert.Len(t, target, tt.wantLen)
			assert.Equal(t, tt.wantRead, conn.requestedRead)
			assert.Zero(t, conn.getAttrCalls)
		})
	}
}

func TestReadLinkReadError(t *testing.T) {
	conn := &fakeReadLinkConn{blobSize: 6, readErr: syscall.ENOENT}
	az := &AzStorage{storage: conn}

	_, err := az.ReadLink(internal.ReadLinkOptions{Name: "link", Size: 6})
	assert.ErrorIs(t, err, syscall.ENOENT)
	assert.Zero(t, conn.getAttrCalls)
}
