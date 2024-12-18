package block_cache_new

import (
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type File struct {
	sync.RWMutex
	handles      map[*handlemap.Handle]bool // Open file handles for this file
	blockList    blockList                  // Blocklist
	Etag         string                     // Etag of the file
	Name         string                     // File Name
	size         int64                      // File Size
	transactions chan *Transaction          // Channel which contains all the outstanding requests
}

func CreateFile(fileName string) *File {
	f := &File{
		Name:         fileName,
		transactions: make(chan *Transaction, 1),
		handles:      make(map[*handlemap.Handle]bool),
		size:         -1,
	}

	return f
}

func startHandlingRequests(file *File) {
	for {
		select {
		case t := <-file.transactions:
			HandleTransaction(file, t)
		}
	}
}

// Sync Map filepath->*File
var fileMap sync.Map

func CreateFreshHandleForFile(name string, size int64, mtime time.Time) *handlemap.Handle {
	handle := handlemap.NewHandle(name)
	handle.Mtime = mtime
	handle.Size = size
	return handle
}

func GetFile(key string) (*File, bool) {
	f := CreateFile(key)
	var first_open bool = false
	file, loaded := fileMap.LoadOrStore(key, f)
	if !loaded {
		first_open = true
		go startHandlingRequests(f)
	}
	return file.(*File), first_open
}

func DeleteHandleForFile(file *File, handle *handlemap.Handle) {
	file.Lock()
	delete(file.handles, handle)
	if len(file.handles) == 0 {
		//fileMap.Delete(file.Name)
	}
	file.Unlock()
}
