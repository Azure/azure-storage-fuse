/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package xload

import (
	"bufio"
	"container/list"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/fsnotify/fsnotify"
	"github.com/vibhansa-msft/tlru"
)

// Common structure for Component
type Xload struct {
	internal.BaseComponent
	blockSize          uint64              // Size of each block to be cached
	mode               Mode                // Mode of the Xload component
	exportProgress     bool                // Export the progress of xload operation to json file
	validateMD5        bool                // validate md5sum on download, if md5sum is set on blob
	workerCount        uint32              // Number of workers running
	blockPool          *BlockPool          // Pool of blocks
	path               string              // Path on local disk where Xload will operate
	defaultPermission  os.FileMode         // Default permissions of files and directories in the xload path
	comps              []XComponent        // list of components in xload
	hintFile           string              // external prefetch hint file path
	hintSeen           map[string]struct{} // already scheduled hint entries
	hintPollInterval   time.Duration       // poll interval for hint file
	hintPollConfigured bool                // whether user explicitly set poll interval
	hintMu             sync.Mutex          // guard hintSeen
	maxCacheSizeMB     float64             // optional cap for xload cache (MB), 0 = disabled
	evictPolicy        *tlru.TLRU          // eviction policy shared with file_cache limits
	cacheNodes         sync.Map            // track tlru nodes per path
	statsMgr           *StatsManager       // stats manager
	fileLocks          *common.LockMap     // lock to take on a file if one thread is processing it
	poolSize           uint32              // Number of blocks in the pool
	poolctx            context.Context     // context for the thread pool
	poolCancelFunc     context.CancelFunc  // cancel function for the thread pool
}

// Structure defining your config parameters
type XloadOptions struct {
	BlockSize      float64 `config:"block-size-mb" yaml:"block-size-mb,omitempty"`
	Mode           string  `config:"mode" yaml:"mode,omitempty"`
	PrefetchHint   string  `config:"prefetch-hint-file" yaml:"prefetch-hint-file,omitempty"`
	HintPollSec    uint32  `config:"prefetch-hint-poll-sec" yaml:"prefetch-hint-poll-sec,omitempty"`
	Path           string  `config:"path" yaml:"path,omitempty"`
	ExportProgress bool    `config:"export-progress" yaml:"path,omitempty"`
	ValidateMD5    bool    `config:"validate-md5" yaml:"validate-md5,omitempty"`
	CleanupOnStart bool    `config:"cleanup-on-start" yaml:"cleanup-on-start,omitempty"`
	Workers        int32   `config:"workers" yaml:"workers,omitempty"`
	PoolSize       uint32  `config:"pool-size" yaml:"pool-size,omitempty"`
	// TODO:: xload : add parallelism parameter
}

const (
	compName         = "xload"
	defaultBlockSize = 16
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &Xload{}

func (xl *Xload) Name() string {
	return compName
}

func (xl *Xload) SetName(name string) {
	xl.BaseComponent.SetName(name)
}

func (xl *Xload) SetNextComponent(nc internal.Component) {
	xl.BaseComponent.SetNextComponent(nc)
}

func (xl *Xload) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.LevelMid()
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
func (xl *Xload) Configure(_ bool) error {
	log.Trace("Xload::Configure : %s", xl.Name())

	// xload component should be used only in readonly mode
	var readonly bool
	err := config.UnmarshalKey("read-only", &readonly)
	if err != nil {
		log.Err("Xload::Configure : config error [unable to obtain read-only]")
		return fmt.Errorf("config error in %s [%s]", xl.Name(), err.Error())
	}

	if !readonly {
		log.Err("Xload::Configure : Xload component should be used only in read-only mode")
		return fmt.Errorf("Xload component should be used in only in read-only mode")
	}

	conf := XloadOptions{}
	err = config.UnmarshalKey(xl.Name(), &conf)
	if err != nil {
		log.Err("Xload::Configure : config error [invalid config attributes]")
		return fmt.Errorf("Xload: config error [invalid config attributes]")
	}

	blockSize := (float64)(defaultBlockSize) // 16 MB as default block size
	if config.IsSet(compName + ".block-size-mb") {
		blockSize = conf.BlockSize
	} else if config.IsSet("stream.block-size-mb") {
		err = config.UnmarshalKey("stream.block-size-mb", &blockSize)
		if err != nil {
			log.Err("Xload::Configure : Failed to unmarshal block-size-mb [%s]", err.Error())
		}
	}

	localPath := strings.TrimSpace(conf.Path)
	if localPath == "" {
		if config.IsSet("file_cache.path") {
			err = config.UnmarshalKey("file_cache.path", &localPath)
			if err != nil {
				log.Err("Xload::Configure : Failed to unmarshal tmp-path [%s]", err.Error())
			}
		}
	}

	xl.path = common.ExpandPath(localPath)
	if xl.path == "" {
		// TODO:: xload : should we use current working directory in this case
		log.Err("Xload::Configure : config error [path not given in xload]")
		return fmt.Errorf("config error in %s [path not given]", xl.Name())
	} else {
		//check mnt path is not same as xload path
		mntPath := ""
		err = config.UnmarshalKey("mount-path", &mntPath)
		if err != nil {
			log.Err("Xload::Configure : config error [unable to obtain Mount Path [%s]]", err.Error())
			return fmt.Errorf("config error in %s [%s]", xl.Name(), err.Error())
		}

		if xl.path == mntPath {
			log.Err("Xload::Configure : config error [xload path is same as mount path]")
			return fmt.Errorf("config error in %s error [xload path is same as mount path]", xl.Name())
		}

		_, err = os.Stat(xl.path)
		if os.IsNotExist(err) {
			log.Info("Xload::Configure : config error [xload path does not exist, attempting to create path]")
			err := os.Mkdir(xl.path, os.FileMode(0755))
			if err != nil {
				log.Err("Xload::Configure : config error creating directory of xload path [%s]", err.Error())
				return fmt.Errorf("config error in %s [%s]", xl.Name(), err.Error())
			}
		}

		if !common.IsDirectoryEmpty(xl.path) {
			log.Err("Xload::Configure : config error %s directory is not empty", xl.path)
			return fmt.Errorf("config error in %s [temp directory not empty]", xl.Name())
		}
	}

	var mode = EMode.PRELOAD() // using preload as the default mode
	if len(conf.Mode) > 0 {
		err = mode.Parse(conf.Mode)
		if err != nil {
			log.Err("Xload::Configure : Failed to parse mode %s [%s]", conf.Mode, err.Error())
			return fmt.Errorf("invalid mode in xload : %s", conf.Mode)
		}

		if mode == EMode.INVALID_MODE() {
			log.Err("Xload::Configure : Invalid mode : %s", conf.Mode)
			return fmt.Errorf("invalid mode in xload : %s", conf.Mode)
		}
	}

	xl.mode = mode
	xl.exportProgress = conf.ExportProgress
	xl.validateMD5 = conf.ValidateMD5

	// Prefetch hint support: if a hint file is provided, xload will only download paths listed in that file and skip full-container listing.
	xl.hintFile = strings.TrimSpace(conf.PrefetchHint)
	hintPoll := uint32(2)
	if config.IsSet(compName + ".prefetch-hint-poll-sec") {
		xl.hintPollConfigured = true
	}
	if conf.HintPollSec > 0 {
		hintPoll = conf.HintPollSec
	}
	xl.hintPollInterval = time.Duration(hintPoll) * time.Second
	if xl.hintFile != "" {
		xl.hintSeen = make(map[string]struct{})
	}

	// Reuse file_cache max-size-mb to cap xload's cache.
	if config.IsSet("file_cache.max-size-mb") {
		var fcConf struct {
			MaxSizeMB float64 `config:"max-size-mb"`
		}
		if err := config.UnmarshalKey("file_cache", &fcConf); err != nil {
			log.Warn("Xload::Configure : failed to read file_cache.max-size-mb [%s]", err.Error())
		} else if fcConf.MaxSizeMB > 0 {
			xl.maxCacheSizeMB = fcConf.MaxSizeMB
		}
	}

	allowOther := false
	err = config.UnmarshalKey("allow-other", &allowOther)
	if err != nil {
		log.Err("Xload::Configure : config error [unable to obtain allow-other]")
	}

	if allowOther {
		xl.defaultPermission = common.DefaultAllowOtherPermissionBits
	} else {
		xl.defaultPermission = common.DefaultFilePermissionBits
	}

	xl.workerCount = uint32(math.Min(float64(runtime.NumCPU()*3), float64(MAX_WORKER_COUNT)))
	if config.IsSet(compName+".workers") && conf.Workers > 0 {
		xl.workerCount = uint32(math.Min(float64(conf.Workers), float64(MAX_WORKER_COUNT)))
	}

	xl.blockSize = uint64(blockSize * float64(MB))
	xl.poolSize = xl.workerCount * 3
	if config.IsSet(compName + ".pool-size") {
		xl.poolSize = conf.PoolSize
	}

	xl.poolctx, xl.poolCancelFunc = context.WithCancel(context.Background())

	if xl.maxCacheSizeMB > 0 {
		// ttl must be >0 for tlru; we rely on cacheNeedsEviction for size-based cleanup, so use a long TTL.
		const evictTTLSeconds = 3600

		// Channel-backed tlru buffers on MaxNodes, so bound it based on the estimated number of cached files.
		maxCacheBytes := xl.maxCacheSizeMB * float64(MB)
		estimatedNodes := uint32(1)
		if xl.blockSize > 0 {
			estimatedNodes = uint32(math.Ceil(maxCacheBytes / float64(xl.blockSize)))
			if estimatedNodes == 0 {
				estimatedNodes = 1
			}
		}

		// Cap the buffer to keep memory reasonable even for large cache sizes.
		const maxEvictBuffer = 100000
		if estimatedNodes > maxEvictBuffer {
			estimatedNodes = maxEvictBuffer
		}

		xl.evictPolicy, err = tlru.New(estimatedNodes, evictTTLSeconds, xl.cacheEvict, 60, xl.cacheNeedsEviction)
		if err != nil {
			log.Err("Xload::Configure : failed to create eviction policy [%s]", err.Error())
			return fmt.Errorf("config error in %s [%s]", xl.Name(), err.Error())
		}
	}

	hintFileLog := "disabled"
	hintPollLog := "-"
	maxCacheLog := "unbounded"
	if xl.hintFile != "" {
		hintFileLog = xl.hintFile
		hintPollLog = xl.hintPollInterval.String()
	}
	if xl.maxCacheSizeMB > 0 {
		maxCacheLog = fmt.Sprintf("%.2fMB", xl.maxCacheSizeMB)
	}

	log.Crit("Xload::Configure : block size %v, mode %v, path %v, default permission %v, export progress %v, validate md5 %v, prefetch hint %v, hint poll %v, max cache %v",
		xl.blockSize, xl.mode.String(), xl.path, xl.defaultPermission, xl.exportProgress, xl.validateMD5, hintFileLog, hintPollLog, maxCacheLog)

	return nil
}

// Start : Pipeline calls this method to start the component functionality
func (xl *Xload) Start(ctx context.Context) error {
	log.Trace("Xload::Start : Starting component %s", xl.Name())

	xl.blockPool = NewBlockPool(xl.blockSize, xl.poolSize, xl.poolctx)
	if xl.blockPool == nil {
		log.Err("Xload::Start : Failed to create block pool")
		return fmt.Errorf("failed to create block pool")
	}

	var err error

	// create stats manager
	xl.statsMgr, err = NewStatsManager(xl.workerCount*2, xl.exportProgress, xl.blockPool)
	if err != nil {
		log.Err("Xload::Start : Failed to create stats manager [%s]", err.Error())
		return err
	}

	if xl.evictPolicy != nil {
		if err := xl.evictPolicy.Start(); err != nil {
			log.Err("Xload::Start : failed to start eviction policy [%s]", err.Error())
			return err
		}
	}

	// Xload : start code goes here
	switch xl.mode {
	case EMode.PRELOAD():
		// Start downloader here
		err = xl.createDownloader()
		if err != nil {
			log.Err("Xload::Start : Failed to start downloader [%s]", err.Error())
			return err
		}
	case EMode.UPLOAD():
		// Start uploader here
		return fmt.Errorf("uploader is currently unsupported")
	case EMode.SYNC():
		//Start syncer here
		return fmt.Errorf("sync is currently unsupported")
	default:
		log.Err("Xload::Start : Invalid mode : %s", xl.mode.String())
		return fmt.Errorf("invalid mode in xload : %s", xl.mode.String())
	}

	xl.statsMgr.Start()
	return xl.startComponents()
}

// Stop : Stop the component functionality and kill all threads started
func (xl *Xload) Stop() error {
	log.Trace("Xload::Stop : Stopping component %s", xl.Name())

	terminateTimeout := time.After(2 * time.Second)
	stopCh := make(chan int, 1)

	xl.poolCancelFunc()

	go func() {
		for i := 0; i < len(xl.comps); i++ {
			xl.comps[i].Stop()
		}

		xl.statsMgr.Stop()
		xl.blockPool.Terminate()
		stopCh <- 1
	}()

	select {
	case <-stopCh:
		log.Debug("Xload::Stop : Stop successful")
	case <-terminateTimeout:
		log.Warn("Xload::Stop : Stop timeout")
	}

	if xl.evictPolicy != nil {
		_ = xl.evictPolicy.Stop()
	}

	// TODO:: xload : should we delete the files from local path
	err := common.TempCacheCleanup(xl.path)
	if err != nil {
		log.Err("unable to clean xload local path [%s]", err.Error())
		return err
	}
	return nil
}

func (xl *Xload) createDownloader() error {
	log.Trace("Xload::createDownloader : Starting downloader")

	comps := []XComponent{}

	// If no hint file is provided, run the normal full-container listing.
	if xl.hintFile == "" {
		rl, err := newRemoteLister(&remoteListerOptions{
			path:              xl.path,
			workerCount:       uint32(math.Min(float64(runtime.NumCPU()/2), float64(MAX_LISTER))),
			defaultPermission: xl.defaultPermission,
			remote:            xl.NextComponent(),
			statsMgr:          xl.statsMgr,
		})
		if err != nil {
			log.Err("Xload::createDownloader : Unable to create remote lister [%s]", err.Error())
			return err
		}
		comps = append(comps, rl)
	}

	ds, err := newDownloadSplitter(&downloadSplitterOptions{
		blockPool:   xl.blockPool,
		path:        xl.path,
		workerCount: uint32(math.Min(float64(runtime.NumCPU()), float64(MAX_DATA_SPLITTER))),
		remote:      xl.NextComponent(),
		statsMgr:    xl.statsMgr,
		fileLocks:   xl.fileLocks,
		validateMD5: xl.validateMD5,
	})
	if err != nil {
		log.Err("Xload::createDownloader : Unable to create download splitter [%s]", err.Error())
		return err
	}

	rdm, err := newRemoteDataManager(&remoteDataManagerOptions{
		workerCount: xl.workerCount,
		remote:      xl.NextComponent(),
		statsMgr:    xl.statsMgr,
	})
	if err != nil {
		log.Err("Xload::startUploader : failed to create remote data manager [%s]", err.Error())
		return err
	}

	comps = append(comps, ds, rdm)
	xl.comps = comps
	return nil
}

func (xl *Xload) createChain() error {
	if len(xl.comps) == 0 {
		log.Err("Xload::createChain : no component initialized in xload")
		return fmt.Errorf("no component initialized in xload")
	}

	currComp := xl.comps[0]

	for i := 1; i < len(xl.comps); i++ {
		nextComp := xl.comps[i]
		currComp.SetNext(nextComp)
		currComp = nextComp
	}

	return nil
}

func (xl *Xload) startComponents() error {
	err := xl.createChain()
	if err != nil {
		log.Err("Xload::startComponents : failed to create chain [%s]", err.Error())
		return err
	}

	for i := len(xl.comps) - 1; i >= 0; i-- {
		xl.comps[i].Start(xl.poolctx)
	}

	if xl.hintFile != "" {
		xl.startHintWatcher()
	}

	return nil
}

// startHintWatcher monitors an external hint file and schedules downloads for newly added paths.
// If the user configured a poll interval, we use polling; otherwise, we prefer fsnotify watch.
func (xl *Xload) startHintWatcher() {
	if xl.hintPollConfigured && xl.hintPollInterval > 0 {
		log.Info("Xload::startHintWatcher : polling hint file %s every %s", xl.hintFile, xl.hintPollInterval)
		xl.startHintPoller()
		return
	}

	if err := xl.startHintFileWatch(); err != nil {
		log.Warn("Xload::startHintWatcher : watch failed for %s, falling back to polling [%s]", xl.hintFile, err.Error())
		xl.startHintPoller()
	}
}

// startHintPoller uses time-based polling to read the hint file.
func (xl *Xload) startHintPoller() {
	go func() {
		ticker := time.NewTicker(xl.hintPollInterval)
		defer ticker.Stop()

		xl.processHintFile()

		for {
			select {
			case <-xl.poolctx.Done():
				return
			case <-ticker.C:
				xl.processHintFile()
			}
		}
	}()
}

// startHintFileWatch uses fsnotify to react to changes without busy polling.
func (xl *Xload) startHintFileWatch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	hintDir := filepath.Dir(xl.hintFile)
	if err := watcher.Add(hintDir); err != nil {
		_ = watcher.Close()
		return err
	}

	log.Info("Xload::startHintWatcher : watching hint file %s for changes", xl.hintFile)

	go func() {
		defer watcher.Close()

		xl.processHintFile()

		for {
			select {
			case <-xl.poolctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Name == xl.hintFile && (event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Chmod) != 0) {
					xl.processHintFile()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Warn("Xload::startHintWatcher : fsnotify error [%s]", err.Error())
			}
		}
	}()

	return nil
}

// processHintFile reads the hint file and triggers downloads for unseen paths.
func (xl *Xload) processHintFile() {
	if xl.hintFile == "" {
		return
	}

	f, err := os.Open(xl.hintFile)
	if err != nil {
		log.Debug("Xload::processHintFile : unable to open hint file %s [%s]", xl.hintFile, err.Error())
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fileName := strings.TrimSpace(scanner.Text())
		if fileName == "" {
			continue
		}

		xl.hintMu.Lock()
		_, seen := xl.hintSeen[fileName]
		if !seen {
			xl.hintSeen[fileName] = struct{}{}
		}
		xl.hintMu.Unlock()

		if seen {
			continue
		}

		log.Debug("Xload::processHintFile : scheduling download for hinted file %s", fileName)
		if err := xl.downloadFile(fileName); err != nil {
			log.Err("Xload::processHintFile : failed to schedule %s [%s]", fileName, err.Error())
		}
	}

	if err := scanner.Err(); err != nil {
		log.Err("Xload::processHintFile : scanner error for %s [%s]", xl.hintFile, err.Error())
	}
}

// cacheNeedsEviction is invoked by the eviction policy to decide whether to evict entries.
func (xl *Xload) cacheNeedsEviction() bool {
	if xl.maxCacheSizeMB <= 0 {
		return false
	}

	usageMB, err := common.GetUsage(xl.path)
	if err != nil {
		log.Warn("Xload::cacheNeedsEviction : unable to read usage for %s [%s]", xl.path, err.Error())
		return false
	}

	if usageMB >= xl.maxCacheSizeMB {
		log.Info("Xload::cacheNeedsEviction : cache usage %.2f MB >= cap %.2f MB", usageMB, xl.maxCacheSizeMB)
		return true
	}

	return false
}

// cacheEvict removes a cached file when eviction policy picks it.
func (xl *Xload) cacheEvict(node *list.Element) {
	fileName := node.Value.(string)

	if xl.fileLocks.Locked(fileName) {
		log.Info("Xload::cacheEvict : skipping eviction for locked file %s", fileName)
		return
	}

	flock := xl.fileLocks.Get(fileName)
	flock.Lock()
	defer flock.Unlock()

	xl.cacheNodes.Delete(fileName)

	localPath := filepath.Join(xl.path, fileName)
	if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
		log.Warn("Xload::cacheEvict : failed to remove %s [%s]", localPath, err.Error())
	} else {
		log.Info("Xload::cacheEvict : evicted %s", fileName)
	}
}

// touchCacheEntry refreshes recency for a cached file and enrolls new items into the eviction policy.
func (xl *Xload) touchCacheEntry(name string) {
	if xl.evictPolicy == nil {
		return
	}

	node, found := xl.cacheNodes.Load(name)
	if !found {
		node = xl.evictPolicy.Add(name)
		xl.cacheNodes.Store(name, node)
	} else {
		xl.evictPolicy.Refresh(node.(*list.Element))
	}
}

func (xl *Xload) getSplitter() XComponent {
	for _, c := range xl.comps {
		if c.GetName() == SPLITTER {
			return c
		}
	}

	return nil
}

// downloadFile sends the file to splitter to be downloaded on priority
func (xl *Xload) downloadFile(fileName string) error {
	log.Debug("Xload::downloadFile : download file %s", fileName)
	splitter := xl.getSplitter()
	if splitter == nil {
		log.Err("Xload::downloadFile : failed to  get download splitter for %s", fileName)
		return fmt.Errorf("failed to  get download splitter")
	}

	attr, err := xl.NextComponent().GetAttr(internal.GetAttrOptions{Name: fileName})
	if err != nil {
		log.Err("Xload::downloadFile : Failed to get attr of %s [%s]", fileName, err.Error())
		return err
	}

	xl.touchCacheEntry(fileName)

	fileMode := xl.defaultPermission
	if !attr.IsModeDefault() {
		fileMode = attr.Mode
	}

	// create the local path where the file will be downloaded
	err = os.MkdirAll(filepath.Dir(filepath.Join(xl.path, fileName)), xl.defaultPermission)
	if err != nil {
		log.Err("Xload::downloadFile : Failed to create local directory for %s [%s]", fileName, err.Error())
		return err
	}

	_, err = splitter.Process(&WorkItem{
		CompName: splitter.GetName(),
		Path:     fileName,
		DataLen:  uint64(attr.Size),
		Priority: true,
		Mode:     fileMode,
		Atime:    attr.Atime,
		Mtime:    attr.Mtime,
		MD5:      attr.MD5,
	})

	if err != nil {
		log.Err("Xload::downloadFile : failed to download file %s [%s]", fileName, err.Error())
		return err
	}

	return nil
}

// OpenFile: Download the file if not already downloaded and return the file handle
func (xl *Xload) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("Xload::OpenFile : name=%s, flags=%s, mode=%s",
		options.Name, common.PrettyOpenFlags(options.Flags), options.Mode)

	localPath := filepath.Join(xl.path, options.Name)

	flock := xl.fileLocks.Get(options.Name)
	flock.Lock()
	defer flock.Unlock()

	filePresent, _, _ := isFilePresent(localPath)

	// if file is not present, send it to splitter for downloading on priority
	if !filePresent {
		err := xl.downloadFile(options.Name)
		if err != nil {
			log.Err("Xload::OpenFile : failed to download file %s [%s]", options.Name, err.Error())
			return nil, err
		}

	} else {
		log.Debug("Xload::OpenFile : %s will be served from local path", options.Name)
	}

	xl.touchCacheEntry(options.Name)

	fh, err := os.OpenFile(localPath, options.Flags, options.Mode)
	if err != nil {
		log.Err("Xload::OpenFile : error opening cached file %s [%s]", options.Name, err.Error())
		return nil, err
	}

	// Increment the handle count in this lock item as there is one handle open for this now
	flock.Inc()

	handle := handlemap.NewHandle(options.Name)
	info, err := fh.Stat()
	if err == nil {
		handle.Size = info.Size()
	}

	handle.UnixFD = uint64(fh.Fd())
	handle.Flags.Set(handlemap.HandleFlagCached)

	log.Info("Xload::OpenFile : file=%s, fd=%d", options.Name, fh.Fd())
	handle.SetFileObject(fh)

	return handle, nil
}

func (xl *Xload) ReleaseFile(options internal.ReleaseFileOptions) error {
	// Lock the file so that while close is in progress no one can open the file again
	flock := xl.fileLocks.Get(options.Handle.Path)
	flock.Lock()
	defer flock.Unlock()

	flock.Dec()
	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
func NewXloadComponent() internal.Component {
	comp := &Xload{
		fileLocks: common.NewLockMap(),
	}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewXloadComponent)

	workers := config.AddInt32Flag("workers", 100, "number of workers to execute parallel download during preload")
	config.BindPFlag(compName+".workers", workers)

	poolSize := config.AddInt32Flag("pool-size", 300, "number of blocks in the blockpool for preload")
	config.BindPFlag(compName+".pool-size", poolSize)

	hintFile := config.AddStringFlag("prefetch-hint-file", "", "path to a prefetch hint file for xload")
	config.BindPFlag(compName+".prefetch-hint-file", hintFile)

	hintPoll := config.AddInt32Flag("prefetch-hint-poll-sec", 2, "poll interval in seconds for the xload hint file")
	config.BindPFlag(compName+".prefetch-hint-poll-sec", hintPoll)
}
