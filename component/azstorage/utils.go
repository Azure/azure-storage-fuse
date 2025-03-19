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
	"crypto/md5"
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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	azlog "github.com/Azure/azure-sdk-for-go/sdk/azcore/log"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	serviceBfs "github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/service"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azdatalake/datalakeerror"
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
	MaxIdleConnsPerHost    int           = 200
	MaxConnsPerHost        int           = 300
	IdleConnTimeout        time.Duration = 90 * time.Second
	TLSHandshakeTimeout    time.Duration = 10 * time.Second
	ExpectContinueTimeout  time.Duration = 1 * time.Second
	DisableKeepAlives      bool          = false
	DisableCompression     bool          = false
	MaxResponseHeaderBytes int64         = 0
)

// getAzStorageClientOptions : Create client options based on the config
func getAzStorageClientOptions(conf *AzStorageConfig) (azcore.ClientOptions, error) {
	retryOptions := policy.RetryOptions{
		MaxRetries:    conf.maxRetries,                                 // Try at most 3 times to perform the operation (set to 1 to disable retries)
		TryTimeout:    time.Second * time.Duration(conf.maxTimeout),    // Maximum time allowed for any single try
		RetryDelay:    time.Second * time.Duration(conf.backoffTime),   // Backoff amount for each retry (exponential or linear)
		MaxRetryDelay: time.Second * time.Duration(conf.maxRetryDelay), // Max delay between retries
	}

	telemetryValue := conf.telemetry
	if telemetryValue != "" {
		telemetryValue += " "
	}
	telemetryValue += UserAgent() + " (" + common.GetCurrentDistro() + ")"
	telemetryPolicy := newBlobfuseTelemetryPolicy(telemetryValue)

	logOptions := getSDKLogOptions()

	transportOptions, err := newBlobfuse2HttpClient(conf)
	if err != nil {
		log.Err("utils::getAzStorageClientOptions : Failed to create transport client [%s]", err.Error())
	}

	return azcore.ClientOptions{
		Retry:           retryOptions,
		Logging:         logOptions,
		PerCallPolicies: []policy.Policy{telemetryPolicy},
		Transport:       transportOptions,
	}, err
}

// getAzBlobServiceClientOptions : Create azblob service client options based on the config
func getAzBlobServiceClientOptions(conf *AzStorageConfig) (*service.ClientOptions, error) {
	opts, err := getAzStorageClientOptions(conf)
	return &service.ClientOptions{
		ClientOptions: opts,
	}, err
}

// getAzDatalakeServiceClientOptions : Create azdatalake service client options based on the config
func getAzDatalakeServiceClientOptions(conf *AzStorageConfig) (*serviceBfs.ClientOptions, error) {
	opts, err := getAzStorageClientOptions(conf)
	return &serviceBfs.ClientOptions{
		ClientOptions: opts,
	}, err
}

// getLogOptions : to configure the SDK logging policy
func getSDKLogOptions() policy.LogOptions {
	if log.GetType() == "silent" || log.GetLogLevel() < common.ELogLevel.LOG_DEBUG() {
		return policy.LogOptions{}
	} else {
		// add headers and query params which should be logged and not redacted
		return policy.LogOptions{
			AllowedHeaders:     allowedHeaders,
			AllowedQueryParams: allowedQueryParams,
		}
	}
}

// setSDKLogListener : log the requests and responses.
// It is disabled if,
//   - logging type is silent
//   - logging level is less than debug
func setSDKLogListener() {
	if log.GetType() == "silent" || log.GetLogLevel() < common.ELogLevel.LOG_DEBUG() {
		// reset listener
		azlog.SetListener(nil)
	} else {
		azlog.SetListener(func(cls azlog.Event, msg string) {
			log.Debug("SDK(%s) : %s", cls, msg)
		})
	}
}

// Create an HTTP Client with configured proxy
func newBlobfuse2HttpClient(conf *AzStorageConfig) (*http.Client, error) {
	var ProxyURL func(req *http.Request) (*url.URL, error)
	if conf.proxyAddress == "" {
		ProxyURL = http.ProxyFromEnvironment
	} else {
		u, err := url.Parse(conf.proxyAddress)
		if err != nil {
			log.Err("utils::newBlobfuse2HttpClient : Failed to parse proxy : %s [%s]", conf.proxyAddress, err.Error())
			return nil, err
		}
		ProxyURL = http.ProxyURL(u)
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
			MaxConnsPerHost:       MaxConnsPerHost,
			IdleConnTimeout:       IdleConnTimeout,
			TLSHandshakeTimeout:   TLSHandshakeTimeout,
			ExpectContinueTimeout: ExpectContinueTimeout,
			DisableKeepAlives:     DisableKeepAlives,
			// if content-encoding is set in blob then having transport layer compression will
			// make things ugly and hence user needs to disable this feature through config
			DisableCompression:     conf.disableCompression,
			MaxResponseHeaderBytes: MaxResponseHeaderBytes,
		},
	}, nil
}

// getCloudConfiguration : returns cloud configuration type on the basis of endpoint
func getCloudConfiguration(endpoint string) cloud.Configuration {
	if strings.Contains(endpoint, "core.chinacloudapi.cn") {
		return cloud.AzureChina
	} else if strings.Contains(endpoint, "core.usgovcloudapi.net") {
		return cloud.AzureGovernment
	} else {
		return cloud.AzurePublic
	}
}

// blobfuseTelemetryPolicy is a custom pipeline policy to prepend the blobfuse user agent string to the one coming from SDK.
// This is added in the PerCallPolicies which executes after the SDK's default telemetry policy.
type blobfuseTelemetryPolicy struct {
	telemetryValue string
}

// newBlobfuseTelemetryPolicy creates an object which prepends the blobfuse user agent string to the User-Agent request header
func newBlobfuseTelemetryPolicy(telemetryValue string) policy.Policy {
	return &blobfuseTelemetryPolicy{telemetryValue: telemetryValue}
}

func (p blobfuseTelemetryPolicy) Do(req *policy.Request) (*http.Response, error) {
	userAgent := p.telemetryValue

	// prepend the blobfuse user agent string
	if ua := req.Raw().Header.Get(common.UserAgentHeader); ua != "" {
		userAgent = fmt.Sprintf("%s %s", userAgent, ua)
	}
	req.Raw().Header.Set(common.UserAgentHeader, userAgent)
	return req.Next()
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

// For detailed error list refer below link,
// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/storage/azblob/bloberror/error_codes.go
// Convert blob storage error to common errors
func storeBlobErrToErr(err error) uint16 {
	var respErr *azcore.ResponseError
	errors.As(err, &respErr)

	if respErr != nil {
		switch (bloberror.Code)(respErr.ErrorCode) {
		case bloberror.BlobAlreadyExists:
			return ErrFileAlreadyExists
		case bloberror.BlobNotFound:
			return ErrFileNotFound
		case bloberror.InvalidRange:
			return InvalidRange
		case bloberror.LeaseIDMissing:
			return BlobIsUnderLease
		case bloberror.InsufficientAccountPermissions, bloberror.AuthorizationPermissionMismatch:
			return InvalidPermission
		default:
			return ErrUnknown
		}
	}
	return ErrNoErr
}

// Convert datalake storage error to common errors
func storeDatalakeErrToErr(err error) uint16 {
	var respErr *azcore.ResponseError
	errors.As(err, &respErr)

	if respErr != nil {
		switch (datalakeerror.StorageErrorCode)(respErr.ErrorCode) {
		case datalakeerror.PathAlreadyExists:
			return ErrFileAlreadyExists
		case datalakeerror.PathNotFound:
			return ErrFileNotFound
		case datalakeerror.SourcePathNotFound:
			return ErrFileNotFound
		case datalakeerror.LeaseIDMissing:
			return BlobIsUnderLease
		case datalakeerror.AuthorizationPermissionMismatch:
			return InvalidPermission
		default:
			return ErrUnknown
		}
	}
	return ErrNoErr
}

//	----------- Metadata handling  ---------------
//
// parseMetadata : Parse the metadata of a given path and populate its attributes
func parseMetadata(attr *internal.ObjAttr, metadata map[string]*string) {
	// Save the metadata in attributes so that later if someone wants to add anything it can work
	attr.Metadata = metadata
	for k, v := range metadata {
		if v != nil {
			if strings.ToLower(k) == folderKey && *v == "true" {
				attr.Flags = internal.NewDirBitMap()
				attr.Mode = attr.Mode | os.ModeDir
			} else if strings.ToLower(k) == symlinkKey && *v == "true" {
				attr.Flags = internal.NewSymlinkBitMap()
				attr.Mode = attr.Mode | os.ModeSymlink
			}
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
var AccessTiers = map[string]blob.AccessTier{
	"hot":     blob.AccessTierHot,
	"cool":    blob.AccessTierCool,
	"cold":    blob.AccessTierCold,
	"archive": blob.AccessTierArchive,
	"p4":      blob.AccessTierP4,
	"p6":      blob.AccessTierP6,
	"p10":     blob.AccessTierP10,
	"p15":     blob.AccessTierP15,
	"p20":     blob.AccessTierP20,
	"p30":     blob.AccessTierP30,
	"p40":     blob.AccessTierP40,
	"p50":     blob.AccessTierP50,
	"p60":     blob.AccessTierP60,
	"p70":     blob.AccessTierP70,
	"p80":     blob.AccessTierP80,
	"premium": blob.AccessTierPremium,
}

func getAccessTierType(name string) *blob.AccessTier {
	if name == "" {
		return nil
	}

	value, found := AccessTiers[strings.ToLower(name)]
	if found {
		return &value
	}
	return nil
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

// removePrefixPath removes the given prefixPath from the beginning of path,
// if it exists, and returns the resulting string without leading slashes.
func removePrefixPath(prefixPath, path string) string {
	if prefixPath == "" {
		return path
	}
	path = strings.TrimPrefix(path, prefixPath)
	if path[0] == '/' {
		return path[1:]
	}
	return path
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

func modifyLMTandEtag(attr *internal.ObjAttr, lmt *time.Time, ETag string) {
	if attr != nil {
		attr.Atime = *lmt
		attr.Mtime = *lmt
		attr.Ctime = *lmt
		attr.ETag = ETag
	}
}

func sanitizeEtag(ETag *azcore.ETag) string {
	if ETag != nil {
		return strings.Trim(string(*ETag), `"`)
	}
	return ""
}

// func parseBlobTags(tags *container.BlobTags) map[string]string {

// 	if tags == nil {
// 		return nil
// 	}

// 	blobtags := make(map[string]string)
// 	for _, tag := range tags.BlobTagSet {
// 		if tag != nil {
// 			if tag.Key != nil {
// 				blobtags[*tag.Key] = *tag.Value
// 			}
// 		}
// 	}

// 	return blobtags
// }
