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

package azstorage

import (
	"context"
	"fmt"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"

	"github.com/spf13/cobra"
)

// AzStorage Wrapper type around azure go-sdk (track-1)
type AzStorage struct {
	internal.BaseComponent
	storage     AzConnection
	stConfig    AzStorageConfig
	startTime   time.Time
	listBlocked bool
}

const compName = "azstorage"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &AzStorage{}

var azStatsCollector *stats_manager.StatsCollector

func (az *AzStorage) Name() string {
	return az.BaseComponent.Name()
}

func (az *AzStorage) SetName(name string) {
	az.BaseComponent.SetName(name)
}

func (az *AzStorage) SetNextComponent(c internal.Component) {
	az.BaseComponent.SetNextComponent(c)
}

func (az *AzStorage) GetBlobStorage() AzConnection {
	return az.storage
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
func (az *AzStorage) Configure(isParent bool) error {
	log.Trace("AzStorage::Configure : %s", az.Name())

	conf := AzStorageOptions{}
	err := config.UnmarshalKey(az.Name(), &conf)
	if err != nil {
		log.Err("AzStorage::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", az.Name(), err.Error())
	}

	err = ParseAndValidateConfig(az, conf)
	if err != nil {
		log.Err("AzStorage::Configure : Config validation failed [%s]", err.Error())
		return fmt.Errorf("config error in %s [%s]", az.Name(), err.Error())
	}

	err = az.configureAndTest(isParent)
	if err != nil {
		log.Err("AzStorage::Configure : Failed to validate storage account [%s]", err.Error())
		return err
	}

	return nil
}

func (az *AzStorage) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.Consumer()
}

// OnConfigChange : When config file is changed, this will be called by pipeline. Refresh required config here
func (az *AzStorage) OnConfigChange() {
	log.Trace("AzStorage::OnConfigChange : %s", az.Name())

	conf := AzStorageOptions{}
	err := config.UnmarshalKey(az.Name(), &conf)
	if err != nil {
		log.Err("AzStorage::OnConfigChange : Config error [invalid config attributes]")
		return
	}

	err = ParseAndReadDynamicConfig(az, conf, true)
	if err != nil {
		log.Err("AzStorage::OnConfigChange : failed to reparse config", err.Error())
		return
	}

	err = az.storage.UpdateConfig(az.stConfig)
	if err != nil {
		log.Err("AzStorage::OnConfigChange : failed to UpdateConfig", err.Error())
		return
	}

	// dynamic update of the sdk log listener
	setSDKLogListener()
}

func (az *AzStorage) configureAndTest(isParent bool) error {
	az.storage = NewAzStorageConnection(az.stConfig)

	err := az.storage.SetupPipeline()
	if err != nil {
		log.Err("AzStorage::configureAndTest : Failed to create container URL [%s]", err.Error())
		return err
	}

	// set SDK log listener to log the requests and responses
	setSDKLogListener()

	err = az.storage.SetPrefixPath(az.stConfig.prefixPath)
	if err != nil {
		log.Err("AzStorage::configureAndTest : Failed to set prefix path [%s]", err.Error())
		return err
	}

	// The daemon runs all pipeline Configure code twice. isParent allows us to only validate credentials in parent mode, preventing a second unnecessary REST call.
	if isParent {
		err = az.storage.TestPipeline()
		if err != nil {
			log.Err("AzStorage::configureAndTest : Failed to validate credentials [%s]", err.Error())
			return fmt.Errorf("failed to authenticate %s credentials with error [%s]", az.Name(), err.Error())
		}
	}

	return nil
}

// Start : Initialize the go-sdk pipeline here and test auth is working fine
func (az *AzStorage) Start(ctx context.Context) error {
	log.Trace("AzStorage::Start : Starting component %s", az.Name())
	// On mount block the ListBlob call for certain amount of time
	az.startTime = time.Now()
	az.listBlocked = true

	// create stats collector for azstorage
	azStatsCollector = stats_manager.NewStatsCollector(az.Name())

	return nil
}

// Stop : Disconnect all running operations here
func (az *AzStorage) Stop() error {
	log.Trace("AzStorage::Stop : Stopping component %s", az.Name())
	azStatsCollector.Destroy()
	return nil
}

// ------------------------- Container listing -------------------------------------------
func (az *AzStorage) ListContainers() ([]string, error) {
	return az.storage.ListContainers()
}

// ------------------------- Core Operations -------------------------------------------

// Directory operations
func (az *AzStorage) CreateDir(options internal.CreateDirOptions) error {
	log.Trace("AzStorage::CreateDir : %s", options.Name)

	err := az.storage.CreateDirectory(internal.TruncateDirName(options.Name), options.Etag)

	if err == nil {
		azStatsCollector.PushEvents(createDir, options.Name, map[string]interface{}{mode: options.Mode.String()})
		azStatsCollector.UpdateStats(stats_manager.Increment, createDir, (int64)(1))
	}

	return err
}

func (az *AzStorage) DeleteDir(options internal.DeleteDirOptions) error {
	log.Trace("AzStorage::DeleteDir : %s", options.Name)

	err := az.storage.DeleteDirectory(internal.TruncateDirName(options.Name))

	if err == nil {
		azStatsCollector.PushEvents(deleteDir, options.Name, nil)
		azStatsCollector.UpdateStats(stats_manager.Increment, deleteDir, (int64)(1))
	}

	return err
}

func formatListDirName(path string) string {
	// If we check the root directory, make sure we pass "" instead of "/"
	// If we aren't checking the root directory, then we want to extend the directory name so List returns all children and does not include the path itself.
	if path == "/" {
		path = ""
	} else if path != "" {
		path = internal.ExtendDirName(path)
	}
	return path
}

func (az *AzStorage) IsDirEmpty(options internal.IsDirEmptyOptions) bool {
	log.Trace("AzStorage::IsDirEmpty : %s", options.Name)
	list, _, err := az.storage.List(formatListDirName(options.Name), nil, 1)
	if err != nil {
		log.Err("AzStorage::IsDirEmpty : error listing [%s]", err)
		return false
	}
	if len(list) == 0 {
		return true
	}
	return false
}

func (az *AzStorage) ReadDir(options internal.ReadDirOptions) ([]*internal.ObjAttr, error) {
	log.Trace("AzStorage::ReadDir : %s", options.Name)
	blobList := make([]*internal.ObjAttr, 0)

	if az.listBlocked {
		diff := time.Since(az.startTime)
		if diff.Seconds() > float64(az.stConfig.cancelListForSeconds) {
			az.listBlocked = false
			log.Info("AzStorage::ReadDir : Unblocked List API")
		} else {
			log.Info("AzStorage::ReadDir : Blocked List API for %d more seconds", int(az.stConfig.cancelListForSeconds)-int(diff.Seconds()))
			return blobList, nil
		}
	}

	path := formatListDirName(options.Name)
	var iteration int = 0
	var marker *string = nil
	for {
		new_list, new_marker, err := az.storage.List(path, marker, common.MaxDirListCount)
		if err != nil {
			log.Err("AzStorage::ReadDir : Failed to read dir [%s]", err)
			return blobList, err
		}
		blobList = append(blobList, new_list...)
		marker = new_marker
		iteration++

		log.Debug("AzStorage::ReadDir : So far retrieved %d objects in %d iterations", len(blobList), iteration)
		if new_marker == nil || *new_marker == "" {
			break
		}
	}

	return blobList, nil
}

func (az *AzStorage) StreamDir(options internal.StreamDirOptions) ([]*internal.ObjAttr, string, error) {
	log.Trace("AzStorage::StreamDir : Path %s, offset %d, count %d", options.Name, options.Offset, options.Count)

	if az.listBlocked {
		diff := time.Since(az.startTime)
		if diff.Seconds() > float64(az.stConfig.cancelListForSeconds) {
			az.listBlocked = false
			log.Info("AzStorage::StreamDir : Unblocked List API")
		} else {
			log.Info("AzStorage::StreamDir : Blocked List API for %d more seconds", int(az.stConfig.cancelListForSeconds)-int(diff.Seconds()))
			return make([]*internal.ObjAttr, 0), "", nil
		}
	}

	path := formatListDirName(options.Name)

	new_list, new_marker, err := az.storage.List(path, &options.Token, options.Count)
	if err != nil {
		log.Err("AzStorage::StreamDir : Failed to read dir [%s]", err)
		return new_list, "", err
	}

	log.Debug("AzStorage::StreamDir : Retrieved %d objects with %s marker for Path %s", len(new_list), options.Token, path)

	if new_marker == nil {
		new_marker = to.Ptr("")
	} else if *new_marker != "" {
		log.Debug("AzStorage::StreamDir : next-marker %s for Path %s", *new_marker, path)
		if len(new_list) == 0 {
			/* In some customer scenario we have seen that new_list is empty but marker is not empty
			   which means backend has not returned any items this time but there are more left.
			   If we return back this empty list to libfuse layer it will assume listing has completed
			   and will terminate the readdir call. As there are more items left on the server side we
			   need to retry getting a list here.
			*/
			log.Warn("AzStorage::StreamDir : next-marker %s but current list is empty. Need to retry listing", *new_marker)
			options.Token = *new_marker
			return az.StreamDir(options)
		}
	}

	// if path is empty, it means it is the root, relative to the mounted directory
	if len(path) == 0 {
		path = "/"
	}
	azStatsCollector.PushEvents(streamDir, path, map[string]interface{}{count: len(new_list)})

	// increment streamdir call count
	azStatsCollector.UpdateStats(stats_manager.Increment, streamDir, (int64)(1))

	return new_list, *new_marker, nil
}

func (az *AzStorage) RenameDir(options internal.RenameDirOptions) error {
	log.Trace("AzStorage::RenameDir : %s to %s", options.Src, options.Dst)
	options.Src = internal.TruncateDirName(options.Src)
	options.Dst = internal.TruncateDirName(options.Dst)

	err := az.storage.RenameDirectory(options.Src, options.Dst)

	if err == nil {
		azStatsCollector.PushEvents(renameDir, options.Src, map[string]interface{}{src: options.Src, dest: options.Dst})
		azStatsCollector.UpdateStats(stats_manager.Increment, renameDir, (int64)(1))
	}
	return err
}

// File operations
func (az *AzStorage) CreateFile(options internal.CreateFileOptions) (*handlemap.Handle, error) {
	log.Trace("AzStorage::CreateFile : %s", options.Name)

	// Create a handle object for the file being created
	// This handle will be added to handlemap by the first component in pipeline
	handle := handlemap.NewHandle(options.Name)
	if handle == nil {
		log.Err("AzStorage::CreateFile : Failed to create handle for %s", options.Name)
		return nil, syscall.EFAULT
	}

	err := az.storage.CreateFile(options.Name, options.Mode)
	if err != nil {
		return nil, err
	}
	handle.Mtime = time.Now()

	azStatsCollector.PushEvents(createFile, options.Name, map[string]interface{}{mode: options.Mode.String()})

	// increment open file handles count
	azStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return handle, nil
}

func (az *AzStorage) OpenFile(options internal.OpenFileOptions) (*handlemap.Handle, error) {
	log.Trace("AzStorage::OpenFile : %s", options.Name)

	attr, err := az.storage.GetAttr(options.Name)
	if err != nil {
		return nil, err
	}

	// Create a handle object for the file being opened
	// This handle will be added to handlemap by the first component in pipeline
	handle := handlemap.NewHandle(options.Name)
	if handle == nil {
		log.Err("AzStorage::OpenFile : Failed to create handle for %s", options.Name)
		return nil, syscall.EFAULT
	}
	handle.Size = int64(attr.Size)
	handle.Mtime = attr.Mtime

	// increment open file handles count
	azStatsCollector.UpdateStats(stats_manager.Increment, openHandles, (int64)(1))

	return handle, nil
}

func (az *AzStorage) CloseFile(options internal.CloseFileOptions) error {
	log.Trace("AzStorage::CloseFile : %s", options.Handle.Path)

	// decrement open file handles count
	azStatsCollector.UpdateStats(stats_manager.Decrement, openHandles, (int64)(1))

	return nil
}

func (az *AzStorage) DeleteFile(options internal.DeleteFileOptions) error {
	log.Trace("AzStorage::DeleteFile : %s", options.Name)

	err := az.storage.DeleteFile(options.Name)

	if err == nil {
		azStatsCollector.PushEvents(deleteFile, options.Name, nil)
		azStatsCollector.UpdateStats(stats_manager.Increment, deleteFile, (int64)(1))
	}

	return err
}

func (az *AzStorage) RenameFile(options internal.RenameFileOptions) error {
	log.Trace("AzStorage::RenameFile : %s to %s", options.Src, options.Dst)

	err := az.storage.RenameFile(options.Src, options.Dst, options.SrcAttr)

	if err == nil {
		azStatsCollector.PushEvents(renameFile, options.Src, map[string]interface{}{src: options.Src, dest: options.Dst})
		azStatsCollector.UpdateStats(stats_manager.Increment, renameFile, (int64)(1))
	}
	return err
}

func (az *AzStorage) ReadFile(options internal.ReadFileOptions) (data []byte, err error) {
	//log.Trace("AzStorage::ReadFile : Read %s", h.Path)
	return az.storage.ReadBuffer(options.Handle.Path, 0, 0)
}

func (az *AzStorage) ReadFileWithName(options internal.ReadFileWithNameOptions) (data []byte, err error) {
	return az.storage.ReadBuffer(options.Path, 0, 0)
}

func (az *AzStorage) ReadInBuffer(options internal.ReadInBufferOptions) (length int, err error) {
	//log.Trace("AzStorage::ReadInBuffer : Read %s from %d offset", h.Path, offset)

	if options.Offset > atomic.LoadInt64(&options.Handle.Size) {
		return 0, syscall.ERANGE
	}

	var dataLen int64 = int64(len(options.Data))
	if atomic.LoadInt64(&options.Handle.Size) < (options.Offset + int64(len(options.Data))) {
		dataLen = options.Handle.Size - options.Offset
	}

	if dataLen == 0 {
		return 0, nil
	}

	err = az.storage.ReadInBuffer(options.Handle.Path, options.Offset, dataLen, options.Data, options.Etag)

	if err != nil {
		log.Err("AzStorage::ReadInBuffer : Failed to read %s [%s]", options.Handle.Path, err.Error())
	}

	length = int(dataLen)
	return
}

func (az *AzStorage) WriteFile(options internal.WriteFileOptions) (int, error) {
	err := az.storage.Write(options)
	return len(options.Data), err
}

func (az *AzStorage) GetFileBlockOffsets(options internal.GetFileBlockOffsetsOptions) (*common.BlockOffsetList, error) {
	return az.storage.GetFileBlockOffsets(options.Name)

}

func (az *AzStorage) TruncateFile(options internal.TruncateFileOptions) error {
	log.Trace("AzStorage::TruncateFile : %s to %d bytes", options.Name, options.Size)
	err := az.storage.TruncateFile(options.Name, options.Size)

	if err == nil {
		azStatsCollector.PushEvents(truncateFile, options.Name, map[string]interface{}{size: options.Size})
		azStatsCollector.UpdateStats(stats_manager.Increment, truncateFile, (int64)(1))
	}
	return err
}

func (az *AzStorage) CopyToFile(options internal.CopyToFileOptions) error {
	log.Trace("AzStorage::CopyToFile : Read file %s", options.Name)
	return az.storage.ReadToFile(options.Name, options.Offset, options.Count, options.File)
}

func (az *AzStorage) CopyFromFile(options internal.CopyFromFileOptions) error {
	log.Trace("AzStorage::CopyFromFile : Upload file %s", options.Name)
	return az.storage.WriteFromFile(options.Name, options.Metadata, options.File)
}

// Symlink operations
func (az *AzStorage) CreateLink(options internal.CreateLinkOptions) error {
	log.Trace("AzStorage::CreateLink : Create symlink %s -> %s", options.Name, options.Target)
	err := az.storage.CreateLink(options.Name, options.Target)

	if err == nil {
		azStatsCollector.PushEvents(createLink, options.Name, map[string]interface{}{target: options.Target})
		azStatsCollector.UpdateStats(stats_manager.Increment, createLink, (int64)(1))
	}

	return err
}

func (az *AzStorage) ReadLink(options internal.ReadLinkOptions) (string, error) {
	log.Trace("AzStorage::ReadLink : Read symlink %s", options.Name)
	data, err := az.storage.ReadBuffer(options.Name, 0, options.Size)

	if err != nil {
		azStatsCollector.PushEvents(readLink, options.Name, nil)
		azStatsCollector.UpdateStats(stats_manager.Increment, readLink, (int64)(1))
	}

	return string(data), err
}

// Attribute operations
func (az *AzStorage) GetAttr(options internal.GetAttrOptions) (attr *internal.ObjAttr, err error) {
	//log.Trace("AzStorage::GetAttr : Get attributes of file %s", name)
	return az.storage.GetAttr(options.Name)
}

func (az *AzStorage) Chmod(options internal.ChmodOptions) error {
	log.Trace("AzStorage::Chmod : Change mod of file %s", options.Name)
	err := az.storage.ChangeMod(options.Name, options.Mode)

	if err == nil {
		azStatsCollector.PushEvents(chmod, options.Name, map[string]interface{}{mode: options.Mode.String()})
		azStatsCollector.UpdateStats(stats_manager.Increment, chmod, (int64)(1))
	}

	return err
}

func (az *AzStorage) Chown(options internal.ChownOptions) error {
	log.Trace("AzStorage::Chown : Change ownership of file %s to %d-%d", options.Name, options.Owner, options.Group)
	return az.storage.ChangeOwner(options.Name, options.Owner, options.Group)
}

func (az *AzStorage) FlushFile(options internal.FlushFileOptions) error {
	log.Trace("AzStorage::FlushFile : Flush file %s", options.Handle.Path)
	return az.storage.StageAndCommit(options.Handle.Path, options.Handle.CacheObj.BlockOffsetList)
}

func (az *AzStorage) GetCommittedBlockList(name string) (*internal.CommittedBlockList, error) {
	return az.storage.GetCommittedBlockList(name)
}

func (az *AzStorage) StageData(opt internal.StageDataOptions) error {
	return az.storage.StageBlock(opt.Name, opt.Data, opt.Id)
}

func (az *AzStorage) CommitData(opt internal.CommitDataOptions) error {
	return az.storage.CommitBlocks(opt.Name, opt.List, opt.NewETag)
}

func (az *AzStorage) WriteFromBuffer(opt internal.WriteFromBufferOptions) error {
	return az.storage.WriteFromBuffer(opt)
}

// TODO : Below methods are pending to be implemented
// SetAttr(string, internal.ObjAttr) error
// UnlinkFile(string) error
// ReleaseFile(*handlemap.Handle) error
// FlushFile(*handlemap.Handle) error

// ------------------------- Factory methods to create objects -------------------------------------------

// Constructor to create object of this component
func NewazstorageComponent() internal.Component {
	// Init the component with default config
	az := &AzStorage{
		stConfig: AzStorageConfig{
			blockSize:      0,
			maxConcurrency: 32,
			defaultTier:    getAccessTierType(""),
			authConfig: azAuthConfig{
				AuthMode: EAuthType.KEY(),
				UseHTTP:  false,
			},
		},
	}

	az.SetName(compName)
	config.AddConfigChangeEventListener(az)
	return az
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewazstorageComponent)
	RegisterEnvVariables()

	useHttps := config.AddBoolFlag("use-https", true, "Enables HTTPS communication with Blob storage.")
	config.BindPFlag(compName+".use-https", useHttps)
	useHttps.Hidden = true

	blockListSecFlag := config.AddInt32Flag("cancel-list-on-mount-seconds", 0, "Number of seconds list call is blocked post mount")
	config.BindPFlag(compName+".block-list-on-mount-sec", blockListSecFlag)
	blockListSecFlag.Hidden = true

	containerNameFlag := config.AddStringFlag("container-name", "", "Configures the name of the container to be mounted")
	config.BindPFlag(compName+".container", containerNameFlag)

	useAdls := config.AddBoolFlag("use-adls", false, "Enables blobfuse to access Azure DataLake storage account.")
	config.BindPFlag(compName+".use-adls", useAdls)
	useAdls.Hidden = true

	maxConcurrency := config.AddUint16Flag("max-concurrency", 32, "Option to override default number of concurrent storage connections")
	config.BindPFlag(compName+".max-concurrency", maxConcurrency)
	maxConcurrency.Hidden = true

	httpProxy := config.AddStringFlag("http-proxy", "", "HTTP Proxy address.")
	config.BindPFlag(compName+".http-proxy", httpProxy)
	httpProxy.Hidden = true

	httpsProxy := config.AddStringFlag("https-proxy", "", "HTTPS Proxy address.")
	config.BindPFlag(compName+".https-proxy", httpsProxy)
	httpsProxy.Hidden = true

	maxRetry := config.AddUint16Flag("max-retry", 3, "Maximum retry count if the failure codes are retryable.")
	config.BindPFlag(compName+".max-retries", maxRetry)
	maxRetry.Hidden = true

	maxRetryInterval := config.AddUint16Flag("max-retry-interval-in-seconds", 3, "Maximum number of seconds between 2 retries.")
	config.BindPFlag(compName+".max-retry-timeout-sec", maxRetryInterval)
	maxRetryInterval.Hidden = true

	retryDelayFactor := config.AddUint16Flag("retry-delay-factor", 1, "Retry delay between two tries")
	config.BindPFlag(compName+".retry-backoff-sec", retryDelayFactor)
	retryDelayFactor.Hidden = true

	setContentType := config.AddBoolFlag("set-content-type", true, "Turns on automatic 'content-type' property based on the file extension.")
	config.BindPFlag(compName+".set-content-type", setContentType)
	setContentType.Hidden = true

	caCertFile := config.AddStringFlag("ca-cert-file", "", "Specifies the proxy pem certificate path if its not in the default path.")
	config.BindPFlag(compName+".ca-cert-file", caCertFile)
	caCertFile.Hidden = true

	debugLibcurl := config.AddStringFlag("debug-libcurl", "", "Flag to allow users to debug libcurl calls.")
	config.BindPFlag(compName+".debug-libcurl", debugLibcurl)
	debugLibcurl.Hidden = true

	virtualDir := config.AddBoolFlag("virtual-directory", false, "Support virtual directories without existence of a special marker blob.")
	config.BindPFlag(compName+".virtual-directory", virtualDir)

	subDirectory := config.AddStringFlag("subdirectory", "", "Mount only this sub-directory from given container.")
	config.BindPFlag(compName+".subdirectory", subDirectory)

	disableCompression := config.AddBoolFlag("disable-compression", false, "Disable transport layer compression.")
	config.BindPFlag(compName+".disable-compression", disableCompression)

	telemetry := config.AddStringFlag("telemetry", "", "Additional telemetry information.")
	config.BindPFlag(compName+".telemetry", telemetry)
	telemetry.Hidden = true

	honourACL := config.AddBoolFlag("honour-acl", false, "Match ObjectID in ACL against the one used for authentication.")
	config.BindPFlag(compName+".honour-acl", honourACL)
	honourACL.Hidden = true

	cpkEnabled := config.AddBoolFlag("cpk-enabled", false, "Enable client provided key.")
	config.BindPFlag(compName+".cpk-enabled", cpkEnabled)

	preserveACL := config.AddBoolFlag("preserve-acl", false, "Preserve ACL and Permissions set on file during updates")
	config.BindPFlag(compName+".preserve-acl", preserveACL)

	blobFilter := config.AddStringFlag("filter", "", "Filter string to match blobs")
	config.BindPFlag(compName+".filter", blobFilter)

	config.RegisterFlagCompletionFunc("container-name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
}
