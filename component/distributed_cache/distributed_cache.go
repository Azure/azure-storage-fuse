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

package distributed_cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	cm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/cluster_manager"
	fm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/file_manager"
	mm "github.com/Azure/azure-storage-fuse/v2/internal/dcache/metadata_manager"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type DistributedCache struct {
	internal.BaseComponent
	cfg DistributedCacheOptions // ← holds cache‐id, cache‐dirs, replicas, chunk‐size, etc.

	azstorage       internal.Component
	storageCallback dcache.StorageCallbacks
}

// Structure defining your config parameters
type DistributedCacheOptions struct {
	CacheID   string   `config:"cache-id" yaml:"cache-id,omitempty"`
	CacheDirs []string `config:"cache-dirs" yaml:"cache-dirs,omitempty"`

	ChunkSize  uint64 `config:"chunk-size" yaml:"chunk-size,omitempty"`
	StripeSize uint64 `config:"stripe-size" yaml:"stripe-size,omitempty"`
	Replicas   uint32 `config:"replicas" yaml:"replicas,omitempty"`

	HeartbeatDuration   uint16 `config:"heartbeat-duration" yaml:"heartbeat-duration,omitempty"`
	MaxMissedHeartbeats uint8  `config:"max-missed-heartbeats" yaml:"max-missed-heartbeats,omitempty"`
	RVFullThreshold     uint64 `config:"rv-full-threshold" yaml:"rv-full-threshold,omitempty"`
	RVNearfullThreshold uint64 `config:"rv-nearfull-threshold" yaml:"rv-nearfull-threshold,omitempty"`
	MaxCacheSize        uint64 `config:"max-cache-size" yaml:"max-cache-size,omitempty"`

	MinNodes            uint32 `config:"min-nodes" yaml:"min-nodes,omitempty"`
	MVsPerRv            uint64 `config:"mvs-per-rv" yaml:"mvs-per-rv,omitempty"`
	RebalancePercentage uint8  `config:"rebalance-percentage" yaml:"rebalance-percentage,omitempty"`
	SafeDeletes         bool   `config:"safe-deletes" yaml:"safe-deletes,omitempty"`
	CacheAccess         string `config:"cache-access" yaml:"cache-access,omitempty"`
	ClustermapEpoch     uint64 `config:"clustermap-epoch" yaml:"clustermap-epoch,omitempty"`
}

const (
	compName                         = "distributed_cache"
	defaultHeartBeatDurationInSecond = 30
	defaultReplicas                  = 1
	defaultMaxMissedHBs              = 3
	defaultChunkSize                 = 4 * 1024 * 1024 // 4 MB
	defaultMinNodes                  = 1
	defaultStripeSize                = 16 * 1024 * 1024 // 16 MB
	defaultMvsPerRv                  = 10
	defaultRvFullThreshold           = 95
	defaultRvNearfullThreshold       = 80
	defaultClustermapEpoch           = 300
	defaultRebalancePercentage       = 80
	defaultSafeDeletes               = false
	defaultCacheAccess               = "automatic"
)

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &DistributedCache{}

func (dc *DistributedCache) Name() string {
	return compName
}

func (dc *DistributedCache) SetName(name string) {
	dc.BaseComponent.SetName(name)
}

func (dc *DistributedCache) SetNextComponent(nextComponent internal.Component) {
	dc.BaseComponent.SetNextComponent(nextComponent)
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (dc *DistributedCache) Start(ctx context.Context) error {

	log.Trace("DistributedCache::Start : Starting component %s", dc.Name())

	dc.azstorage = dc.NextComponent()
	for dc.azstorage != nil && dc.azstorage.Name() != "azstorage" {
		dc.azstorage = dc.azstorage.NextComponent()
	}
	if dc.azstorage == nil {
		return log.LogAndReturnError("DistributedCache::Start error [azstorage component not found]")
	}

	dc.storageCallback = initStorageCallback(
		dc.NextComponent(),
		dc.azstorage)

	err := mm.Init(dc.storageCallback, dc.cfg.CacheID)
	if err != nil {
		return log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to start metadata manager : %v]", err))
	}

	errString := dc.startClusterManager()
	if errString != "" {
		return log.LogAndReturnError(errString)
	}
	log.Info("DistributedCache::Start : component started successfully")
	// todo : Replace the hardcoded values with user config values.
	// todo:  Add Init function to fileIOmanager to initialize the defaults.
	fm.NewFileIOManager(10, 4, 4, 4*1024*1024, 100)
	return nil
}

func (dc *DistributedCache) startClusterManager() string {

	dCacheConfig := &dcache.DCacheConfig{
		CacheId:                dc.cfg.CacheID,
		MinNodes:               dc.cfg.MinNodes,
		ChunkSize:              dc.cfg.ChunkSize,
		StripeSize:             dc.cfg.StripeSize,
		NumReplicas:            dc.cfg.Replicas,
		MvsPerRv:               dc.cfg.MVsPerRv,
		HeartbeatSeconds:       dc.cfg.HeartbeatDuration,
		HeartbeatsTillNodeDown: dc.cfg.MaxMissedHeartbeats,
		ClustermapEpoch:        dc.cfg.ClustermapEpoch,
		RebalancePercentage:    dc.cfg.RebalancePercentage,
		SafeDeletes:            dc.cfg.SafeDeletes,
		CacheAccess:            dc.cfg.CacheAccess,
	}
	rvList, err := dc.createRVList()
	if err != nil {
		return fmt.Sprintf("DistributedCache::Start error [Failed to create RV List for cluster manager : %v]", err)
	}
	if cm.Init(dCacheConfig, rvList) != nil {
		return fmt.Sprintf("DistributedCache::Start error [Failed to start cluster manager : %v]", err)
	}
	return ""
}

func (dc *DistributedCache) createRVList() ([]dcache.RawVolume, error) {
	ipaddr, err := getVmIp()
	if err != nil {
		return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to get VM IP : %v]", err))
	}

	uuidVal, err := common.GetNodeUUID()
	if err != nil {
		return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [Failed to retrieve UUID, error: %v]", err))
	}
	rvList := make([]dcache.RawVolume, len(dc.cfg.CacheDirs))
	for index, path := range dc.cfg.CacheDirs {
		// TODO{Akku} : More than 1 cache dir with same rvId for rv, must fail distributed cache startup
		rvId, err := getBlockDeviceUUId(path)
		if err != nil {
			return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to get raw volume UUID: %v]", err))
		}

		totalSpace, availableSpace, err := common.GetDiskSpaceMetricsFromStatfs(path)
		if err != nil {
			return nil, log.LogAndReturnError(fmt.Sprintf("DistributedCache::Start error [failed to evaluate local cache Total space: %v]", err))
		}

		rvList[index] = dcache.RawVolume{
			NodeId:         uuidVal,
			IPAddress:      ipaddr,
			RvId:           rvId,
			FDID:           "0",
			TotalSpace:     totalSpace,
			AvailableSpace: availableSpace,
			LocalCachePath: path,
		}
	}
	return rvList, nil
}

// Stop : Stop the component functionality and kill all threads started
func (dc *DistributedCache) Stop() error {
	log.Trace("DistributedCache::Stop : Stopping component %s", dc.Name())
	fm.EndFileIOManager()
	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (distributedCache *DistributedCache) Configure(_ bool) error {
	log.Trace("DistributedCache::Configure : %s", distributedCache.Name())

	err := config.UnmarshalKey(distributedCache.Name(), &distributedCache.cfg)

	if err != nil {
		log.Err("DistributedCache::Configure : config error [invalid config attributes]")
		return fmt.Errorf("DistributedCache: config error [invalid config attributes]")
	}
	if distributedCache.cfg.CacheID == "" {
		return fmt.Errorf("config error in %s: [cache-id not set]", distributedCache.Name())
	}
	if len(distributedCache.cfg.CacheDirs) == 0 {
		return fmt.Errorf("config error in %s: [cache-dirs not set]", distributedCache.Name())
	}

	if !config.IsSet(compName + ".replicas") {
		distributedCache.cfg.Replicas = defaultReplicas
	}
	if !config.IsSet(compName + ".heartbeat-duration") {
		distributedCache.cfg.HeartbeatDuration = defaultHeartBeatDurationInSecond
	}
	if !config.IsSet(compName + ".max-missed-heartbeats") {
		distributedCache.cfg.MaxMissedHeartbeats = defaultMaxMissedHBs
	}
	if !config.IsSet(compName + ".chunk-size") {
		distributedCache.cfg.ChunkSize = defaultChunkSize
	}
	if !config.IsSet(compName + ".min-nodes") {
		distributedCache.cfg.MinNodes = defaultMinNodes
	}
	if !config.IsSet(compName + ".stripe-size") {
		distributedCache.cfg.StripeSize = defaultStripeSize
	}
	if !config.IsSet(compName + ".mvs-per-rv") {
		distributedCache.cfg.MVsPerRv = defaultMvsPerRv
	}
	if !config.IsSet(compName + ".rv-full-threshold") {
		distributedCache.cfg.RVFullThreshold = defaultRvFullThreshold
	}
	if !config.IsSet(compName + ".rv-nearfull-threshold") {
		distributedCache.cfg.RVNearfullThreshold = defaultRvNearfullThreshold
	}
	if !config.IsSet(compName + ".clustermap-epoch") {
		distributedCache.cfg.ClustermapEpoch = defaultClustermapEpoch
	}
	if !config.IsSet(compName + ".rebalance-percentage") {
		distributedCache.cfg.RebalancePercentage = defaultRebalancePercentage
	}
	if !config.IsSet(compName + ".safe-deletes") {
		distributedCache.cfg.SafeDeletes = defaultSafeDeletes
	}
	if !config.IsSet(compName + ".cache-access") {
		distributedCache.cfg.CacheAccess = defaultCacheAccess
	}
	return nil
}

// OnConfigChange : If component has registered, on config file change this method is called
func (dc *DistributedCache) OnConfigChange() {
}

func (dc *DistributedCache) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	if strings.HasPrefix(options.Name, "__CACHE__") {
		return nil, syscall.ENOENT
	}

	isAzurePath, isDcachePath, rawPath := getFS(options.Name)
	if isMountPointRoot(rawPath) {
		if isAzurePath {
			return getPlaceholderDirForRoot("fs=azure"), nil
		} else if isDcachePath {
			return getPlaceholderDirForRoot("fs=dcache"), nil
		}
	}
	if isAzurePath {
		// properties should be fetched from Azure
		log.Debug("DistributedCache::GetAttr : Path is having Azure subcomponent, path : %s", options.Name)
	} else if isDcachePath {
		// properties should be fetched from Dcache
		log.Debug("DistributedCache::GetAttr : Path is having Dcache subcomponent, path : %s", options.Name)
		// todo :: call GetMdRoot() from metadata manager
		rawPath = filepath.Join("__CACHE__"+dc.cfg.CacheID, "Objects", rawPath)
	} else {
		common.Assert(rawPath == options.Name)
	}

	attr, err := dc.NextComponent().GetAttr(internal.GetAttrOptions{Name: rawPath})
	if err != nil {
		return nil, err
	}
	// Modify the attr if it came from specific virtual component.
	// todo : if the path is fs=dcache/*, then populate size, times from the fileLayout
	if isAzurePath || isDcachePath {
		attr.Path = options.Name
		attr.Name = filepath.Base(options.Name)
	}
	return attr, nil
}

func (dc *DistributedCache) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	isAzurePath, isDcachePath, rawPath := getFS(options.Name)
	if isAzurePath {
		// properties should be fetched from Azure
		log.Debug("DistributedCache::StreamDir : Path is having Azure subcomponent, path : %s", options.Name)
	} else if isDcachePath {
		// properties should be fetched from Dcache
		log.Debug("DistributedCache::StreamDir : Path is having Dcache subcomponent, path : %s", options.Name)
		// todo :: call GetMdRoot() from metadata manager
		rawPath = filepath.Join("__CACHE__"+dc.cfg.CacheID, "Objects", rawPath)
	} else {
		// properties should be fetched from Azure
		common.Assert(rawPath == options.Name)
	}
	options.Name = rawPath
	dirList, token, err := dc.NextComponent().StreamDir(options)
	if err != nil {
		return dirList, token, err
	}
	// todo : parse the attributes of the file like size,etc.. from the file layout.
	// If the attributes come for the dcache virtual component.
	if isMountPointRoot(rawPath) {
		// todo : Show cache metadata when debug is enabled.
		dirList = hideCacheMetadata(dirList)
	}
	return dirList, token, nil
}

func (dc *DistributedCache) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	var dcFile *fm.DcacheFile
	var handle *handlemap.Handle
	var err error
	isAzurePath, isDcachePath, rawPath := getFS(options.Name)
	if isAzurePath {
		log.Debug("DistributedCache::CreateFile : Path is having Azure subcomponent, path : %s", options.Name)
		options.Name = rawPath
		handle, err = dc.NextComponent().CreateFile(options)
		if err != nil {
			log.Err("DistributedCache::CreateFile : File Creation failed with err : %s Azure subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else if isDcachePath {
		log.Debug("DistributedCache::CreateFile : Path is having Dcache subcomponent, path : %s", options.Name)
		options.Name = rawPath
		dcFile, err = fm.NewDcacheFile(rawPath)
		if err != nil {
			log.Err("DistributedCache::CreateFile : File Creation failed with err : %s Dcache subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else {
		common.Assert(options.Name == rawPath)
		dcFile, err = fm.NewDcacheFile(rawPath)
		if err != nil {
			log.Err("DistributedCache::CreateFile : File Creation failed with err : %s Dcache subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}

		handle, err = dc.NextComponent().CreateFile(options)
		if err != nil {
			log.Err("DistributedCache::CreateFile : File Creation failed with err : %s Azure subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}
		// todo : if one is success and other is failure, get to the previous state by removing the
		// created entries for the files.
	}

	if handle == nil {
		handle = handlemap.NewHandle(options.Name)
	}

	// Set the respective filesystems that this path can access
	if isAzurePath {
		handle.SetFsAzure()
	} else if isDcachePath {
		handle.SetFsDcache()
	} else {
		handle.SetFsDefault()
	}

	// Set Dcache file inside the handle
	handle.IFObj = dcFile

	// Only the files created with dcache needs flush.
	handle.Flags.Set(handlemap.HandleFlagDcacheAllowWrites)

	return handle, nil
}
func (dc *DistributedCache) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	if options.Flags&os.O_WRONLY != 0 || options.Flags&os.O_RDWR != 0 {
		log.Err("DistributedCache::OpenFile: Writing to an exisiting File is not allowed, file : %s", options.Name)
		return nil, syscall.EACCES
	}

	var dcFile *fm.DcacheFile
	var handle *handlemap.Handle
	var err error

	isAzurePath, isDcachePath, rawPath := getFS(options.Name)
	if isAzurePath {
		log.Debug("DistributedCache::OpenFile : Path is having Azure subcomponent, path : %s", options.Name)
		options.Name = rawPath
		handle, err = dc.NextComponent().OpenFile(options)
		if err != nil {
			log.Err("DistributedCache::OpenFile : File Open failed with err : %s Azure subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else if isDcachePath {
		log.Debug("DistributedCache::OpenFile : Path is having Dcache subcomponent, path : %s", options.Name)
		options.Name = rawPath
		dcFile, err = fm.OpenDcacheFile(options.Name)
		if err != nil {
			log.Err("DistributedCache::OpenFile : File Open failed with err : %s Dcache subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}
	} else {
		common.Assert(options.Name == rawPath)
		dcFile, err = fm.OpenDcacheFile(rawPath)
		if err != nil {
			log.Err("DistributedCache::OpenFile : File Open failed with err : %s Dcache subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}
		handle, err = dc.NextComponent().OpenFile(options)
		if err != nil {
			log.Err("DistributedCache::OpenFile : File Open failed with err : %s Azure subcomponent, path : %s", err.Error(), options.Name)
			return nil, err
		}
	}

	if handle == nil {
		handle = handlemap.NewHandle(options.Name)
	}

	// Set the respective filesystems that this path can access
	if isAzurePath {
		handle.SetFsAzure()
	} else if isDcachePath {
		handle.SetFsDcache()
	} else {
		handle.SetFsDefault()
	}

	// Set Dcache file inside the handle
	handle.IFObj = dcFile

	return handle, nil
}

func (dc *DistributedCache) ReadInBuffer(options internal.ReadInBufferOptions) (int, error) {
	log.Err("DistributedCache::ReadInBuffer : ReadInBuffer, offset : %d, buf size : %d, file : %s",
		options.Offset, len(options.Data), options.Handle.Path)

	var dcacheErr, azureErr error
	var bytesRead int
	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.IFObj != nil)
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		bytesRead, dcacheErr = dcFile.ReadFile(options.Offset, options.Data)
		if dcacheErr == nil || dcacheErr == io.EOF {
			return bytesRead, dcacheErr
		}
		common.Assert(bytesRead == 0)
		log.Err("DistributedCache::ReadInBuffer : Failed to read the file from the Dcache, offset : %d, file : %s",
			options.Offset, options.Handle.Path)
		// Let's try to read from the azure, if the handle has the access
	}
	if options.Handle.IsFsAzure() {
		bytesRead, azureErr = dc.NextComponent().ReadInBuffer(options)
		if azureErr == nil || azureErr == io.EOF {
			return bytesRead, azureErr
		}
		log.Err("DistributedCache::ReadInBuffer : Failed to read the file from the Azure, offset : %d, file : %s",
			options.Offset, options.Handle.Path)
	}
	err := errors.Join(dcacheErr, azureErr)
	common.Assert(err != nil)
	return 0, err
}

func (dc *DistributedCache) WriteFile(options internal.WriteFileOptions) (int, error) {
	log.Err("DistributedCache::WriteFile : WriteFile, offset : %d, buf size : %d, file : %s",
		options.Offset, len(options.Data), options.Handle.Path)

	if !options.Handle.Flags.IsSet(handlemap.HandleFlagDcacheAllowWrites) {
		return 0, syscall.EIO
	}

	// Set the handle is dirty to get the flush call.
	options.Handle.Flags.Set(handlemap.HandleFlagDirty)
	var dcacheErr, azureErr error
	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.IFObj != nil)
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		dcacheErr = dcFile.WriteFile(options.Offset, options.Data)
		if dcacheErr != nil {
			// If write on one media fails, then return err instantly
			log.Err("DistributedCache::WriteFile : Failed to write the file from the Dcache, offset : %d, file : %s",
				options.Offset, options.Handle.Path)
			return 0, dcacheErr
		}
	}
	if options.Handle.IsFsAzure() {
		_, azureErr = dc.NextComponent().WriteFile(options)
		if azureErr != nil {
			log.Err("DistributedCache::WriteFile : Failed to write the file from the Azure, offset : %d, file : %s",
				options.Offset, options.Handle.Path)
			return 0, azureErr
		}
	}
	return len(options.Data), nil
}

func (dc *DistributedCache) SyncFile(options internal.SyncFileOptions) error {
	log.Debug("DistributedCache::SyncFile : SyncFile file : %s", options.Handle.Path)

	var dcacheErr, azureErr error
	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.IFObj != nil)
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		dcacheErr = dcFile.SyncFile()
		if dcacheErr != nil {
			log.Err("DistributedCache::SyncFile : Failed to SyncFile to Dcache file : %s", options.Handle.Path)
		}
	}

	if options.Handle.IsFsAzure() {
		azureErr = dc.NextComponent().SyncFile(options)
		if azureErr != nil {
			log.Err("DistributedCache::SyncFile : Failed to SyncFile to Azure file : %s", options.Handle.Path)
		}
	}
	return errors.Join(dcacheErr, azureErr)
}

func (dc *DistributedCache) FlushFile(options internal.FlushFileOptions) error {
	log.Debug("DistributedCache::FlushFile : Close file : %s", options.Handle.Path)
	// Allow only one Flush/close call per file when writing, if user application duplicates the fd
	// then the writes after fist close would fail.

	var dcacheErr, azureErr error

	if !options.Handle.Flags.IsSet(handlemap.HandleFlagDcacheAllowWrites) {
		return nil
	}

	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.FObj != nil)
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		dcacheErr = dcFile.CloseFile()
		common.Assert(dcacheErr == nil)
		if dcacheErr == nil {
			// Clear this flag to signal no more writes on this handle.
			// Fail any writes that come after this.
			options.Handle.Flags.Clear(handlemap.HandleFlagDcacheAllowWrites)
		}
	}

	if options.Handle.IsFsAzure() {
		azureErr = dc.NextComponent().SyncFile(internal.SyncFileOptions{
			Handle: options.Handle,
		})
		if azureErr != nil {
			log.Err("DistributedCache::SyncFile : Failed to SyncFile to Azure file : %s", options.Handle.Path)
		}
	}
	return errors.Join(dcacheErr, azureErr)
}

// Deallocate all the buffers for the file. This is an async call.
func (dc *DistributedCache) CloseFile(options internal.CloseFileOptions) error {
	log.Debug("DistributedCache::CloseFile : Release file : %s", options.Handle.Path)

	var dcacheErr, azureErr error
	if options.Handle.IsFsDcache() {
		common.Assert(options.Handle.FObj != nil)
		dcFile := options.Handle.IFObj.(*fm.DcacheFile)
		dcacheErr = dcFile.ReleaseFile()
		if dcacheErr != nil {
			log.Err("DistributedCache::CloseFile : Failed to ReleaseFile for Dcache file : %s", options.Handle.Path)
		}
	}

	if options.Handle.IsFsAzure() {
		azureErr = dc.NextComponent().CloseFile(options)
		if azureErr != nil {
			log.Err("DistributedCache::SyncFile : Failed to ReleaseFile for Azure file : %s", options.Handle.Path)
		}
	}
	return errors.Join(dcacheErr, azureErr)
}

func (dc *DistributedCache) DeleteFile(options internal.DeleteFileOptions) error {
	return syscall.ENOTSUP
}

func (dc *DistributedCache) RenameFile(options internal.RenameFileOptions) error {
	return syscall.ENOTSUP
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewDistributedCacheComponent() internal.Component {
	comp := &DistributedCache{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewDistributedCacheComponent)

	cacheID := config.AddStringFlag("cache-id", "", "Cache ID for the distributed cache")
	config.BindPFlag(compName+".cache-id", cacheID)

	//TODO{Akku} : Need to update cache-dirs to be a list of strings for command line run, may be use StringSlice
	cachePath := config.AddStringFlag("cache-dirs", "", "Local path(s) of the cache (comma‑separated)")
	config.BindPFlag(compName+".cache-dirs", cachePath)

	chunkSize := config.AddUint64Flag("chunk-size", defaultChunkSize, "Chunk size for the cache")
	config.BindPFlag(compName+".chunk-size", chunkSize)

	maxCacheSize := config.AddUint64Flag("max-cache-size", 0, "Cache size for the cache")
	config.BindPFlag(compName+".max-cache-size", maxCacheSize)

	replicas := config.AddUint32Flag("replicas", defaultReplicas, "Number of replicas for the cache")
	config.BindPFlag(compName+".replicas", replicas)

	heartbeatDuration := config.AddUint16Flag("heartbeat-duration", defaultHeartBeatDurationInSecond, "Heartbeat duration for the cache")
	config.BindPFlag(compName+".heartbeat-duration", heartbeatDuration)

	missedHB := config.AddUint32Flag("max-missed-heartbeats", 3, "Heartbeat absence for the cache")
	config.BindPFlag(compName+".max-missed-heartbeats", missedHB)

	clustermapEpoch := config.AddUint64Flag("clustermap-epoch", defaultClustermapEpoch, "Epoch duration for the clustermap update")
	config.BindPFlag(compName+".clustermap-epoch", clustermapEpoch)

	stripeSize := config.AddUint64Flag("stripe-size", defaultStripeSize, "Stripe size for the cache")
	config.BindPFlag(compName+".stripe-size", stripeSize)

	mvsPerRv := config.AddUint64Flag("mvs-per-rv", defaultMvsPerRv, "Number of MVs per raw volume")
	config.BindPFlag(compName+".mvs-per-rv", mvsPerRv)

	rvFullThreshold := config.AddUint64Flag("rv-full-threshold", defaultRvFullThreshold, "Percent to mark RV full")
	config.BindPFlag(compName+".rv-full-threshold", rvFullThreshold)

	rvNearfullThreshold := config.AddUint64Flag("rv-nearfull-threshold", defaultRvNearfullThreshold, "Percent to mark RV near full")
	config.BindPFlag(compName+".rv-nearfull-threshold", rvNearfullThreshold)

	minNodes := config.AddUint32Flag("min-nodes", defaultMinNodes, "Minimum number of nodes required")
	config.BindPFlag(compName+".min-nodes", minNodes)

	rebalancePercentage := config.AddUint8Flag("rebalance-percentage", defaultRebalancePercentage, "Rebalance threshold percentage")
	config.BindPFlag(compName+".rebalance-percentage", rebalancePercentage)

	safeDeletes := config.AddBoolFlag("safe-deletes", defaultSafeDeletes, "Enable safe‑delete mode")
	config.BindPFlag(compName+".safe-deletes", safeDeletes)

	cacheAccess := config.AddStringFlag("cache-access", defaultCacheAccess, "Cache access mode (automatic/manual)")
	config.BindPFlag(compName+".cache-access", cacheAccess)
}
