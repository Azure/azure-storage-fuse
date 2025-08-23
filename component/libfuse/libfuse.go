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

package libfuse

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/stats_manager"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type Libfuse struct {
	internal.BaseComponent
	mountPath             string
	dirPermission         uint
	filePermission        uint
	readOnly              bool
	attributeExpiration   uint32
	entryExpiration       uint32
	negativeTimeout       uint32
	allowOther            bool
	allowRoot             bool
	ownerUID              uint32
	ownerGID              uint32
	traceEnable           bool
	extensionPath         string
	disableWritebackCache bool
	ignoreOpenFlags       bool
	nonEmptyMount         bool
	lsFlags               common.BitMap16
	maxFuseThreads        uint32
	directIO              bool
	umask                 uint32
	disableKernelCache    bool
}

// To support pagination in readdir calls this structure holds a block of items for a given directory
type dirChildCache struct {
	sIndex   uint64              // start index of current block of items
	eIndex   uint64              // End index of current block of items
	length   uint64              // Length of the children list
	token    string              // Token to get next block of items from container
	children []*internal.ObjAttr // Slice holding current block of children
	// If IsFsDcache is true, we will  enumerate this streamdir call from dcache FS.
	// else we enumerate from Azure FS. This is used when user enumerates through an unqualified path.
	isFsDcache bool
	// This is used when reading the directory for unqualified path. Initially all the entries from the dcache are put
	// into the map to later compare them with the Azure entries to avoid double occurrence of the same entry.
	dcacheEntries map[string]struct{}
}

// Structure defining your config parameters
type LibfuseOptions struct {
	mountPath               string
	DefaultPermission       uint32 `config:"default-permission" yaml:"default-permission,omitempty"`
	AttributeExpiration     uint32 `config:"attribute-expiration-sec" yaml:"attribute-expiration-sec,omitempty"`
	EntryExpiration         uint32 `config:"entry-expiration-sec" yaml:"entry-expiration-sec,omitempty"`
	NegativeEntryExpiration uint32 `config:"negative-entry-expiration-sec" yaml:"negative-entry-expiration-sec,omitempty"`
	EnableFuseTrace         bool   `config:"fuse-trace" yaml:"fuse-trace,omitempty"`
	allowOther              bool   `config:"allow-other" yaml:"-"`
	allowRoot               bool   `config:"allow-root" yaml:"-"`
	readOnly                bool   `config:"read-only" yaml:"-"`
	ExtensionPath           string `config:"extension" yaml:"extension,omitempty"`
	DisableWritebackCache   bool   `config:"disable-writeback-cache" yaml:"-"`
	IgnoreOpenFlags         bool   `config:"ignore-open-flags" yaml:"ignore-open-flags,omitempty"`
	nonEmptyMount           bool   `config:"nonempty" yaml:"nonempty,omitempty"`
	Uid                     uint32 `config:"uid" yaml:"uid,omitempty"`
	Gid                     uint32 `config:"gid" yaml:"gid,omitempty"`
	MaxFuseThreads          uint32 `config:"max-fuse-threads" yaml:"max-fuse-threads,omitempty"`
	DirectIO                bool   `config:"direct-io" yaml:"direct-io,omitempty"`
	Umask                   uint32 `config:"umask" yaml:"umask,omitempty"`
}

const compName = "libfuse"

// Default values for various timeouts in seconds.
// These are passed to libfuse as entry_timeout, attr_timeout and negative_timeout options.
//
// Note: When distributed cache is enabled we use much lower values, see Validate().
const defaultEntryExpiration = 120
const defaultAttrExpiration = 120
const defaultNegativeEntryExpiration = 120

// This is the default value for max_background which controls how many async I/O requests that fuse kernel
// module will keep outstanding to fuse userspace.
const defaultMaxFuseThreads = 128

var fuseFS *Libfuse

var libfuseStatsCollector *stats_manager.StatsCollector

// Bitmasks in Go: https://yourbasic.org/golang/bitmask-flag-set-clear/

var ignoreFiles = map[string]bool{
	".Trash":           true,
	".Trash-1000":      true,
	".xdg-volume-info": true,
	"autorun.inf":      true,
}

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &Libfuse{}

func (lf *Libfuse) Name() string {
	return compName
}

func (lf *Libfuse) SetName(name string) {
	lf.BaseComponent.SetName(name)
}

func (lf *Libfuse) SetNextComponent(nc internal.Component) {
	lf.BaseComponent.SetNextComponent(nc)
}

func (lf *Libfuse) Priority() internal.ComponentPriority {
	return internal.EComponentPriority.Producer()
}

// Start : Pipeline calls this method to start the component functionality
//
//	this shall not block the call otherwise pipeline will not start
func (lf *Libfuse) Start(ctx context.Context) error {
	log.Trace("Libfuse::Start : Starting component %s", lf.Name())

	// create stats collector for libfuse
	libfuseStatsCollector = stats_manager.NewStatsCollector(lf.Name())

	lf.lsFlags = internal.NewDirBitMap()
	lf.lsFlags.Set(internal.PropFlagModeDefault)

	// This marks the global fuse object so shall be the first statement
	fuseFS = lf

	// This starts the libfuse process and hence shall always be the last statement
	err := lf.initFuse()
	if err != nil {
		log.Err("Libfuse::Start : Failed to init fuse [%s]", err.Error())
		return err
	}

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (lf *Libfuse) Stop() error {
	log.Trace("Libfuse::Stop : Stopping component %s", lf.Name())
	_ = lf.destroyFuse()
	libfuseStatsCollector.Destroy()
	return nil
}

// Validate : Validate available config and convert them if required
func (lf *Libfuse) Validate(opt *LibfuseOptions) error {
	lf.mountPath = opt.mountPath
	lf.readOnly = opt.readOnly
	lf.traceEnable = opt.EnableFuseTrace
	lf.allowOther = opt.allowOther
	lf.allowRoot = opt.allowRoot
	lf.extensionPath = opt.ExtensionPath
	lf.disableWritebackCache = opt.DisableWritebackCache
	lf.ignoreOpenFlags = opt.IgnoreOpenFlags
	lf.nonEmptyMount = opt.nonEmptyMount
	lf.directIO = opt.DirectIO
	lf.ownerGID = opt.Gid
	lf.ownerUID = opt.Uid
	lf.umask = opt.Umask

	if lf.disableKernelCache {
		opt.DirectIO = true
		lf.directIO = true
		log.Crit("Libfuse::Validate : Kernel cache disabled, setting direct-io mode in fuse")
	}

	if opt.allowOther {
		lf.dirPermission = uint(common.DefaultAllowOtherPermissionBits)
		lf.filePermission = uint(common.DefaultAllowOtherPermissionBits)
	} else {
		if opt.DefaultPermission != 0 {
			lf.dirPermission = uint(opt.DefaultPermission)
			lf.filePermission = uint(opt.DefaultPermission)
		} else {
			lf.dirPermission = uint(common.DefaultDirectoryPermissionBits)
			lf.filePermission = uint(common.DefaultFilePermissionBits)
		}
	}

	//
	// With distributed cache, we need much lower timeouts o/w it results in a sub-optimal user experience, as
	// changes made by one node (files created and deleted) may not be visible to another node for a long time.
	// We don't disable caching altogether as it helps unnecessary metadata calls to Azure from fastpath.
	//
	if config.IsSet(compName+".entry-expiration-sec") || config.IsSet("lfuse.entry-expiration-sec") {
		lf.entryExpiration = opt.EntryExpiration
	} else {
		if common.IsDistributedCacheEnabled {
			lf.entryExpiration = 3
		} else {
			lf.entryExpiration = defaultEntryExpiration
		}
	}

	if config.IsSet(compName+".attribute-expiration-sec") || config.IsSet("lfuse.attribute-expiration-sec") {
		lf.attributeExpiration = opt.AttributeExpiration
	} else {
		if common.IsDistributedCacheEnabled {
			lf.attributeExpiration = 3
		} else {
			lf.attributeExpiration = defaultAttrExpiration
		}
	}

	if config.IsSet(compName+".negative-entry-expiration-sec") || config.IsSet("lfuse.negative-entry-expiration-sec") {
		lf.negativeTimeout = opt.NegativeEntryExpiration
	} else {
		lf.negativeTimeout = defaultNegativeEntryExpiration
	}

	//
	// fuse_invalidate_path() cannot invalidate negative entries, as libfuse needs an inode number to
	// invalidate and for non-existent paths we cannot have an inode number. Ask kernel not to cache
	// negative entries.
	//
	if common.IsDistributedCacheEnabled {
		lf.negativeTimeout = 0
		log.Crit("Libfuse::Validate : DistributedCache enabled, forcing negative_timeout to 0")
	}

	// See comment in libfuse_init() why we should not force this.
	/*
		// Distributed Cache always runs in the directIO mode.
		if common.IsDistributedCacheEnabled {
			lf.directIO = true
		}
	*/

	if lf.directIO {
		lf.negativeTimeout = 0
		lf.attributeExpiration = 0
		lf.entryExpiration = 0
		log.Crit("Libfuse::Validate : DirectIO enabled, setting fuse timeouts to 0")
	}

	if !(config.IsSet(compName+".uid") || config.IsSet(compName+".gid") ||
		config.IsSet("lfuse.uid") || config.IsSet("lfuse.gid")) {
		var err error
		lf.ownerUID, lf.ownerGID, err = common.GetCurrentUser()
		if err != nil {
			log.Err("Libfuse::Validate : config error [unable to obtain current user info]")
			return nil
		}
	}

	if config.IsSet(compName + ".max-fuse-threads") {
		lf.maxFuseThreads = opt.MaxFuseThreads
	} else {
		lf.maxFuseThreads = defaultMaxFuseThreads
	}

	log.Info("Libfuse::Validate : UID %v, GID %v", lf.ownerUID, lf.ownerGID)

	return nil
}

func (lf *Libfuse) GenConfig() string {
	log.Info("Libfuse::Configure : config generation started")

	// If DirectIO is enabled, override expiration values
	directIO := false
	_ = config.UnmarshalKey("direct-io", &directIO)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s:", lf.Name()))

	timeout := defaultEntryExpiration
	negativeTimeout := defaultNegativeEntryExpiration

	//
	// Reduce attribute cache timeout for distributed cache.
	// Also, negative_timeout MUST be set to 0 when distributed cache is enabled.
	// see Validate() for details.
	//
	if common.IsDistributedCacheEnabled {
		timeout = 3
		negativeTimeout = 0
	}

	if directIO {
		timeout = 0
		negativeTimeout = 0
		sb.WriteString("\n  direct-io: true")
	}

	sb.WriteString(fmt.Sprintf("\n  attribute-expiration-sec: %v", timeout))
	sb.WriteString(fmt.Sprintf("\n  entry-expiration-sec: %v", timeout))
	sb.WriteString(fmt.Sprintf("\n  negative-entry-expiration-sec: %v", negativeTimeout))

	return sb.String()
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
//
//	Return failure if any config is not valid to exit the process
func (lf *Libfuse) Configure(_ bool) error {
	log.Trace("Libfuse::Configure : %s", lf.Name())
	// >> If you do not need any config parameters remove below code and return nil
	conf := LibfuseOptions{IgnoreOpenFlags: true}
	err := config.UnmarshalKey(lf.Name(), &conf)
	if err != nil {
		log.Err("Libfuse::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [invalid config attributes]", lf.Name())
	}

	err = config.UnmarshalKey("lfuse", &conf)
	if err != nil {
		log.Err("Libfuse::Configure : config error [invalid config attributes: %s]", err.Error())
		return fmt.Errorf("config error in lfuse [invalid config attributes]")
	}
	// Extract values from 'conf' and store them as you wish here

	err = config.UnmarshalKey("mount-path", &conf.mountPath)
	if err != nil {
		log.Err("Libfuse::Configure : config error [unable to obtain mount-path]")
		return err
	}
	err = config.UnmarshalKey("read-only", &conf.readOnly)
	if err != nil {
		log.Err("Libfuse::Configure : config error [unable to obtain read-only]")
		return err
	}

	err = config.UnmarshalKey("allow-other", &conf.allowOther)
	if err != nil {
		log.Err("Libfuse::Configure : config error [unable to obtain allow-other]")
		return err
	}

	err = config.UnmarshalKey("allow-root", &conf.allowRoot)
	if err != nil {
		log.Err("Libfuse::Configure : config error [unable to obtain allow-root]")
		return err
	}

	err = config.UnmarshalKey("nonempty", &conf.nonEmptyMount)
	if err != nil {
		log.Err("Libfuse::Configure : config error [unable to obtain nonempty]")
		return err
	}

	_ = config.UnmarshalKey("disable-kernel-cache", &lf.disableKernelCache)

	err = lf.Validate(&conf)
	if err != nil {
		log.Err("Libfuse::Configure : config error [invalid config settings]")
		return fmt.Errorf("%s config error %s", lf.Name(), err.Error())
	}

	// Disable libfuse logs if the mount is not running in foreground.
	// Currently as of 01-05-2025, we emit the libfuse logs only to the stdout.
	if !common.ForegroundMount {
		if lf.traceEnable {
			lf.traceEnable = false
		}
	}

	log.Crit("Libfuse::Configure : read-only %t, allow-other %t, allow-root %t, default-perm %d, entry-timeout %d, attr-time %d, negative-timeout %d, ignore-open-flags %t, nonempty %t, direct_io %t, max-fuse-threads %d, fuse-trace %t, extension %s, disable-writeback-cache %t, dirPermission %v, mountPath %v, umask %v, disableKernelCache %v",
		lf.readOnly, lf.allowOther, lf.allowRoot, lf.filePermission, lf.entryExpiration, lf.attributeExpiration, lf.negativeTimeout, lf.ignoreOpenFlags, lf.nonEmptyMount, lf.directIO, lf.maxFuseThreads, lf.traceEnable, lf.extensionPath, lf.disableWritebackCache, lf.dirPermission, lf.mountPath, lf.umask, lf.disableKernelCache)

	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
// << DO NOT DELETE ANY AUTO GENERATED CODE HERE >>
func NewLibfuseComponent() internal.Component {
	comp := &Libfuse{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewLibfuseComponent)

	attrTimeoutFlag := config.AddUint32Flag("attr-timeout", 0, " The attribute timeout in seconds")
	config.BindPFlag(compName+".attribute-expiration-sec", attrTimeoutFlag)

	entryTimeoutFlag := config.AddUint32Flag("entry-timeout", 0, "The entry timeout in seconds.")
	config.BindPFlag(compName+".entry-expiration-sec", entryTimeoutFlag)

	negativeTimeoutFlag := config.AddUint32Flag("negative-timeout", 0, "The negative entry timeout in seconds.")
	config.BindPFlag(compName+".negative-entry-expiration-sec", negativeTimeoutFlag)

	allowOther := config.AddBoolFlag("allow-other", false, "Allow other users to access this mount point.")
	config.BindPFlag("allow-other", allowOther)

	disableWritebackCache := config.AddBoolFlag("disable-writeback-cache", false, "Disallow libfuse to buffer write requests if you must strictly open files in O_WRONLY or O_APPEND mode.")
	config.BindPFlag(compName+".disable-writeback-cache", disableWritebackCache)

	debug := config.AddBoolPFlag("d", false, "Mount with foreground and FUSE logs on.")
	config.BindPFlag(compName+".fuse-trace", debug)
	debug.Hidden = true

	ignoreOpenFlags := config.AddBoolFlag("ignore-open-flags", true, "Ignore unsupported open flags (APPEND, WRONLY) by blobfuse when writeback caching is enabled.")
	config.BindPFlag(compName+".ignore-open-flags", ignoreOpenFlags)
}
