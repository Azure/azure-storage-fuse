package block_cache_new

import (
	"io"

	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// Type of Transaction :create, Open, Read, Write, Truncate, Flush
//
//	Trans Id: 	0, 	  1,     2,        3,     4, 		5
type Transaction struct {
	type_of  int // Describing type of the request
	id       int // Transaction Id
	request  any
	response chan any
}

func CreateTransaction(type_of int) Transaction {
	return Transaction{type_of: type_of, response: make(chan any)}
}

func HandleTransaction(file *File, t *Transaction) {
	switch t.type_of {
	case 1:
		serve_open(file, t)
	case 2:
		serve_read(file, t)
	case 3:
		serve_write(file, t)
	case 4:
		serve_sync(file, t)
	}
}

func serve_open(file *File, t *Transaction) {
	req := t.request.(*open_req)
	res := CreateEmptyOpenRes()

	file.Lock()
	if file.size == -1 {
		populateFileInfo(file, req.attr)
	}
	handle := CreateFreshHandleForFile(file.Name, file.size, req.attr.Mtime)
	file.handles[handle] = true
	res.h = handle
	file.blockList = req.bl
	file.Unlock()

	t.response <- res
}

func serve_read(file *File, t *Transaction) {
	req := t.request.(*read_req)
	res := &read_res{bytesRead: 0, err: nil}
	//There may be atmost 2 blocks involved in worst case while reading the data from buffers.
	//maybe getting the 2 concurrently might improve latency.
	// l := getBlockIndex(req.offset)
	// r := getBlockIndex(req.offset + int64(len(req.data)))

	// for i := l; i < r; i++ {
	// 	GetBlock(i, file)
	// }
	offset := req.offset
	dataRead := 0
	len_of_copy := len(req.data)
	for dataRead < len_of_copy {
		idx := getBlockIndex(offset)
		block_buf, err := getBlockForRead(idx, req.h, file)
		if err != nil {
			res.err = err
			break
		}
		blockOffset := convertOffsetIntoBlockOffset(offset)

		block_buf.RLock()
		len_of_block_buf := block_buf.dataSize
		bytesCopied := copy(req.data[dataRead:], block_buf.data[blockOffset:len_of_block_buf])
		block_buf.RUnlock()

		dataRead += bytesCopied
		offset += int64(bytesCopied)
		if offset >= file.size { //this should be protected by lock ig, idk
			res.err = io.EOF
			break
		}
	}
	res.bytesRead = dataRead
	t.response <- res
}

func serve_write(file *File, t *Transaction) {
	req := t.request.(*write_req)
	res := &write_res{bytesWritten: 0, err: nil}

	offset := req.offset
	len_of_copy := len(req.data)
	dataWritten := 0
	for dataWritten < len_of_copy {
		idx := getBlockIndex(offset)
		block_buf, err := getBlockForWrite(idx, req.h, file)
		if err != nil {
			res.err = err
			break
		}
		blockOffset := convertOffsetIntoBlockOffset(offset)

		block_buf.Lock()
		bytesCopied := copy(block_buf.data[blockOffset:BlockSize], req.data[dataWritten:])
		block_buf.synced = 0
		block_buf.Unlock()

		dataWritten += bytesCopied
		offset += int64(dataWritten)
		//Update the file size if it fall outside
		file.Lock()
		if offset > file.size {
			file.size = offset
		}
		file.Unlock()
	}

	res.bytesWritten = dataWritten
	t.response <- res
}

func serve_sync(file *File, t *Transaction) {
	req := t.request.(*sync_req)
	res := &sync_res{err: nil}

	err := syncBuffersForFile(req.h, file)
	if err == nil {
		err = commitBuffersForFile(req.h, file)
	}
	res.err = err
	t.response <- res

}

// Todo: This following is incomplete
func populateFileInfo(file *File, attr *internal.ObjAttr) {
	file.size = attr.Size
}
