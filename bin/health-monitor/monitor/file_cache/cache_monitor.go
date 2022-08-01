package file_cache

import (
	"fmt"
	"time"

	hmcommon "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/bin/health-monitor/internal"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/radovskyb/watcher"
)

type FileCache struct {
	name      string
	pid       string
	tmpPath   string
	maxSizeMB float64
}

func (fc *FileCache) GetName() string {
	return fc.name
}

func (fc *FileCache) SetName(name string) {
	fc.name = name
}

func (fc *FileCache) Monitor() error {
	err := fc.Validate()
	if err != nil {
		log.Err("cache_monitor::Monitor : [%v]", err)
		return err
	}

	w := watcher.New()

	// ignore hidden files
	w.IgnoreHiddenFiles(true)

	// watch file cache directory for changes
	if err := w.Add(fc.tmpPath); err != nil {
		log.Err("cache_monitor::Monitor : [%v]", err)
		return err
	}

	// set recuursive watcher on file cache directory
	if err := w.AddRecursive(fc.tmpPath); err != nil {
		log.Err("cache_monitor::Monitor : [%v]", err)
		return err
	}

	// list of all of the files and folders currently being watched
	for path, _ := range w.WatchedFiles() {
		log.Debug("Watching %v", path)
	}

	// Start the watching process - it'll check for changes every 100ms
	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Err("cache_monitor::Monitor : [%v]", err)
		return err
	}

	return fc.cacheWatcher()
}

func (fc *FileCache) ExportStats() {
	fmt.Println("Inside file cache export stats")
}

func (fc *FileCache) Validate() error {
	if len(fc.pid) == 0 {
		return fmt.Errorf("pid of blobfuse2 is not given")
	}

	if len(fc.tmpPath) == 0 {
		return fmt.Errorf("cache path is not given")
	}

	return nil
}

func (fc *FileCache) cacheWatcher() error {
	return nil
}

func NewFileCacheMonitor() hminternal.Monitor {
	fc := &FileCache{
		pid:       hmcommon.Pid,
		tmpPath:   hmcommon.TempCachePath,
		maxSizeMB: hmcommon.MaxCacheSize,
	}

	fc.SetName(hmcommon.File_cache)

	return fc
}

func init() {
	hminternal.AddMonitor(hmcommon.File_cache, NewFileCacheMonitor)
}
