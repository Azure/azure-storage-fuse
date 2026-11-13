// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

const (
	createDir    = "CreateDir"
	deleteDir    = "DeleteDir"
	createFile   = "CreateFile"
	truncateFile = "TruncateFile"
	deleteFile   = "DeleteFile"
	renameDir    = "RenameDir"
	renameFile   = "RenameFile"
	createLink   = "CreateLink"
	readLink     = "ReadLink"
	syncFile     = "SyncFile"
	syncDir      = "SyncDir"
	chmod        = "Chmod"

	openHandles = "OpenFileHandles"
	md          = "Mode"
	size        = "Size"
	source      = "Src"
	dest        = "Dest"
	trgt        = "Target"
)
