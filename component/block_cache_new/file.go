package block_cache_new

import (
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

type File struct {
	sync.RWMutex
	handles      map[*handlemap.Handle]bool // Open file handles for this file
	blockList    blockList                  //  These blocks inside blocklist is used for files which can both read and write.
	Etag         string                     // Etag of the file
	Name         string                     // File Name
	size         int64                      // File Size
	synced       bool                       // Is file synced with Azure storage
	holePunched  bool                       // Represents if we have punched any hole while uploading the data.
	blkListState blocklistState             // all blocklists which are not compatible with block cache can only be read
}

type blocklistState int

const (
	blockListInvalid blocklistState = iota
	blockListValid
	blockListNotRetrieved
)

func CreateFile(fileName string) *File {
	f := &File{
		Name:         fileName,
		handles:      make(map[*handlemap.Handle]bool),
		blockList:    make([]*block, 0),
		size:         -1,
		synced:       true,
		blkListState: blockListNotRetrieved,
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

func GetFileFromPath(key string) (*File, bool) {
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
	file, _ := GetFileFromPath(handle.Path)
	file.Lock()
	delete(file.handles, handle)
	if len(file.handles) == 0 {
		releaseBuffers(file)
		fileMap.Delete(file.Name) // Todo: what happens open call comes before release async call finish
	}
	file.Unlock()
}

func checkFileExistsInOpen(key string) (*File, bool) {
	f, ok := fileMap.Load(key)
	if ok {
		return f.(*File), ok
	}
	return nil, ok
}

func DeleteFile(f *File) {
	if len(f.handles) == 0 {
		fileMap.Delete(f.Name)
	}
}

func HardDeleteFile(path string) {
	fileMap.Delete(path)
}

// Sync map for handles, *handle->*File
var handleMap sync.Map

func PutHandleIntoMap(h *handlemap.Handle, f *File) {
	handleMap.Store(h, f)
}

func GetFileFromHandle(h *handlemap.Handle) *File {
	f, ok := handleMap.Load(h)
	if !ok {
		panic("handle was not found inside the handlemap")
	}
	return f.(*File)
}

func DeleteHandleFromMap(h *handlemap.Handle) {
	handleMap.Delete(h)
}
