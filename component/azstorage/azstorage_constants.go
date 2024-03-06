/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
}
