package debug

import (
	"fmt"
	"io"
	"sync"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// This package gives the debug facility to the dcache. Users can use top level sub-directory as "fs=debug" to know the
// state of the cluster/cache(maybe regarding clusterinfo, rpc calls, etc...). The files this package serves would be
// created on the fly and not stored anywhere in the filesystem.

var procFiles map[string]*procFile

// Mutex, openCnt is used for correctness of the filesystem if more that one handles for these procFiles are opened.
// without which also one can implement given that always there would one handle open for the file.
type procFile struct {
	mu            sync.Mutex            // lock for updating the openCnt and refreshing the buffer.
	buf           []byte                // Contents of the file.
	openCnt       int                   // Open handles for this file.
	refreshBuffer func(*procFile) error // Refresh the contents of the file.
}

// Directory entries in "fs=debug" directory. This list don't change as the files we support were already known.
var procDirList []*internal.ObjAttr

func init() {
	// Register the callbacks for the procFiles.
	procFiles = map[string]*procFile{
		"clusterMap.json": &procFile{
			buf:           make([]byte, 0, 4096),
			openCnt:       0,
			refreshBuffer: readClusterMapCallback,
		}, // Show clusterInfo about dcache.
	}
	procDirList = make([]*internal.ObjAttr, 0, len(procFiles))
	for path, _ := range procFiles {
		attr := &internal.ObjAttr{
			Name: path,
			Path: path,
			Size: 0,
		}
		procDirList = append(procDirList, attr)
	}
}

// Return the size of the file as zero, as we don't know the size at this point.
func GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if _, ok := procFiles[options.Name]; ok {
		attr := internal.ObjAttr{
			Name: options.Name,
			Path: options.Name,
			Size: 0,
		}
		return &attr, nil
	}
	return nil, syscall.ENOENT
}

func StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	return procDirList, "", nil
}

// Read the file at the time of openFile into the corresponding buffer.
func OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	if options.Flags&syscall.O_RDWR != 0 || options.Flags&syscall.O_WRONLY != 0 {
		return nil, syscall.EACCES
	}

	handle := handlemap.NewHandle(options.Name)
	handle.SetFsDebug()

	pFile, err := openProcFile(options.Name)
	if err != nil {
		return nil, syscall.ENOENT
	}
	handle.IFObj = pFile
	return handle, nil
}

// Read the buffer inside the procFile.
// No need to acquire the lock before reading from the buffer. As the buffer for proc file  would only refreshed only
// once at the start of the openFile even there are multiple handles.
func ReadFile(options internal.ReadInBufferOptions) (int, error) {
	common.Assert(options.Handle.IFObj != nil)
	pFile := options.Handle.IFObj.(*procFile)
	if options.Offset >= int64(len(pFile.buf)) {
		return 0, io.EOF
	}
	bytesRead := copy(options.Data, pFile.buf[options.Offset:])
	return bytesRead, nil
}

func CloseFile(options internal.CloseFileOptions) error {
	common.Assert(options.Handle.IFObj != nil)
	pFile := options.Handle.IFObj.(*procFile)
	closeProcFile(pFile)
	return nil
}

// Refresh the contents of the proc File if needed
func openProcFile(path string) (*procFile, error) {
	if pFile, ok := procFiles[path]; ok {
		pFile.mu.Lock()
		defer pFile.mu.Unlock()
		common.Assert(pFile.openCnt >= 0, fmt.Sprintf("Open Cnt for procFile: %s, openCnt: %d", path, pFile.openCnt))
		if pFile.openCnt == 0 {
			// This is the first handle to the proc File refresh the contents of the procFile.
			// Reset the buffer to length 0
			pFile.buf = pFile.buf[:0]
			err := pFile.refreshBuffer(pFile)
			if err != nil {
				return nil, err
			}
		}
		pFile.openCnt++
		return pFile, nil
	}
	return nil, syscall.ENOENT
}

func closeProcFile(pFile *procFile) {
	pFile.mu.Lock()
	defer pFile.mu.Unlock()
	pFile.openCnt--
}
