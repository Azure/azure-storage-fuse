package block_cache_new

import (
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// Note: There is a reason why we are storing the references to open handles inside a file rather
// maintaing a counter, because to support deferring the removal of files when some open handles are present.
// At that time we dont want to iterate over entire open handle map to change some fields
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
	changed      bool                       // is there any write/truncate operation happened?
	flushOngoing chan struct{}              // This channel blocks the operations on the blocks when there is a flush operation going on.
}

func (f *File) getOpenFDcount() int {
	return len(f.handles)
}

func (f *File) getFileSize() int64 {
	f.Lock()
	defer f.Unlock()
	return f.size
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
		changed:      false,
		flushOngoing: make(chan struct{}),
	}
	close(f.flushOngoing)

	return f
}

// Sync Map filepath->*File
var fileMap sync.Map

func createFreshHandleForFile(name string, size int64, mtime time.Time, flags int) *handlemap.Handle {
	handle := handlemap.NewHandle(name)
	handle.Mtime = mtime
	handle.Size = size
	if flags&os.O_RDONLY != 0 {
		handle.Flags.Set(handlemap.HandleFlagOpenRDONLY)
	} else if flags&os.O_WRONLY != 0 {
		handle.Flags.Set(handlemap.HandleFlagOpenWRONLY)
	} else if flags&os.O_RDWR != 0 {
		handle.Flags.Set(handlemap.HandleFlagOpenRDWR)
	} else {
		log.Info("BlockCache::createFreshHandleForFile : Unknown Open flags %X, file : %s", handle.ID, name)
		//todo: Do this correctly
		handle.Flags.Set(handlemap.HandleFlagOpenRDONLY)
	}
	return handle
}

func getFileFromPath(key string) (*File, bool) {
	f := CreateFile(key)
	var first_open bool = false
	file, loaded := fileMap.LoadOrStore(key, f)
	if !loaded {
		first_open = true
	}
	return file.(*File), first_open
}

// Remove the handle from the file
// Release the buffers if the openFDcount is zero for the file
func deleteOpenHandleForFile(handle *handlemap.Handle) {
	file, _ := getFileFromPath(handle.Path)
	file.Lock()
	delete(file.handles, handle)
	if file.getOpenFDcount() == 0 {
		releaseBuffersOfFile(file)
		fileMap.Delete(file.Name) // Todo: what happens open call comes before release async call finish
	}
	file.Unlock()
	deleteHandleFromHandleMap(handle)
}

func checkFileExistsInOpen(key string) (*File, bool) {
	f, ok := fileMap.Load(key)
	if ok {
		return f.(*File), ok
	}
	return nil, ok
}

func deleteFile(f *File) {
	if f.getOpenFDcount() == 0 {
		fileMap.Delete(f.Name)
	}
}

func hardDeleteFile(path string) {
	fileMap.Delete(path)
}

// Global Map for all the open fds/handles across blobfuse, *handle->*File
var handleMap sync.Map

func putHandleIntoMap(h *handlemap.Handle, f *File) {
	handleMap.Store(h, f)
}

func getFileFromHandle(h *handlemap.Handle) *File {
	f, ok := handleMap.Load(h)
	if !ok {
		panic("handle was not found inside the handlemap")
	}
	return f.(*File)
}

func deleteHandleFromHandleMap(h *handlemap.Handle) {
	handleMap.Delete(h)
}
