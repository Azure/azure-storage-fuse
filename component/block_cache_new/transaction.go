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
		serve_close(file, t)
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
	copied := 0
	len_of_copy := len(req.data)
	for copied < len_of_copy {
		idx := getBlockIndex(offset)
		block, err := getBlockForRead(idx, req.h, file)
		if err != nil {
			res.err = err
			break
		}
		blockOffset := getBlockOffset(offset)
		bytesCopied := copy(req.data[copied:], block[blockOffset:])
		copied += bytesCopied
		offset += int64(bytesCopied)
		if offset >= file.size { //this should be protected by lock ig, idk
			res.err = io.EOF
			break
		}
	}
	res.bytesRead = copied
	t.response <- res
}

func serve_write(file *File, t *Transaction) {

}

func serve_close(file *File, t *Transaction) {

}

func populateFileInfo(file *File, attr *internal.ObjAttr) {
	file.size = attr.Size
}
