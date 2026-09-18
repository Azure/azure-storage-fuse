//go:build !unittest
// +build !unittest

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

package dcache_e2e

import "testing"

// TestCLIFlags_ReadPath_NoYAML runs an L2 miss -> populate -> remount -> hit
// scenario against a blobfuse2 pod launched purely from
// `--distributed-cache-*` CLI flags and azstorage env vars, with no config.yaml
// mounted at all. Non-default tuning values make ignored or incorrectly bound
// flags observable through startup logs and the populated chunk count.
//
// If any of the CLI flag bindings introduced in PR #2316 regresses -- the
// flag isn't registered, isn't bound to viper, isn't consumed by
// distributed_cache.Configure, or the pipeline isn't auto-populated when
// the discovery endpoint is set without a components: list -- the pod fails
// readiness, the startup configuration differs from the requested values, or
// the L2 metrics do not reflect the configured chunking and cache activity.
//
// Passing this test proves that distributed_cache can be fully driven from the
// CLI surface and that its tuning flags reach DistCache::Configure and
// BlockCache::Configure.
func TestCLIFlags_ReadPath_NoYAML(t *testing.T) {
	flags := defaultCLIFlags()
	flags.blockSizeMB = 4
	flags.memoryMB = 256
	flags.prefetch = 12
	flags.parallelism = 9
	flags.ttlSeconds = 300

	const bytesPerMB = 1024 * 1024
	runReadPathL2MissHitScenario(t, newCLIPodMounter(t, flags), flags.blockSizeMB*bytesPerMB)
}
