package block_cache

// errorInjectingComponent wraps a real internal.Component (typically loopback FS) and allows
// injecting errors for specific operations to test error-handling code paths in BlockCache.

import (
	"context"
	"fmt"
	"sync"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

// errorInjectingComponent delegates all calls to an inner component but can return
// injected errors for specific operations. Thread-safe via mutex.
type errorInjectingComponent struct {
	inner  internal.Component
	mu     sync.RWMutex
	errors map[string]error // operation name -> error to return
	calls  map[string]int   // operation name -> call count
}

func newErrorInjectingComponent(inner internal.Component) *errorInjectingComponent {
	return &errorInjectingComponent{
		inner:  inner,
		errors: make(map[string]error),
		calls:  make(map[string]int),
	}
}

func (e *errorInjectingComponent) setError(op string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err == nil {
		delete(e.errors, op)
	} else {
		e.errors[op] = err
	}
}

func (e *errorInjectingComponent) clearErrors() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errors = make(map[string]error)
}

func (e *errorInjectingComponent) getCallCount(op string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.calls[op]
}

func (e *errorInjectingComponent) checkError(op string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls[op]++
	if err, ok := e.errors[op]; ok {
		return err
	}
	return nil
}

// Pipeline methods — delegate to inner
func (e *errorInjectingComponent) Name() string                          { return e.inner.Name() }
func (e *errorInjectingComponent) SetName(n string)                      { e.inner.SetName(n) }
func (e *errorInjectingComponent) Configure(b bool) error                { return e.inner.Configure(b) }
func (e *errorInjectingComponent) GenConfig() string                     { return e.inner.GenConfig() }
func (e *errorInjectingComponent) Priority() internal.ComponentPriority  { return e.inner.Priority() }
func (e *errorInjectingComponent) SetNextComponent(c internal.Component) { e.inner.SetNextComponent(c) }
func (e *errorInjectingComponent) NextComponent() internal.Component     { return e.inner.NextComponent() }
func (e *errorInjectingComponent) Start(ctx context.Context) error {
	return nil
}
func (e *errorInjectingComponent) Stop() error { return e.inner.Stop() }

// Directory operations
func (e *errorInjectingComponent) CreateDir(o internal.CreateDirOptions) error {
	return e.inner.CreateDir(o)
}
func (e *errorInjectingComponent) DeleteDir(o internal.DeleteDirOptions) error {
	if err := e.checkError("DeleteDir"); err != nil {
		return err
	}
	return e.inner.DeleteDir(o)
}
func (e *errorInjectingComponent) IsDirEmpty(o internal.IsDirEmptyOptions) bool {
	return e.inner.IsDirEmpty(o)
}
func (e *errorInjectingComponent) OpenDir(o internal.OpenDirOptions) error {
	return e.inner.OpenDir(o)
}
func (e *errorInjectingComponent) ReadDir(o internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	return e.inner.ReadDir(o)
}
func (e *errorInjectingComponent) StreamDir(o internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	return e.inner.StreamDir(o)
}
func (e *errorInjectingComponent) CloseDir(o internal.CloseDirOptions) error {
	return e.inner.CloseDir(o)
}
func (e *errorInjectingComponent) RenameDir(o internal.RenameDirOptions) error {
	if err := e.checkError("RenameDir"); err != nil {
		return err
	}
	return e.inner.RenameDir(o)
}

// File operations
func (e *errorInjectingComponent) CreateFile(o internal.CreateFileOptions) (*handlemap.Handle, error) {
	if err := e.checkError("CreateFile"); err != nil {
		return nil, err
	}
	return e.inner.CreateFile(o)
}
func (e *errorInjectingComponent) DeleteFile(o internal.DeleteFileOptions) error {
	if err := e.checkError("DeleteFile"); err != nil {
		return err
	}
	return e.inner.DeleteFile(o)
}
func (e *errorInjectingComponent) OpenFile(o internal.OpenFileOptions) (*handlemap.Handle, error) {
	return e.inner.OpenFile(o)
}
func (e *errorInjectingComponent) ReadFile(o internal.ReadFileOptions) ([]byte, error) {
	return e.inner.ReadFile(o)
}
func (e *errorInjectingComponent) ReadInBuffer(o *internal.ReadInBufferOptions) (int, error) {
	if err := e.checkError("ReadInBuffer"); err != nil {
		return 0, err
	}
	return e.inner.ReadInBuffer(o)
}
func (e *errorInjectingComponent) WriteFile(o *internal.WriteFileOptions) (int, error) {
	return e.inner.WriteFile(o)
}
func (e *errorInjectingComponent) SyncFile(o internal.SyncFileOptions) error {
	return e.inner.SyncFile(o)
}
func (e *errorInjectingComponent) FlushFile(o internal.FlushFileOptions) error {
	return e.inner.FlushFile(o)
}
func (e *errorInjectingComponent) ReleaseFile(o internal.ReleaseFileOptions) error {
	return e.inner.ReleaseFile(o)
}
func (e *errorInjectingComponent) RenameFile(o internal.RenameFileOptions) error {
	if err := e.checkError("RenameFile"); err != nil {
		return err
	}
	return e.inner.RenameFile(o)
}
func (e *errorInjectingComponent) CopyToFile(o internal.CopyToFileOptions) error {
	return e.inner.CopyToFile(o)
}
func (e *errorInjectingComponent) CopyFromFile(o internal.CopyFromFileOptions) error {
	return e.inner.CopyFromFile(o)
}
func (e *errorInjectingComponent) SyncDir(o internal.SyncDirOptions) error {
	return e.inner.SyncDir(o)
}
func (e *errorInjectingComponent) UnlinkFile(o internal.UnlinkFileOptions) error {
	return e.inner.UnlinkFile(o)
}

// Symlink operations
func (e *errorInjectingComponent) CreateLink(o internal.CreateLinkOptions) error {
	return e.inner.CreateLink(o)
}
func (e *errorInjectingComponent) ReadLink(o internal.ReadLinkOptions) (string, error) {
	return e.inner.ReadLink(o)
}

// Filesystem operations
func (e *errorInjectingComponent) GetAttr(o internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if err := e.checkError("GetAttr"); err != nil {
		return nil, err
	}
	return e.inner.GetAttr(o)
}
func (e *errorInjectingComponent) Chmod(o internal.ChmodOptions) error { return e.inner.Chmod(o) }
func (e *errorInjectingComponent) Chown(o internal.ChownOptions) error { return e.inner.Chown(o) }
func (e *errorInjectingComponent) TruncateFile(o internal.TruncateFileOptions) error {
	return e.inner.TruncateFile(o)
}
func (e *errorInjectingComponent) GetFileBlockOffsets(o internal.GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	return e.inner.GetFileBlockOffsets(o)
}
func (e *errorInjectingComponent) FileUsed(name string) error { return e.inner.FileUsed(name) }
func (e *errorInjectingComponent) StatFs() (*syscall.Statfs_t, bool, error) {
	if err := e.checkError("StatFs"); err != nil {
		return nil, false, err
	}
	return e.inner.StatFs()
}

// Block/blob operations
func (e *errorInjectingComponent) GetCommittedBlockList(name string) (*internal.CommittedBlockList, error) {
	if err := e.checkError("GetCommittedBlockList"); err != nil {
		return nil, err
	}
	return e.inner.GetCommittedBlockList(name)
}
func (e *errorInjectingComponent) StageData(o internal.StageDataOptions) error {
	if err := e.checkError("StageData"); err != nil {
		return err
	}
	return e.inner.StageData(o)
}
func (e *errorInjectingComponent) CommitData(o internal.CommitDataOptions) error {
	if err := e.checkError("CommitData"); err != nil {
		return err
	}
	return e.inner.CommitData(o)
}

// Verify interface compliance
var _ internal.Component = &errorInjectingComponent{}

// injectedError creates a standard test error.
func injectedError(op string) error {
	return fmt.Errorf("injected %s error", op)
}
