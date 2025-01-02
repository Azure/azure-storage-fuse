/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package file_cache

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	hmcommon "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/common"
	hminternal "github.com/Azure/azure-storage-fuse/v2/tools/health-monitor/internal"

	"github.com/radovskyb/watcher"
)

type FileCache struct {
	name      string
	pid       string
	tmpPath   string
	maxSizeMB float64
	cacheObj  CacheDir
}

type CacheDir struct {
	cacheSize      int64
	cacheConsumed  float64
	fileCreatedMap map[string]int64
	fileRemovedMap map[string]int64
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
	log.Debug("cache_monitor::Monitor : started")

	return fc.cacheWatcher()
}

func (fc *FileCache) ExportStats(timestamp string, st interface{}) {
	se, err := hminternal.NewStatsExporter()
	if err != nil || se == nil {
		log.Err("cache_monitor::ExportStats : Error in creating stats exporter instance [%v]", err)
		return
	}

	se.AddMonitorStats(fc.GetName(), timestamp, st)
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
	fc.cacheObj.fileCreatedMap = make(map[string]int64)
	fc.cacheObj.fileRemovedMap = make(map[string]int64)

	w := watcher.New()

	// ignore hidden files
	w.IgnoreHiddenFiles(true)

	go func() {
		for {
			select {

			case event := <-w.Event:
				err := hmcommon.CheckProcessStatus(hmcommon.Pid)
				if err != nil {
					log.Err("cache_monitor::cacheWatcher : [%v]", err)
					return
				}

				if strings.ToUpper(event.Op.String()) == create {
					fc.createEvent(&event)

				} else if strings.ToUpper(event.Op.String()) == remove {
					fc.removeEvent(&event)

				} else if strings.ToUpper(event.Op.String()) == chmod {
					fc.chmodEvent(&event)

				} else if strings.ToUpper(event.Op.String()) == write {
					fc.writeEvent(&event)

				} else if strings.ToUpper(event.Op.String()) == rename {
					fc.renameEvent(&event)

				} else if strings.ToUpper(event.Op.String()) == move {
					fc.moveEvent(&event)

				}

			case err := <-w.Error:
				log.Err("cache_monitor::cache_watcher : [%v]", err)
				return

			case <-w.Closed:
				return
			}
		}
	}()

	// watch file cache directory for changes
	if err := w.Add(fc.tmpPath); err != nil {
		log.Err("cache_monitor::cacheWatcher : [%v]", err)
		return err
	}

	// set recursive watcher on file cache directory
	if err := w.AddRecursive(fc.tmpPath); err != nil {
		log.Err("cache_monitor::cacheWatcher : [%v]", err)
		return err
	}

	// list of all of the files and folders currently being watched
	for path := range w.WatchedFiles() {
		log.Debug("cache_monitor::cacheWatcher : Watching %v", path)
	}

	// Start the watching process - it'll check for changes every 100ms
	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Err("cache_monitor::cacheWatcher : [%v]", err)
		return err
	}

	return nil
}

func (fc *FileCache) createEvent(event *watcher.Event) {
	if !event.IsDir() {
		delete(fc.cacheObj.fileRemovedMap, event.Path)
		fc.cacheObj.fileCreatedMap[event.Path] = event.Size()
		fc.cacheObj.cacheSize += event.Size()
		fc.cacheObj.cacheConsumed = (float64)(fc.cacheObj.cacheSize*100) / (fc.maxSizeMB * common.MbToBytes)
	}

	e := fc.getCacheEventObj(event)
	if !event.IsDir() {
		e.Value[fileSize] = strconv.FormatInt(event.Size(), 10)
	}

	fc.ExportStats(time.Now().Format(time.RFC3339), e)
}

func (fc *FileCache) removeEvent(event *watcher.Event) {
	if !event.IsDir() {
		delete(fc.cacheObj.fileCreatedMap, event.Path)
		fc.cacheObj.fileRemovedMap[event.Path] = event.Size()
		fc.cacheObj.cacheSize = int64(math.Max(0, float64(fc.cacheObj.cacheSize-event.Size())))
		fc.cacheObj.cacheConsumed = (float64)(fc.cacheObj.cacheSize*100) / (fc.maxSizeMB * common.MbToBytes)
	}

	e := fc.getCacheEventObj(event)
	if !event.IsDir() {
		e.Value[fileSize] = strconv.FormatInt(event.Size(), 10)
	}

	fc.ExportStats(time.Now().Format(time.RFC3339), e)
}

func (fc *FileCache) chmodEvent(event *watcher.Event) {
	if !event.IsDir() {
		delete(fc.cacheObj.fileRemovedMap, event.Path)
		fileSize := fc.cacheObj.fileCreatedMap[event.Path]

		if fileSize != event.Size() {
			fc.cacheObj.cacheSize += event.Size() - fileSize
			fc.cacheObj.fileCreatedMap[event.Path] = event.Size()
			fc.cacheObj.cacheConsumed = (float64)(fc.cacheObj.cacheSize*100) / (fc.maxSizeMB * common.MbToBytes)
		}
	}

	e := fc.getCacheEventObj(event)
	e.Value[mode] = event.Mode().String()

	fc.ExportStats(time.Now().Format(time.RFC3339), e)
}

func (fc *FileCache) writeEvent(event *watcher.Event) {
	if !event.IsDir() {
		delete(fc.cacheObj.fileRemovedMap, event.Path)
		fileSize := fc.cacheObj.fileCreatedMap[event.Path]

		if fileSize == event.Size() {
			return
		}

		fc.cacheObj.cacheSize += event.Size() - fileSize
		fc.cacheObj.fileCreatedMap[event.Path] = event.Size()
		fc.cacheObj.cacheConsumed = (float64)(fc.cacheObj.cacheSize*100) / (fc.maxSizeMB * common.MbToBytes)
	} else {
		return
	}

	// not logging write events since there are multiple events in case of uploads
	// e := fc.getCacheEventObj(event)
	// if !event.IsDir() {
	// 	e.Value[fileSize] = strconv.FormatInt(event.Size(), 10)
	// }

	// fc.ExportStats(time.Now().Format(time.RFC3339), e)
}

func (fc *FileCache) renameEvent(event *watcher.Event) {
	e := fc.getCacheEventObj(event)
	e.Value[oldPath] = event.OldPath

	fc.ExportStats(time.Now().Format(time.RFC3339), e)
}

func (fc *FileCache) moveEvent(event *watcher.Event) {
	e := fc.getCacheEventObj(event)
	e.Value[oldPath] = event.OldPath

	fc.ExportStats(time.Now().Format(time.RFC3339), e)
}

func (fc *FileCache) getCacheEventObj(event *watcher.Event) *hmcommon.CacheEvent {
	e := &hmcommon.CacheEvent{
		CacheEvent:      event.Op.String(),
		Path:            event.Path,
		IsDir:           event.IsDir(),
		CacheSize:       fc.cacheObj.cacheSize,
		CacheConsumed:   fmt.Sprintf("%.2f%%", fc.cacheObj.cacheConsumed),
		CacheFilesCnt:   (int64)(len(fc.cacheObj.fileCreatedMap)),
		EvictedFilesCnt: (int64)(len(fc.cacheObj.fileRemovedMap)),
		Value:           make(map[string]string),
	}

	return e
}

func NewFileCacheMonitor() hminternal.Monitor {
	fc := &FileCache{
		pid:       hmcommon.Pid,
		tmpPath:   hmcommon.TempCachePath,
		maxSizeMB: hmcommon.MaxCacheSize,
		cacheObj:  CacheDir{},
	}

	fc.SetName(hmcommon.FileCacheMon)

	return fc
}

func init() {
	hminternal.AddMonitor(hmcommon.FileCacheMon, NewFileCacheMonitor)
}
