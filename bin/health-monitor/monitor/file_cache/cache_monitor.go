package file_cache

import (
	"fmt"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
)

type FileCache struct {
	name string
}

func (fc *FileCache) GetName() string {
	return fc.name
}

func (fc *FileCache) SetName(name string) {
	fc.name = name
}

func (fc *FileCache) Monitor() error {
	fmt.Println("Inside file cache monitor")
	return nil
}

func (fc *FileCache) ExportStats() {
	fmt.Println("Inside file cache export stats")
}

func NewFileCacheMonitor() hminternal.Monitor {
	fc := &FileCache{}
	fc.SetName(hmcommon.File_cache)

	return fc
}

func init() {
	fmt.Println("Inside file cache monitor")
	hminternal.AddMonitor(hmcommon.File_cache, NewFileCacheMonitor)
}
