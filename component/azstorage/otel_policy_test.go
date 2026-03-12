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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

type OtelPolicyTestSuite struct {
	suite.Suite
}

func TestOtelPolicySuite(t *testing.T) {
	suite.Run(t, new(OtelPolicyTestSuite))
}

// TearDownTest resets the singleton between tests so each test starts clean.
func (s *OtelPolicyTestSuite) TearDownTest() {
	ResetOtelRequestPolicy()
}

// ---------------------------------------------------------------------------
// classifyOperation — Blob GET operations
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyListContainers() {
	op := classifyOperation("GET", "/", "comp=list")
	assert.Equal(s.T(), "ListContainers", op)
}

func (s *OtelPolicyTestSuite) TestClassifyListBlobs() {
	op := classifyOperation("GET", "/mycontainer", "restype=container&comp=list&prefix=abc")
	assert.Equal(s.T(), "ListBlobs", op)
}

func (s *OtelPolicyTestSuite) TestClassifyGetBlob() {
	op := classifyOperation("GET", "/mycontainer/myblob.txt", "")
	assert.Equal(s.T(), "GetBlob", op)
}

func (s *OtelPolicyTestSuite) TestClassifyGetBlobNested() {
	op := classifyOperation("GET", "/mycontainer/dir1/dir2/blob.bin", "")
	assert.Equal(s.T(), "GetBlob", op)
}

func (s *OtelPolicyTestSuite) TestClassifyGetBlockList() {
	op := classifyOperation("GET", "/mycontainer/myblob", "comp=blocklist")
	assert.Equal(s.T(), "GetBlockList", op)
}

func (s *OtelPolicyTestSuite) TestClassifyGetBlobMetadata() {
	op := classifyOperation("GET", "/mycontainer/myblob", "comp=metadata")
	assert.Equal(s.T(), "GetBlobMetadata", op)
}

func (s *OtelPolicyTestSuite) TestClassifyGetBlobTags() {
	op := classifyOperation("GET", "/mycontainer/myblob", "comp=tags")
	assert.Equal(s.T(), "GetBlobTags", op)
}

func (s *OtelPolicyTestSuite) TestClassifyGetContainerProperties() {
	op := classifyOperation("GET", "/mycontainer", "restype=container")
	assert.Equal(s.T(), "GetContainerProperties", op)
}

func (s *OtelPolicyTestSuite) TestClassifyGetFallback() {
	// Unknown GET with comp= that doesn't match known patterns
	op := classifyOperation("GET", "/", "")
	assert.Equal(s.T(), "Get", op)
}

// ---------------------------------------------------------------------------
// classifyOperation — Blob PUT operations
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyPutBlob() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "")
	assert.Equal(s.T(), "PutBlob", op)
}

func (s *OtelPolicyTestSuite) TestClassifyPutBlock() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=block&blockid=abc123")
	assert.Equal(s.T(), "PutBlock", op)
}

func (s *OtelPolicyTestSuite) TestClassifyPutBlockList() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=blocklist")
	assert.Equal(s.T(), "PutBlockList", op)
}

func (s *OtelPolicyTestSuite) TestClassifyPutPage() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=page")
	assert.Equal(s.T(), "PutPage", op)
}

func (s *OtelPolicyTestSuite) TestClassifyAppendBlock() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=appendblock")
	assert.Equal(s.T(), "AppendBlock", op)
}

func (s *OtelPolicyTestSuite) TestClassifySetBlobMetadata() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=metadata")
	assert.Equal(s.T(), "SetBlobMetadata", op)
}

func (s *OtelPolicyTestSuite) TestClassifySetBlobTags() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=tags")
	assert.Equal(s.T(), "SetBlobTags", op)
}

func (s *OtelPolicyTestSuite) TestClassifyCopyBlob() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=copy")
	assert.Equal(s.T(), "CopyBlob", op)
}

func (s *OtelPolicyTestSuite) TestClassifySetBlobTier() {
	op := classifyOperation("PUT", "/mycontainer/myblob", "comp=tier")
	assert.Equal(s.T(), "SetBlobTier", op)
}

func (s *OtelPolicyTestSuite) TestClassifyCreateContainer() {
	op := classifyOperation("PUT", "/mycontainer", "restype=container")
	assert.Equal(s.T(), "CreateContainer", op)
}

func (s *OtelPolicyTestSuite) TestClassifyPutFallback() {
	op := classifyOperation("PUT", "/", "comp=unknown")
	assert.Equal(s.T(), "Put", op)
}

// ---------------------------------------------------------------------------
// classifyOperation — HEAD operations
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyGetBlobProperties() {
	op := classifyOperation("HEAD", "/mycontainer/myblob", "")
	assert.Equal(s.T(), "GetBlobProperties", op)
}

func (s *OtelPolicyTestSuite) TestClassifyHeadContainerProperties() {
	op := classifyOperation("HEAD", "/mycontainer", "restype=container")
	assert.Equal(s.T(), "GetContainerProperties", op)
}

func (s *OtelPolicyTestSuite) TestClassifyHeadFallback() {
	op := classifyOperation("HEAD", "/", "")
	assert.Equal(s.T(), "Head", op)
}

// ---------------------------------------------------------------------------
// classifyOperation — DELETE operations
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyDeleteBlob() {
	op := classifyOperation("DELETE", "/mycontainer/myblob", "")
	assert.Equal(s.T(), "DeleteBlob", op)
}

func (s *OtelPolicyTestSuite) TestClassifyDeleteContainer() {
	op := classifyOperation("DELETE", "/mycontainer", "restype=container")
	assert.Equal(s.T(), "DeleteContainer", op)
}

func (s *OtelPolicyTestSuite) TestClassifyDeleteFallback() {
	op := classifyOperation("DELETE", "/", "")
	assert.Equal(s.T(), "Delete", op)
}

// ---------------------------------------------------------------------------
// classifyOperation — PATCH / Data Lake operations
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyDfsAppend() {
	op := classifyOperation("PATCH", "/myfs/dir/file.txt", "action=append")
	assert.Equal(s.T(), "DfsAppend", op)
}

func (s *OtelPolicyTestSuite) TestClassifyDfsFlush() {
	op := classifyOperation("PATCH", "/myfs/dir/file.txt", "action=flush&position=1024")
	assert.Equal(s.T(), "DfsFlush", op)
}

func (s *OtelPolicyTestSuite) TestClassifyDfsSetAccessControl() {
	op := classifyOperation("PATCH", "/myfs/dir/file.txt", "action=setaccesscontrol")
	assert.Equal(s.T(), "DfsSetAccessControl", op)
}

func (s *OtelPolicyTestSuite) TestClassifyDfsGetAccessControl() {
	op := classifyOperation("PATCH", "/myfs/dir/file.txt", "action=getaccesscontrol")
	assert.Equal(s.T(), "DfsGetAccessControl", op)
}

func (s *OtelPolicyTestSuite) TestClassifyDfsPatchUnknownAction() {
	op := classifyOperation("PATCH", "/myfs/dir/file.txt", "action=rename")
	assert.Equal(s.T(), "DfsPatch_rename", op)
}

func (s *OtelPolicyTestSuite) TestClassifyPatchFallback() {
	op := classifyOperation("PATCH", "/myfs/dir/file.txt", "")
	assert.Equal(s.T(), "Patch", op)
}

// ---------------------------------------------------------------------------
// classifyOperation — Unknown method
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyUnknownMethod() {
	op := classifyOperation("OPTIONS", "/something", "")
	assert.Equal(s.T(), "OPTIONS", op)
}

// ---------------------------------------------------------------------------
// classifyStatus tests
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyStatus200() {
	assert.Equal(s.T(), "success", classifyStatus(200))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus201() {
	assert.Equal(s.T(), "success", classifyStatus(201))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus206() {
	assert.Equal(s.T(), "success", classifyStatus(206))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus304() {
	assert.Equal(s.T(), "not_modified", classifyStatus(304))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus404() {
	assert.Equal(s.T(), "not_found", classifyStatus(404))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus409() {
	assert.Equal(s.T(), "conflict", classifyStatus(409))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus412() {
	assert.Equal(s.T(), "precondition_failed", classifyStatus(412))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus429() {
	assert.Equal(s.T(), "throttled", classifyStatus(429))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus500() {
	assert.Equal(s.T(), "server_error", classifyStatus(500))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus503() {
	assert.Equal(s.T(), "server_error", classifyStatus(503))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus400() {
	assert.Equal(s.T(), "error", classifyStatus(400))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus401() {
	assert.Equal(s.T(), "error", classifyStatus(401))
}

func (s *OtelPolicyTestSuite) TestClassifyStatus403() {
	assert.Equal(s.T(), "error", classifyStatus(403))
}

// ---------------------------------------------------------------------------
// extractQueryParam tests
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestExtractQueryParamMiddle() {
	val := extractQueryParam("restype=container&comp=list&prefix=abc", "comp")
	assert.Equal(s.T(), "list", val)
}

func (s *OtelPolicyTestSuite) TestExtractQueryParamFirst() {
	val := extractQueryParam("comp=blocklist&blockid=abc", "comp")
	assert.Equal(s.T(), "blocklist", val)
}

func (s *OtelPolicyTestSuite) TestExtractQueryParamLast() {
	val := extractQueryParam("restype=container&comp=metadata", "comp")
	assert.Equal(s.T(), "metadata", val)
}

func (s *OtelPolicyTestSuite) TestExtractQueryParamMissing() {
	val := extractQueryParam("restype=container", "comp")
	assert.Equal(s.T(), "", val)
}

func (s *OtelPolicyTestSuite) TestExtractQueryParamEmpty() {
	val := extractQueryParam("", "comp")
	assert.Equal(s.T(), "", val)
}

// ---------------------------------------------------------------------------
// NewOtelRequestPolicy singleton and nil-meter behaviour
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestNewPolicyNilMeter() {
	// When OTel metrics are not started, GetOtelMeter returns nil,
	// and NewOtelRequestPolicy should return nil.
	ResetOtelRequestPolicy()
	p := NewOtelRequestPolicy()
	assert.Nil(s.T(), p, "Policy should be nil when metrics are not enabled")
}

func (s *OtelPolicyTestSuite) TestResetOtelRequestPolicy() {
	// Reset should clear the singleton even if nothing was set
	ResetOtelRequestPolicy()
	assert.Nil(s.T(), otelPolicyInstance, "Singleton should be nil after reset")
}

// ---------------------------------------------------------------------------
// classifyOperation — edge cases
// ---------------------------------------------------------------------------

func (s *OtelPolicyTestSuite) TestClassifyOperationEmptyPath() {
	op := classifyOperation("GET", "", "comp=list")
	// Empty path → segCount 0, comp=list, restype="" → ListContainers
	assert.Equal(s.T(), "ListContainers", op)
}

func (s *OtelPolicyTestSuite) TestClassifyOperationCaseSensitiveQuery() {
	// The raw query should be lowered by classifyOperation, test mixed case
	op := classifyOperation("GET", "/mycontainer", "Restype=Container&Comp=List")
	assert.Equal(s.T(), "ListBlobs", op)
}

func (s *OtelPolicyTestSuite) TestClassifyListBlobsWithMarker() {
	// ListBlobs with marker and maxresults
	op := classifyOperation("GET", "/mycontainer", "restype=container&comp=list&marker=xyz&maxresults=100")
	assert.Equal(s.T(), "ListBlobs", op)
}

func (s *OtelPolicyTestSuite) TestClassifyPutBlobNestedPath() {
	op := classifyOperation("PUT", "/container/a/b/c/deep.txt", "")
	assert.Equal(s.T(), "PutBlob", op)
}

func (s *OtelPolicyTestSuite) TestClassifyDeleteBlobNestedPath() {
	op := classifyOperation("DELETE", "/container/a/b/c/deep.txt", "")
	assert.Equal(s.T(), "DeleteBlob", op)
}

func (s *OtelPolicyTestSuite) TestClassifyHeadBlobNestedPath() {
	op := classifyOperation("HEAD", "/container/dir/file.txt", "")
	assert.Equal(s.T(), "GetBlobProperties", op)
}
