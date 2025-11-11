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

package common

import (
	"errors"
	"strconv"
	"strings"
)

const Blobfuse2ListContainerURL = "https://blobfuse2.z13.web.core.windows.net/release"
const BlobFuse2WarningsURL = "https://aka.ms/blobfuse2warnings"
const BlobFuse2BlockingURL = "https://aka.ms/blobfuse2blockers"

type Version struct {
	segments []int64
	preview  bool
	original string
}

// To keep the code simple, we assume we only use a simple subset of semantic versions.
// Namely, the version is either a normal stable version, or a pre-release version with '~preview' or '-preview' attached.
// Examples: 10.1.0, 11.2.0-preview.1, 11.2.0~preview.1
func ParseVersion(raw string) (*Version, error) {
	const standardError = "invalid version string"

	rawSegments := strings.Split(raw, ".")
	if !(len(rawSegments) == 3 || (len(rawSegments) == 4 && (strings.Contains(rawSegments[2], "-") || strings.Contains(rawSegments[2], "~")))) {
		return nil, errors.New(standardError)
	}

	v := &Version{segments: make([]int64, 4), original: raw}
	for i, str := range rawSegments {
		//For any case such as SemVer-preview.1, SemVer-beta.1, SemVer-alpha.1 this would be true, and we assume the version to be a preview version.
		if strings.Contains(str, "-") || strings.Contains(str, "~") {
			if i != 2 {
				return nil, errors.New(standardError)
			}
			v.preview = true
			//Splitting the string into two pieces and extracting SemVer which is always at 0th index
			str = strings.Split(strings.Split(str, "-")[0], "~")[0]
		}

		val, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return nil, errors.New(standardError)
		}
		v.segments[i] = val
	}

	return v, nil
}

// compare this version (v) to another version (v2)
// return -1 if v is smaller/older than v2
// return 0 if v is equal to v2
// return 1 if v is bigger/newer than v2
func (v Version) compare(v2 Version) int {
	// short-circuit if the two version have the exact same raw string, no need to compare
	if v.original == v2.original {
		return 0
	}

	// TODO: Make sure preview version is smaller than GA version

	// compare the major/minor/patch version
	// if v has a bigger number, it is newer
	for i := range 3 {
		if v.segments[i] > v2.segments[i] {
			return 1
		} else if v.segments[i] < v2.segments[i] {
			return -1
		}
	}

	// if both or neither versions are previews, then they are equal
	// usually this shouldn't happen since we already checked whether the two versions have equal raw string
	// however, it is entirely possible that we have new kinds of pre-release versions that this code is not parsing correctly
	// in this case we consider both pre-release version equal anyways
	if v.preview && v2.preview {
		if v.segments[3] > v2.segments[3] {
			return 1
		} else if v.segments[3] < v2.segments[3] {
			return -1
		} else {
			return 0
		}
	} else if !v.preview && !v2.preview {
		return 0
	} else if v.preview && !v2.preview {
		return -1
	}

	return 1
}

// detect if version v is older than v2
func (v Version) OlderThan(v2 Version) bool {
	return v.compare(v2) == -1
}

// detect if version v is newer than v2
func (v Version) NewerThan(v2 Version) bool {
	return v.compare(v2) == 1
}
