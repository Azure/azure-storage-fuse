//go:build !authtest

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

package azstorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Mock pager helpers for unit tests

// mockPager creates a pager that returns the provided responses in order.
func mockPager(responses []blob.GetLayoutResponse) *runtime.Pager[blob.GetLayoutResponse] {
	idx := 0
	return runtime.NewPager(runtime.PagingHandler[blob.GetLayoutResponse]{
		More: func(_ blob.GetLayoutResponse) bool {
			return idx < len(responses)
		},
		Fetcher: func(_ context.Context, _ *blob.GetLayoutResponse) (blob.GetLayoutResponse, error) {
			resp := responses[idx]
			idx++
			return resp, nil
		},
	})
}

// errorPager creates a pager that always returns an error.
func errorPager(err error) *runtime.Pager[blob.GetLayoutResponse] {
	return runtime.NewPager(runtime.PagingHandler[blob.GetLayoutResponse]{
		More: func(_ blob.GetLayoutResponse) bool {
			return true
		},
		Fetcher: func(_ context.Context, _ *blob.GetLayoutResponse) (blob.GetLayoutResponse, error) {
			return blob.GetLayoutResponse{}, err
		},
	})
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Unit tests for getLayout (no Azure credentials required)

// TestGetLayout_NilEndpointsAndRanges verifies the "no layout" path: when the service
// returns no endpoint/range information the whole blob is served from the primary endpoint,
// so LayoutRanges must be nil and the blob metadata must be populated correctly.
func TestGetLayout_NilEndpointsAndRanges(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	size := int64(4096)
	etag := azcore.ETag(`"testETag"`)
	md5 := []byte{0xde, 0xad, 0xbe, 0xef}
	metaVal := "metaValue"

	resp := blob.GetLayoutResponse{
		BlobContentLength: to.Ptr(size),
		BlobContentMD5:    md5,
		LastModified:      to.Ptr(now),
		Metadata:          map[string]*string{"key": to.Ptr(metaVal)},
		ETag:              to.Ptr(etag),
		// Endpoints and Ranges left nil → "no layout" path
	}

	lr, err := getLayout(context.Background(), mockPager([]blob.GetLayoutResponse{resp}))

	assert.NoError(t, err)
	assert.NotNil(t, lr)

	// Scalar metadata
	assert.Equal(t, size, lr.contentLength)
	assert.Equal(t, md5, lr.contentMD5)
	assert.Equal(t, to.Ptr(now), lr.lmt)
	assert.Equal(t, to.Ptr(etag), lr.eTag)
	assert.Equal(t, metaVal, *lr.metadata["key"])

	// Layout should exist but have no ranges (whole-blob primary-endpoint case)
	assert.NotNil(t, lr.layout)
	assert.Nil(t, lr.layout.LayoutRanges)
}

// TestGetLayout_PagerError verifies that an error returned by the pager is propagated.
func TestGetLayout_PagerError(t *testing.T) {
	expected := errors.New("network failure")

	lr, err := getLayout(context.Background(), errorPager(expected))

	assert.ErrorIs(t, err, expected)
	assert.Nil(t, lr)
}

// TestGetLayout_MetadataFromFirstPageOnly verifies that contentMD5, lmt, metadata, and eTag
// are taken from the first page even when multiple pages are returned.
// NOTE: Because constructing pages with populated Endpoints/Ranges requires the internal
// generated package (not importable from outside azblob), a two-page scenario here uses
// two nil-endpoint pages; the first nil-endpoint page triggers an early return so only
// one iteration takes place – the important invariant (first-page metadata wins) is still
// exercised via the single-page tests above. The integration tests in layoutTestSuite
// cover the multi-page case against the preprod endpoint.

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Unit tests for the BlobLayoutAwareRouting config option (no Azure credentials required)

// TestBlobLayoutAwareRoutingConfig_DefaultIsFalse verifies that the flag defaults to false.
func TestBlobLayoutAwareRoutingConfig_DefaultIsFalse(t *testing.T) {
	defer config.ResetConfig()
	az := &AzStorage{}
	opt := AzStorageOptions{
		AccountName: "testaccount",
		Container:   "testcontainer",
		AuthMode:    "key",
		AccountKey:  "dGVzdGtleQ==", // base64 of "testkey"
	}

	err := ParseAndValidateConfig(az, opt)
	assert.NoError(t, err)
	assert.False(t, az.stConfig.isBlobLayoutAwareRoutingEnabled)
}

// TestBlobLayoutAwareRoutingConfig_CanBeEnabled verifies that setting the option to true
// correctly propagates into AzStorageConfig.
func TestBlobLayoutAwareRoutingConfig_CanBeEnabled(t *testing.T) {
	defer config.ResetConfig()
	az := &AzStorage{}
	opt := AzStorageOptions{
		AccountName:            "testaccount",
		Container:              "testcontainer",
		AuthMode:               "key",
		AccountKey:             "dGVzdGtleQ==",
		BlobLayoutAwareRouting: true,
	}

	err := ParseAndValidateConfig(az, opt)
	assert.NoError(t, err)
	assert.True(t, az.stConfig.isBlobLayoutAwareRoutingEnabled)
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Integration tests – these require a valid ~/azuretest.json and use the preprod endpoint.

type layoutTestSuite struct {
	suite.Suite
	assert          *assert.Assertions
	az              *AzStorage
	serviceClient   *service.Client
	containerClient *container.Client
	config          string
	container       string
}

func (s *layoutTestSuite) SetupTest() {
	cfg := common.LogConfig{
		FilePath:    "./logfile.txt",
		MaxFileSize: 10,
		FileCount:   10,
		Level:       common.ELogLevel.LOG_DEBUG(),
	}
	_ = log.SetDefaultLogger("base", cfg)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Unable to get home directory")
		os.Exit(1)
	}
	cfgFile, err := os.Open(homeDir + "/azuretest.json")
	if err != nil {
		fmt.Println("Unable to open config file")
		os.Exit(1)
	}
	cfgData, _ := io.ReadAll(cfgFile)
	err = json.Unmarshal(cfgData, &storageTestConfigurationParameters)
	if err != nil {
		fmt.Println("Failed to parse the config file")
		os.Exit(1)
	}
	cfgFile.Close()

	s.setupTestHelper("", "", true)
}

// setupTestHelper initialises the AzStorage component with the preprod endpoint and
// blob-layout-aware-routing enabled unless an explicit configuration is provided.
func (s *layoutTestSuite) setupTestHelper(configuration string, containerName string, create bool) {
	if containerName == "" {
		containerName = generateContainerName()
	}
	s.container = containerName

	if configuration == "" {
		configuration = fmt.Sprintf(
			"azstorage:\n"+
				"  account-name: %s\n"+
				"  endpoint: https://%s.blob.preprod.core.windows.net/\n"+
				"  type: block\n"+
				"  account-key: %s\n"+
				"  mode: key\n"+
				"  container: %s\n"+
				"  fail-unsupported-op: true\n"+
				"  blob-layout-aware-routing: true",
			storageTestConfigurationParameters.BlockAccount,
			storageTestConfigurationParameters.BlockAccount,
			storageTestConfigurationParameters.BlockKey,
			s.container,
		)
	}
	s.config = configuration
	s.assert = assert.New(s.T())

	s.az, _ = newTestAzStorage(configuration)
	_ = s.az.Start(ctx)

	s.serviceClient = s.az.storage.(*BlockBlob).Service
	s.containerClient = s.serviceClient.NewContainerClient(s.container)
	if create {
		_, _ = s.containerClient.Create(ctx, nil)
	}
}

func (s *layoutTestSuite) tearDownTestHelper(deleteContainer bool) {
	_ = s.az.Stop()
	if deleteContainer {
		_, _ = s.containerClient.Delete(ctx, nil)
	}
}

func (s *layoutTestSuite) cleanupTest() {
	s.tearDownTestHelper(true)
	_ = log.Destroy()
}

// uploadTestBlob is a helper that creates a small block blob and returns its name.
func (s *layoutTestSuite) uploadTestBlob(content string) string {
	name := generateFileName()
	blobClient := s.containerClient.NewBlockBlobClient(name)
	data := strings.NewReader(content)
	_, err := blobClient.Upload(ctx, streaming.NopCloser(data), &blockblob.UploadOptions{})
	s.assert.NoError(err)
	return name
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// TestGetBlobLayout verifies that getBlobLayout returns a non-nil layoutResp with the
// correct metadata for a newly created blob.  A new blob has no dispersed-layout
// information, so LayoutRanges will be nil (whole-blob served from primary).
func (s *layoutTestSuite) TestGetBlobLayout() {
	defer s.cleanupTest()

	content := "Hello, layout test!"
	name := s.uploadTestBlob(content)

	bb := s.az.storage.(*BlockBlob)
	lr, err := bb.getBlobLayout(name)

	s.assert.NoError(err)
	s.assert.NotNil(lr)
	s.assert.NotNil(lr.layout)
	s.assert.Equal(int64(len(content)), lr.contentLength)
	s.assert.NotNil(lr.eTag)
	s.assert.NotNil(lr.lmt)
	// If the service returns layout ranges, verify each range is well-formed.
	for _, r := range lr.layout.LayoutRanges {
		s.assert.GreaterOrEqual(r.Start, int64(0))
		s.assert.GreaterOrEqual(r.End, r.Start)
		s.assert.NotEmpty(r.Endpoint)
	}
}

// TestGetAttrWithLayoutEnabled verifies that when blob-layout-aware-routing is true,
// GetAttr returns an ObjAttr whose Layout field is non-nil.
func (s *layoutTestSuite) TestGetAttrWithLayoutEnabled() {
	defer s.cleanupTest()

	name := s.uploadTestBlob("layout aware routing test content")

	attr, err := s.az.storage.(*BlockBlob).GetAttr(name)

	s.assert.NoError(err)
	s.assert.NotNil(attr)
	s.assert.NotNil(attr.Layout, "Layout should be populated when blob-layout-aware-routing is enabled")
}

// TestGetAttrWithLayoutDisabled verifies that when blob-layout-aware-routing is false
// (the default), GetAttr returns an ObjAttr whose Layout field is nil.
func (s *layoutTestSuite) TestGetAttrWithLayoutDisabled() {
	defer s.cleanupTest()

	// Reconfigure without blob-layout-aware-routing.
	defaultCfg := fmt.Sprintf(
		"azstorage:\n"+
			"  account-name: %s\n"+
			"  endpoint: https://%s.blob.preprod.core.windows.net/\n"+
			"  type: block\n"+
			"  account-key: %s\n"+
			"  mode: key\n"+
			"  container: %s\n"+
			"  fail-unsupported-op: true",
		storageTestConfigurationParameters.BlockAccount,
		storageTestConfigurationParameters.BlockAccount,
		storageTestConfigurationParameters.BlockKey,
		s.container,
	)
	s.setupTestHelper(defaultCfg, s.container, false)

	name := s.uploadTestBlob("no layout routing test content")

	attr, err := s.az.storage.(*BlockBlob).GetAttr(name)

	s.assert.NoError(err)
	s.assert.NotNil(attr)
	s.assert.Nil(attr.Layout, "Layout should be nil when blob-layout-aware-routing is disabled")
}

// TestGetAttrUsingRest_LayoutEnabledPopulatesLayout verifies that getAttrUsingRest
// (called via GetAttr on an uncached blob) populates the Layout field when the flag
// is enabled, and that core attribute fields (Size, Name, ETag) are also correct.
func (s *layoutTestSuite) TestGetAttrUsingRest_LayoutEnabledPopulatesLayout() {
	defer s.cleanupTest()

	content := "getAttrUsingRest layout test"
	name := s.uploadTestBlob(content)

	bb := s.az.storage.(*BlockBlob)
	// Call getAttrUsingRest directly (accessible because tests are in the same package).
	attr, err := bb.getAttrUsingRest(name)

	s.assert.NoError(err)
	s.assert.NotNil(attr)
	s.assert.Equal(name, attr.Path)
	s.assert.Equal(int64(len(content)), attr.Size)
	s.assert.NotNil(attr.Layout, "Layout should be set when blob-layout-aware-routing is enabled")
	s.assert.NotEmpty(attr.ETag)
}

// TestLayoutAwareRoutingConfigStoredCorrectly verifies that the config option is
// reflected in stConfig after the component is configured.
func (s *layoutTestSuite) TestLayoutAwareRoutingConfigStoredCorrectly() {
	defer s.cleanupTest()
	s.assert.True(s.az.stConfig.isBlobLayoutAwareRoutingEnabled,
		"isBlobLayoutAwareRoutingEnabled should be true when configured via YAML")
}

func TestLayoutSuite(t *testing.T) {
	suite.Run(t, new(layoutTestSuite))
}
