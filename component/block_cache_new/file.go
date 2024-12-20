package block_cache_new

import (
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type File struct {
	sync.RWMutex
	handles     map[*handlemap.Handle]bool // Open file handles for this file
	blockList   blockList                  // Blocklist
	Etag        string                     // Etag of the file
	Name        string                     // File Name
	size        int64                      // File Size
	synced      bool                       // Is file synced with Azure storage
	holePunched bool                       // Represents if we have punched any hole while uploading the data.
}

func CreateFile(fileName string) *File {
	f := &File{
		Name:      fileName,
		handles:   make(map[*handlemap.Handle]bool),
		blockList: make([]*block, 0),
		size:      -1,
		synced:    true,
	}

	return f
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
	}
	return file.(*File), first_open
}

// Remove the handle from the file
// Release the buffers
func DeleteHandleForFile(handle *handlemap.Handle) {
	file, _ := GetFile(handle.Path)
	file.Lock()
	delete(file.handles, handle)
	if len(file.handles) == 0 {
		releaseBuffers(file)
		fileMap.Delete(file.Name) // Todo: what happens open call comes before release async call finish
	}
	file.Unlock()
}
