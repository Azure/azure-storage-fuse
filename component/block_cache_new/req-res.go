package block_cache_new

import (
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// TODO: handle O_TRUNC this call should be sequentialized like the truncate
type open_req struct {
	attr  *internal.ObjAttr
	flags int
	bl    blockList
}

type open_res struct {
	h   *handlemap.Handle
	err error
}

func CreateOpenReq(attr *internal.ObjAttr, flags int, bl blockList) *open_req {
	return &open_req{attr: attr, flags: flags, bl: bl}
}

func CreateEmptyOpenRes() *open_res {
	return &open_res{h: nil, err: nil}
}

type read_req struct {
	h      *handlemap.Handle
	data   []byte
	offset int64
}

type read_res struct {
	bytesRead int
	err       error
}

func CreateReadReq(h *handlemap.Handle, data []byte, offset int64) *read_req {
	return &read_req{h: h, data: data, offset: offset}
}

type write_req struct {
	h      *handlemap.Handle
	data   []byte
	offset int64
}

type write_res struct {
	bytesWritten int
	err          error
}

func CreateWriteReq(h *handlemap.Handle, data []byte, offset int64) *write_req {
	return &write_req{h: h, data: data, offset: offset}
}

type sync_req struct {
	h *handlemap.Handle
}

type sync_res struct {
	err error
}

func CreateSyncReq(h *handlemap.Handle) *sync_req {
	return &sync_req{h: h}
}
