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

// TestCLIFlags_ReadPath_NoYAML runs the same L2 miss -> populate -> remount
// -> hit scenario as TestReadPath_L2MissPopulatesAndHits, but against a
// blobfuse2 pod launched purely from `--distributed-cache-*` CLI flags and
// azstorage env vars, with no config.yaml mounted at all.
//
// If any of the CLI flag bindings introduced in PR #2316 regresses -- the
// flag isn't registered, isn't bound to viper, isn't consumed by
// distributed_cache.Configure, or the pipeline isn't auto-populated when
// the discovery endpoint is set without a components: list -- either the pod
// never becomes Ready, the L2 miss produces zero Upload/Success deltas, or the
// L2 hit returns zero Download/Success deltas. Any of those is a fatal error in
// runReadPathL2MissHitScenario.
//
// Passing this test is a strong end-to-end statement: distributed_cache can
// be fully driven from the CLI surface, and the CLI surface produces the
// same L2 populate/hit behavior as the YAML surface exercised by
// TestReadPath_L2MissPopulatesAndHits.
func TestCLIFlags_ReadPath_NoYAML(t *testing.T) {
	runReadPathL2MissHitScenario(t, newCLIPodMounter(t, defaultCLIFlags()))
}
