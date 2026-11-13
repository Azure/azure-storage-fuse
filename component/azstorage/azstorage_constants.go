// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azstorage

const (
	bytesDownloaded  = "Bytes Downloaded"
	bytesUploaded    = "Bytes Uploaded"
	downloadProgress = "DownloadProgress"
	uploadProgress   = "UploadProgress"
	bytesTfrd        = "Bytes Transferred"

	createDir    = "CreateDir"
	deleteDir    = "DeleteDir"
	streamDir    = "StreamDir"
	renameDir    = "RenameDir"
	createFile   = "CreateFile"
	deleteFile   = "DeleteFile"
	renameFile   = "RenameFile"
	truncateFile = "TruncateFile"
	createLink   = "CreateLink"
	readLink     = "ReadLink"
	chmod        = "Chmod"

	openHandles = "OpenFileHandles"
	mode        = "Mode"
	count       = "Count"
	src         = "Src"
	dest        = "Dest"
	size        = "Size"
	target      = "Target"
)

// headers which should be logged and not redacted
var allowedHeaders []string = []string{
	"x-ms-version", "x-ms-date", "x-ms-range", "x-ms-delete-snapshots", "x-ms-delete-type-permanent", "x-ms-blob-content-type",
	"x-ms-blob-type", "x-ms-copy-source", "x-ms-copy-id", "x-ms-copy-status", "x-ms-access-tier", "x-ms-creation-time", "x-ms-copy-progress",
	"x-ms-access-tier-inferred", "x-ms-acl", "x-ms-group", "x-ms-lease-state", "x-ms-owner", "x-ms-permissions", "x-ms-resource-type", "x-ms-content-crc64",
	"x-ms-rename-source", "accept-ranges", "x-ms-continuation",
}

// query parameters which should be logged and not redacted
var allowedQueryParams []string = []string{
	"comp", "delimiter", "include", "marker", "maxresults", "prefix", "restype", "blockid", "blocklisttype",
	"directory", "recursive", "resource", "se", "sp", "spr", "srt", "ss", "st", "sv", "action", "continuation", "mode",
	"client_id", "authorization_endpoint",
}
