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

package clustermanager

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache"
	"github.com/stretchr/testify/suite"
)

type utilsTestSuite struct {
	suite.Suite
}

func TestUtils(t *testing.T) {
	suite.Run(t, new(utilsTestSuite))
}

func LoadAndValidateClusterMapFromFile(path string) dcache.ClusterMap {
	if path == "" {
		path = "clustermap_dummy_success.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Err("utils: failed to read cluster map JSON file: %v", err)
	}

	var cm dcache.ClusterMap
	if err := json.Unmarshal(data, &cm); err != nil {
		log.Err("utils: failed to unmarshal cluster map JSON: %v", err)
	}
	return cm
}

func cloneClusterMap(src dcache.ClusterMap) dcache.ClusterMap {
	dst := src
	dst.RVMap = make(map[string]dcache.RawVolume, len(src.RVMap))
	for k, v := range src.RVMap {
		dst.RVMap[k] = v
	}
	dst.MVMap = make(map[string]dcache.MirroredVolume, len(src.MVMap))
	for k, v := range src.MVMap {
		dst.MVMap[k] = v
	}
	return dst
}

func (suite *utilsTestSuite) TestIsValidClusterMap() {
	// Test case: Valid ClusterMap
	validClusterMap := LoadAndValidateClusterMapFromFile("")
	isValid, errMsg := IsValidClusterMap(validClusterMap)
	suite.True(isValid)
	suite.Empty(errMsg)

	// Test case: Invalid CreatedAt
	invalidCreatedAt := validClusterMap
	invalidCreatedAt.CreatedAt = 0
	isValid, errMsg = IsValidClusterMap(invalidCreatedAt)
	suite.False(isValid)
	suite.Contains(errMsg, "Invalid CreatedAt")

	// Test case: Invalid LastUpdatedAt
	invalidLastUpdatedAt := validClusterMap
	invalidLastUpdatedAt.LastUpdatedAt = 0
	isValid, errMsg = IsValidClusterMap(invalidLastUpdatedAt)
	suite.False(isValid)
	suite.Contains(errMsg, "Invalid LastUpdatedAt")

	// Test case: LastUpdatedAt < CreatedAt
	invalidTimestamps := validClusterMap
	invalidTimestamps.LastUpdatedAt = invalidTimestamps.CreatedAt - 1
	isValid, errMsg = IsValidClusterMap(invalidTimestamps)
	suite.False(isValid)
	suite.Contains(errMsg, "LastUpdatedAt")

	// Test case: Invalid LastUpdatedBy UUID
	invalidLastUpdatedBy := validClusterMap
	invalidLastUpdatedBy.LastUpdatedBy = "invalid-uuid"
	isValid, errMsg = IsValidClusterMap(invalidLastUpdatedBy)
	suite.False(isValid)
	suite.Contains(errMsg, "Invalid LastUpdatedBy UUID")

	// Test case: Invalid Config.HeartbeatSeconds
	invalidHeartbeatSeconds := validClusterMap
	invalidHeartbeatSeconds.Config.HeartbeatSeconds = 0
	isValid, errMsg = IsValidClusterMap(invalidHeartbeatSeconds)
	suite.False(isValid)
	suite.Contains(errMsg, "Invalid Config.HeartbeatSeconds")

	// Test case: Invalid Config.ClustermapEpoch
	invalidClustermapEpoch := validClusterMap
	invalidClustermapEpoch.Config.ClustermapEpoch = 0
	isValid, errMsg = IsValidClusterMap(invalidClustermapEpoch)
	suite.False(isValid)
	suite.Contains(errMsg, "Invalid Config.ClustermapEpoch")

	// Test case: Duplicate RvId in RVMap
	duplicateRvId := cloneClusterMap(validClusterMap)
	duplicateRvId.RVMap["rv2"] = duplicateRvId.RVMap["rv1"]
	isValid, errMsg = IsValidClusterMap(duplicateRvId)
	suite.False(isValid)
	suite.Contains(errMsg, "duplicate RvId")

	// Test case: Invalid MVMap entry
	invalidMVMap := cloneClusterMap(validClusterMap)
	for k, v := range validClusterMap.MVMap {
		invalidMVMap.MVMap[k] = v
	}
	invalidMVMap.MVMap["mv1"] = dcache.MirroredVolume{
		State: "invalid-state",
		RVs:   map[string]dcache.StateEnum{},
	}
	isValid, errMsg = IsValidClusterMap(invalidMVMap)
	suite.False(isValid)
	suite.Contains(errMsg, "Invalid mv State")
}
