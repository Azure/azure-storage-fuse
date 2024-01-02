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

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/Azure/azure-storage-azcopy/v10/azbfs"
	"github.com/Azure/azure-storage-azcopy/v10/ste"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
)

//    ----------- Helper to create pipeline options ---------------

var UserAgent = func() string {
	return "Azure-Storage-Fuse/" + common.Blobfuse2Version
}

const (
	Timeout                time.Duration = 30 * time.Second
	KeepAlive              time.Duration = 30 * time.Second
	DualStack              bool          = true
	MaxIdleConns           int           = 0 // No limit
	MaxIdleConnsPerHost    int           = 100
	IdleConnTimeout        time.Duration = 90 * time.Second
	TLSHandshakeTimeout    time.Duration = 10 * time.Second
	ExpectContinueTimeout  time.Duration = 1 * time.Second
	DisableKeepAlives      bool          = false
	DisableCompression     bool          = false
	MaxResponseHeaderBytes int64         = 0
)

// getAzBlobPipelineOptions : Create pipeline options based on the config
func getAzBlobPipelineOptions(conf AzStorageConfig) (azblob.PipelineOptions, ste.XferRetryOptions) {
	retryOptions := ste.XferRetryOptions{
		Policy:        ste.RetryPolicyExponential,                      // Use exponential backoff as opposed to linear
		MaxTries:      conf.maxRetries,                                 // Try at most 3 times to perform the operation (set to 1 to disable retries)
		TryTimeout:    time.Second * time.Duration(conf.maxTimeout),    // Maximum time allowed for any single try
		RetryDelay:    time.Second * time.Duration(conf.backoffTime),   // Backoff amount for each retry (exponential or linear)
		MaxRetryDelay: time.Second * time.Duration(conf.maxRetryDelay), // Max delay between retries
	}

	telemetryValue := conf.telemetry
	if telemetryValue != "" {
		telemetryValue += " "
	}

	telemetryValue += UserAgent() + " (" + common.GetCurrentDistro() + ")"

	telemetryOptions := azblob.TelemetryOptions{
		Value: telemetryValue,
	}

	sysLogDisabled := log.GetType() == "silent" // If logging is enabled, allow the SDK to log retries to syslog.
	requestLogOptions := azblob.RequestLogOptions{
		// TODO: We can potentially consider making LogWarningIfTryOverThreshold a user settable option. For now lets use the default
		SyslogDisabled: sysLogDisabled,
	}
	logOptions := getLogOptions(conf.sdkTrace)
	// Create custom HTTPClient to pass to the factory in order to set our proxy
	var pipelineHTTPClient = newBlobfuse2HttpClient(conf)
	return azblob.PipelineOptions{
			Log:        logOptions,
			RequestLog: requestLogOptions,
			Telemetry:  telemetryOptions,
			HTTPSender: newBlobfuse2HTTPClientFactory(pipelineHTTPClient),
		},
		// Set RetryOptions to control how HTTP request are retried when retryable failures occur
		retryOptions
}

// getAzBfsPipelineOptions : Create pipeline options based on the config
func getAzBfsPipelineOptions(conf AzStorageConfig) (azbfs.PipelineOptions, ste.XferRetryOptions) {
	retryOptions := ste.XferRetryOptions{
		Policy:        ste.RetryPolicyExponential,                      // Use exponential backoff as opposed to linear
		MaxTries:      conf.maxRetries,                                 // Try at most 3 times to perform the operation (set to 1 to disable retries)
		TryTimeout:    time.Second * time.Duration(conf.maxTimeout),    // Maximum time allowed for any single try
		RetryDelay:    time.Second * time.Duration(conf.backoffTime),   // Backoff amount for each retry (exponential or linear)
		MaxRetryDelay: time.Second * time.Duration(conf.maxRetryDelay), // Max delay between retries
	}

	telemetryValue := conf.telemetry
	if telemetryValue != "" {
		telemetryValue += " "
	}

	telemetryValue += UserAgent() + " (" + common.GetCurrentDistro() + ")"
	telemetryOptions := azbfs.TelemetryOptions{
		Value: telemetryValue,
	}

	sysLogDisabled := log.GetType() == "silent" // If logging is enabled, allow the SDK to log retries to syslog.
	requestLogOptions := azbfs.RequestLogOptions{
		// TODO: We can potentially consider making LogWarningIfTryOverThreshold a user settable option. For now lets use the default
		SyslogDisabled: sysLogDisabled,
	}
	logOptions := getLogOptions(conf.sdkTrace)
	// Create custom HTTPClient to pass to the factory in order to set our proxy
	var pipelineHTTPClient = newBlobfuse2HttpClient(conf)
	return azbfs.PipelineOptions{
			Log:        logOptions,
			RequestLog: requestLogOptions,
			Telemetry:  telemetryOptions,
			HTTPSender: newBlobfuse2HTTPClientFactory(pipelineHTTPClient),
		},
		// Set RetryOptions to control how HTTP request are retried when retryable failures occur
		retryOptions
}

// Create an HTTP Client with configured proxy
// TODO: More configurations for other http client parameters?
func newBlobfuse2HttpClient(conf AzStorageConfig) *http.Client {
	var ProxyURL func(req *http.Request) (*url.URL, error) = func(req *http.Request) (*url.URL, error) {
		// If a proxy address is passed return
		var proxyURL url.URL = url.URL{
			Host: conf.proxyAddress,
		}
		return &proxyURL, nil
	}

	if conf.proxyAddress == "" {
		ProxyURL = nil
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy: ProxyURL,
			// We use Dial instead of DialContext as DialContext has been reported to cause slower performance.
			Dial /*Context*/ : (&net.Dialer{
				Timeout:   Timeout,
				KeepAlive: KeepAlive,
				DualStack: DualStack,
			}).Dial, /*Context*/
			MaxIdleConns:          MaxIdleConns, // No limit
			MaxIdleConnsPerHost:   MaxIdleConnsPerHost,
			IdleConnTimeout:       IdleConnTimeout,
			TLSHandshakeTimeout:   TLSHandshakeTimeout,
			ExpectContinueTimeout: ExpectContinueTimeout,
			DisableKeepAlives:     DisableKeepAlives,
			// if content-encoding is set in blob then having transport layer compression will
			// make things ugly and hence user needs to disable this feature through config
			DisableCompression:     conf.disableCompression,
			MaxResponseHeaderBytes: MaxResponseHeaderBytes,
			//ResponseHeaderTimeout:  time.Duration{},
			//ExpectContinueTimeout:  time.Duration{},
		},
	}
}

// newBlobfuse2HTTPClientFactory creates a custom HTTPClientPolicyFactory object that sends HTTP requests to the http client.
func newBlobfuse2HTTPClientFactory(pipelineHTTPClient *http.Client) pipeline.Factory {
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		return func(ctx context.Context, request pipeline.Request) (pipeline.Response, error) {
			r, err := pipelineHTTPClient.Do(request.WithContext(ctx))
			if err != nil {
				log.Err("BlockBlob::newBlobfuse2HTTPClientFactory : HTTP request failed [%s]", err.Error())
				err = pipeline.NewError(err, "HTTP request failed")
			}
			return pipeline.NewHTTPResponse(r), err
		}
	})
}

func getLogOptions(sdkLogging bool) pipeline.LogOptions {
	return pipeline.LogOptions{
		Log: func(logLevel pipeline.LogLevel, message string) {
			if !sdkLogging {
				return
			}

			// message here is a log generated by SDK and it contains URLs as well
			// These URLs have '%' as part of their data like / replaced with %2F
			// If we pass down message as first argument to our logging api, it assumes it to be
			// a format specifier and treat each % in URL to be a type specifier this will
			// result into log strings saying we have given %d but no integer as argument.
			// Only way to bypass this is to pass message as a second argument to logging method
			// so that logging api does not treat it as format string.
			switch logLevel {
			case pipeline.LogFatal:
				log.Crit("SDK : %s", message)
			case pipeline.LogPanic:
				log.Crit("SDK : %s", message)
			case pipeline.LogError:
				log.Err("SDK : %s", message)
			case pipeline.LogWarning:
				log.Warn("SDK : %s", message)
			case pipeline.LogInfo:
				log.Info("SDK : %s", message)
			case pipeline.LogDebug:
				log.Debug("SDK : %s", message)
			case pipeline.LogNone:
			default:
			}
		},
		ShouldLog: func(level pipeline.LogLevel) bool {
			if !sdkLogging {
				return false
			}
			currentLogLevel := func(commonLog common.LogLevel) pipeline.LogLevel {
				switch commonLog {
				case common.ELogLevel.INVALID():
					return pipeline.LogNone
				case common.ELogLevel.LOG_OFF():
					return pipeline.LogNone
				case common.ELogLevel.LOG_CRIT():
					return pipeline.LogPanic // Panic logs both Panic and Fatal
				case common.ELogLevel.LOG_ERR():
					return pipeline.LogError
				case common.ELogLevel.LOG_WARNING():
					return pipeline.LogWarning
				case common.ELogLevel.LOG_INFO():
					return pipeline.LogInfo
				case common.ELogLevel.LOG_TRACE():
					return pipeline.LogDebug // No Trace in pipeline.LogLevel
				case common.ELogLevel.LOG_DEBUG():
					return pipeline.LogDebug
				}
				return pipeline.LogNone
			}(log.GetLogLevel())
			return level <= currentLogLevel
		},
	}
}

// ----------- Store error code handling ---------------
const (
	ErrNoErr uint16 = iota
	ErrUnknown
	ErrFileNotFound
	ErrFileAlreadyExists
	InvalidRange
	BlobIsUnderLease
	InvalidPermission
)

// ErrStr : Store error to string mapping
var ErrStr = map[uint16]string{
	ErrNoErr:             "No Error found",
	ErrUnknown:           "Unknown store error",
	ErrFileNotFound:      "Blob not found",
	ErrFileAlreadyExists: "Blob already exists",
}

// For detailed error list refert ServiceCodeType at below link
// https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob#ListBlobsSegmentOptions
// Convert blob storage error to common errors
func storeBlobErrToErr(err error) uint16 {
	if serr, ok := err.(azblob.StorageError); ok {
		switch serr.ServiceCode() {
		case azblob.ServiceCodeBlobAlreadyExists:
			return ErrFileAlreadyExists
		case azblob.ServiceCodeBlobNotFound:
			return ErrFileNotFound
		case azblob.ServiceCodeInvalidRange:
			return InvalidRange
		case azblob.ServiceCodeLeaseIDMissing:
			return BlobIsUnderLease
		case azblob.ServiceCodeInsufficientAccountPermissions:
			return InvalidPermission
		case "AuthorizationPermissionMismatch":
			return InvalidPermission
		default:
			return ErrUnknown
		}
	}
	return ErrNoErr
}

// Convert datalake storage error to common errors
func storeDatalakeErrToErr(err error) uint16 {
	if serr, ok := err.(azbfs.StorageError); ok {
		switch serr.ServiceCode() {
		case azbfs.ServiceCodePathAlreadyExists:
			return ErrFileAlreadyExists
		case azbfs.ServiceCodePathNotFound:
			return ErrFileNotFound
		case azbfs.ServiceCodeSourcePathNotFound:
			return ErrFileNotFound
		case "LeaseIdMissing":
			return BlobIsUnderLease
		case "AuthorizationPermissionMismatch":
			return InvalidPermission
		default:
			return ErrUnknown
		}
	}
	return ErrNoErr
}

//	----------- Metadata handling  ---------------
//
// Converts datalake properties to a metadata map
func newMetadata(properties string) map[string]string {
	metadata := make(map[string]string)
	if properties != "" {
		// Create a map of the properties (metadata)
		pairs := strings.Split(properties, ",")

		for _, p := range pairs {
			components := strings.SplitN(p, "=", 2)
			key := components[0]
			value, err := base64.StdEncoding.DecodeString(components[1])
			if err == nil {
				metadata[key] = string(value)
			}
		}
	}
	return metadata
}

// parseProperties : Parse the properties of a given datalake path and populate its attributes
func parseProperties(attr *internal.ObjAttr, properties string) {
	metadata := newMetadata(properties)
	// Parse the metadata
	parseMetadata(attr, metadata)
}

// parseMetadata : Parse the metadata of a given path and populate its attributes
func parseMetadata(attr *internal.ObjAttr, metadata map[string]string) {
	// Save the metadata in attributes so that later if someone wants to add anything it can work
	attr.Metadata = metadata
	for k, v := range metadata {
		if strings.ToLower(k) == folderKey && v == "true" {
			attr.Flags = internal.NewDirBitMap()
			attr.Mode = attr.Mode | os.ModeDir
		} else if strings.ToLower(k) == symlinkKey && v == "true" {
			attr.Flags = internal.NewSymlinkBitMap()
			attr.Mode = attr.Mode | os.ModeSymlink
		}
	}
}

//    ----------- Content-type handling  ---------------

// ContentTypeMap : Store file extension to content-type mapping
var ContentTypes = map[string]string{
	".css":  "text/css",
	".pdf":  "application/pdf",
	".xml":  "text/xml",
	".csv":  "text/csv",
	".json": "application/json",
	".rtf":  "application/rtf",
	".txt":  "text/plain",
	".java": "text/plain",
	".dat":  "text/plain",

	".htm":  "text/html",
	".html": "text/html",

	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".bmp":  "image/bmp",

	".js":   "application/javascript",
	".mjs":  "application/javascript",
	".svg":  "image/svg+xml",
	".wasm": "application/wasm",
	".webp": "image/webp",

	".wav":  "audio/wav",
	".mp3":  "audio/mpeg",
	".mpeg": "video/mpeg",
	".aac":  "audio/aac",
	".avi":  "video/x-msvideo",
	".m3u8": "application/x-mpegURL",
	".ts":   "video/MP2T",
	".mid":  "audio/midiaudio/x-midi",
	".3gp":  "video/3gpp",
	".mp4":  "video/mp4",

	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",

	".gz":   "application/x-gzip",
	".jar":  "application/java-archive",
	".rar":  "application/vnd.rar",
	".tar":  "application/x-tar",
	".zip":  "application/x-zip-compressed",
	".7z":   "application/x-7z-compressed",
	".3g2":  "video/3gpp2",
	".usdz": "application/zip",

	".sh":  "application/x-sh",
	".exe": "application/x-msdownload",
	".dll": "application/x-msdownload",
}

// getContentType : Based on the file extension retrieve the content type to be set
func getContentType(key string) string {
	value, found := ContentTypes[strings.ToLower(filepath.Ext(key))]
	if found {
		return value
	}
	return "application/octet-stream"
}

func populateContentType(newSet string) error { //nolint
	var data map[string]string
	if err := json.Unmarshal([]byte(newSet), &data); err != nil {
		log.Err("Failed to parse config file : %s [%s]", newSet, err.Error())
		return err
	}

	// We can simply append the new data to end of the map
	// however there may be conflicting keys and hence we need to merge manually
	//ContentTypeMap = append(ContentTypeMap, data)
	for k, v := range data {
		ContentTypes[k] = v
	}
	return nil
}

//	----------- Blob access tier type conversion  ---------------
//
// AccessTierMap : Store config to access tier mapping
var AccessTiers = map[string]azblob.AccessTierType{
	"none":    azblob.AccessTierNone,
	"hot":     azblob.AccessTierHot,
	"cool":    azblob.AccessTierCool,
	"archive": azblob.AccessTierArchive,
	"p4":      azblob.AccessTierP4,
	"p6":      azblob.AccessTierP6,
	"p10":     azblob.AccessTierP10,
	"p15":     azblob.AccessTierP15,
	"p20":     azblob.AccessTierP20,
	"p30":     azblob.AccessTierP30,
	"p40":     azblob.AccessTierP40,
	"p50":     azblob.AccessTierP50,
	"p60":     azblob.AccessTierP60,
	"p70":     azblob.AccessTierP70,
	"p80":     azblob.AccessTierP80,
}

func getAccessTierType(name string) azblob.AccessTierType {
	if name == "" {
		return azblob.AccessTierNone
	}

	value, found := AccessTiers[strings.ToLower(name)]
	if found {
		return value
	}
	return azblob.AccessTierNone
}

// Called by x method
func getACLPermissions(mode os.FileMode) string {
	// Format for ACL and Permission string is different
	// ACL:"user::rwx,user:<id>:rwx,group::rwx,mask::rwx,other::rwx"
	// Permissions:"rwxrwxrwx+"
	// If we call the set ACL without giving user then all other principals will be removed.
	var sb strings.Builder
	writePermission(&sb, mode&(1<<8) != 0, 'r')
	writePermission(&sb, mode&(1<<7) != 0, 'w')
	writePermission(&sb, mode&(1<<6) != 0, 'x')
	writePermission(&sb, mode&(1<<5) != 0, 'r')
	writePermission(&sb, mode&(1<<4) != 0, 'w')
	writePermission(&sb, mode&(1<<3) != 0, 'x')
	writePermission(&sb, mode&(1<<2) != 0, 'r')
	writePermission(&sb, mode&(1<<1) != 0, 'w')
	writePermission(&sb, mode&(1<<0) != 0, 'x')
	return sb.String()
}

func writePermission(sb *strings.Builder, permitted bool, permission rune) {
	if permitted {
		sb.WriteRune(permission)
	} else {
		sb.WriteRune('-')
	}
}

// Called by x method
// How to interpret the mask and name user acl : https://learn.microsoft.com/en-us/azure/storage/blobs/data-lake-storage-access-control
func getFileModeFromACL(objid string, acl string, owner string) (os.FileMode, error) {
	var mode os.FileMode = 0
	if acl == "" {
		return mode, fmt.Errorf("empty permissions from the service")
	}

	extractPermission := func(acl string, key string) string {
		idx := strings.Index(acl, key) + len(key)
		return acl[idx : idx+3]
	}

	extractNamedUserACL := func(acl string, objid string) string {
		key := fmt.Sprintf("user:%s:", objid)
		idx := strings.Index(acl, key) + len(key)
		if idx == -1 {
			return "---"
		}

		userACL := acl[idx : idx+3]
		mask := extractPermission(acl, "mask::")

		permissions := ""
		for i, c := range userACL {
			if userACL[i] == mask[i] {
				permissions += string(c)
			} else {
				permissions += "-"
			}
		}

		return permissions
	}

	// Sample string : user::rwx,user:objid1:r--,user:objid2:r--,group::r--,mask::r-x,other::rwx:
	permissions := ""
	if owner == objid {
		// Owner of this blob is the authenticated object id so extract the user permissions from the ACL directly
		permissions = extractPermission(acl, "user::")
	} else {
		// Owner of this blob is not the authenticated object id, search object id exists in the ACL
		permissions = extractNamedUserACL(acl, objid)
	}

	permissions += extractPermission(acl, "group::")
	permissions += extractPermission(acl, "other::")

	return getFileMode(permissions)
}

// Called by x method
func getFileMode(permissions string) (os.FileMode, error) {
	var mode os.FileMode = 0
	if permissions == "" {
		return mode, nil
	}

	// Expect service to return a 9 char string with r, w, x, or -
	const rwx = "rwxrwxrwx"
	if len(rwx) > len(permissions) {
		log.Err("utils::getFileMode : Unexpected length of permissions from the service %d: %s", len(permissions), permissions)
		return 0, fmt.Errorf("unexpected length of permissions from the service %d: %s", len(permissions), permissions)
	} else if len(rwx) < len(permissions) {
		log.Debug("utils::getFileMode : Unexpected permissions from the service: %s", permissions)
	}

	for i, c := range rwx {
		if permissions[i] == byte(c) {
			mode |= 1 << uint(9-1-i)
		} else if permissions[i] != byte('-') {
			log.Debug("utils::getFileMode : Unexpected permissions from the service at character %d: %s", i, permissions)
		}
	}
	return mode, nil
}

// Strips the prefixPath from the path and returns the joined string
func split(prefixPath string, path string) string {
	if prefixPath == "" {
		return path
	}

	// Remove prefixpath from the given path
	paths := strings.Split(path, prefixPath)
	if paths[0] == "" {
		paths = paths[1:]
	}

	// If result starts with "/" then remove that
	if paths[0][0] == '/' {
		paths[0] = paths[0][1:]
	}

	return filepath.Join(paths...)
}

func sanitizeSASKey(key string) string {
	if key == "" {
		return key
	}

	if key[0] != '?' {
		return ("?" + key)
	}

	return key
}

func getMD5(fi *os.File) ([]byte, error) {
	hasher := md5.New()
	_, err := io.Copy(hasher, fi)

	if err != nil {
		return nil, errors.New("failed to generate md5")
	}

	return hasher.Sum(nil), nil
}

func autoDetectAuthMode(opt AzStorageOptions) string {
	if opt.ApplicationID != "" || opt.ResourceID != "" || opt.ObjectID != "" {
		return "msi"
	} else if opt.AccountKey != "" {
		return "key"
	} else if opt.SaSKey != "" {
		return "sas"
	} else if opt.ClientID != "" || opt.ClientSecret != "" || opt.TenantID != "" {
		return "spn"
	}

	return "msi"
}

func removeLeadingSlashes(s string) string {
	for strings.HasPrefix(s, "/") {
		s = strings.TrimLeft(s, "/")
	}
	return s
}
