package external

import (
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/Azure/azure-storage-fuse/v2/internal/handlemap"
)

const (
	PropFlagUnknown uint16 = iota
	PropFlagNotExists
	PropFlagIsDir
	PropFlagEmptyDir
	PropFlagSymlink
	PropFlagMetadataRetrieved
	PropFlagModeDefault // TODO: Does this sound better as ModeDefault or DefaultMode? The getter would be IsModeDefault or IsDefaultMode
)

// // Type aliases for base component
type BaseComponent = internal.BaseComponent

// // Type aliases for component
type Component = internal.Component

type ComponentPriority = internal.ComponentPriority

// // Type aliases for attributes
type ObjAttr = internal.ObjAttr

// // Type aliases for component options
type CreateDirOptions = internal.CreateDirOptions
type DeleteDirOptions = internal.DeleteDirOptions
type IsDirEmptyOptions = internal.IsDirEmptyOptions
type OpenDirOptions = internal.OpenDirOptions
type ReadDirOptions = internal.ReadDirOptions
type StreamDirOptions = internal.StreamDirOptions
type CloseDirOptions = internal.CloseDirOptions
type RenameDirOptions = internal.RenameDirOptions
type CreateFileOptions = internal.CreateFileOptions
type DeleteFileOptions = internal.DeleteFileOptions
type OpenFileOptions = internal.OpenFileOptions
type CloseFileOptions = internal.CloseFileOptions
type RenameFileOptions = internal.RenameFileOptions
type ReadFileOptions = internal.ReadFileOptions
type ReadInBufferOptions = internal.ReadInBufferOptions
type WriteFileOptions = internal.WriteFileOptions
type GetFileBlockOffsetsOptions = internal.GetFileBlockOffsetsOptions
type TruncateFileOptions = internal.TruncateFileOptions
type CopyToFileOptions = internal.CopyToFileOptions
type CopyFromFileOptions = internal.CopyFromFileOptions
type FlushFileOptions = internal.FlushFileOptions
type SyncFileOptions = internal.SyncFileOptions
type SyncDirOptions = internal.SyncDirOptions
type ReleaseFileOptions = internal.ReleaseFileOptions
type UnlinkFileOptions = internal.UnlinkFileOptions
type CreateLinkOptions = internal.CreateLinkOptions
type ReadLinkOptions = internal.ReadLinkOptions
type GetAttrOptions = internal.GetAttrOptions
type SetAttrOptions = internal.SetAttrOptions
type ChmodOptions = internal.ChmodOptions
type ChownOptions = internal.ChownOptions
type StageDataOptions = internal.StageDataOptions
type CommitDataOptions = internal.CommitDataOptions
type CommittedBlock = internal.CommittedBlock
type CommittedBlockList = internal.CommittedBlockList

// // Type aliases for pipeline
type Handle = handlemap.Handle

// Wrapper function to expose AddComponent
func AddComponent(name string, init internal.NewComponent) {
	internal.AddComponent(name, init)
}

type ComponentPriorityWrapper struct {
	internal.ComponentPriority
}

// Wrapper functions to expose ComponentPriority methods
func (ComponentPriorityWrapper) LevelMid() ComponentPriority {
	return internal.ComponentPriority(0).LevelMid()
}

func (ComponentPriorityWrapper) Producer() ComponentPriority {
	return internal.ComponentPriority(0).Producer()
}

func (ComponentPriorityWrapper) Consumer() ComponentPriority {
	return internal.ComponentPriority(0).Consumer()
}

func (ComponentPriorityWrapper) LevelOne() ComponentPriority {
	return internal.ComponentPriority(0).LevelOne()
}

func (ComponentPriorityWrapper) LevelTwo() ComponentPriority {
	return internal.ComponentPriority(0).LevelTwo()
}

// wrapper utility functions to expose internal functions
func TruncateDirName(name string) string {
	return internal.TruncateDirName(name)
}

func ExtendDirName(name string) string {
	return internal.ExtendDirName(name)
}
