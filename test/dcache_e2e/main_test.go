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

// Package dcache_e2e drives dist_cache-focused blobfuse2 E2E tests against a
// kind + Tachyon cluster stood up by test/scripts/dcache/*.
//
// Unlike test/e2e_tests, this package owns the blobfuse2 mount lifecycle:
// most scenarios need a *fresh* mount between reads to guarantee the local
// block_cache is empty, so that an L2-miss (dist_cache) code path is actually
// exercised rather than being served from the local in-memory cache.
package dcache_e2e

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

// testCfg is populated in TestMain from CLI flags and env vars. It is
// deliberately global (rather than passed through every helper) to keep the
// per-test call sites readable; each test still receives its own *testing.T.
var testCfg struct {
	// Path to the blobfuse2 binary the test should invoke for mount/unmount.
	// Falls back to "blobfuse2" (PATH lookup) when unset.
	blobfuseBin string

	// Absolute path to the pre-generated blobfuse2 config file. Must already
	// have the dist_cache.server-list, azstorage credentials, and container
	// name substituted. Produced by `blobfuse2 gen-test-config` upstream in
	// the pipeline (see azure-pipeline-templates/dist-cache-e2e.yml).
	configFile string

	// Mount point the test will (un)mount blobfuse2 at. Must exist and be
	// empty on entry; the test does *not* create it, matching the pipeline
	// contract for MOUNT_DIR.
	mntPath string

	// Path to the local cache directory used by file_cache / block_cache.
	// The test wipes it between remounts to guarantee a cold local cache.
	tmpPath string

	// Comma-separated list of Prometheus scrape URLs, one per cacheserver
	// pod, e.g. "http://localhost:9096,http://localhost:9097". Produced by
	// test/scripts/dcache/expose-metrics.sh.
	//
	// When empty, tests that require metric-based assertions will be
	// skipped rather than failed -- this lets the suite still run in a
	// minimal local iteration mode where only behavioural assertions are
	// meaningful.
	metricsEndpointsRaw string
	metricsEndpoints    []string

	// Azure Storage credentials for out-of-band blob seeding. Required
	// because the FUSE mount is opened read-only -- test payloads have to
	// reach azstorage through the SDK, not through blobfuse2.
	//
	// storageEndpoint is the account-level URL, e.g.
	//   https://myacct.blob.core.windows.net
	// storageContainer is the container name (no leading '/'). These
	// mirror the STO_ACC_* + containerName variables the existing e2e
	// pipeline already provides; we accept the same names here to avoid
	// introducing new ADO variable group entries.
	storageAccount   string
	storageKey       string
	storageEndpoint  string
	storageContainer string

	// "true" / "false". When true, expensive or long-running tests are
	// skipped. Mirrors the existing test/e2e_tests convention.
	quickTest string
}

func registerFlags() {
	if flag.Lookup("blobfuse-bin") == nil {
		flag.StringVar(&testCfg.blobfuseBin, "blobfuse-bin", "blobfuse2",
			"Path to the blobfuse2 binary used to mount/unmount")
	}
	if flag.Lookup("config-file") == nil {
		flag.StringVar(&testCfg.configFile, "config-file", "",
			"Path to the pre-generated dist_cache blobfuse2 config file")
	}
	if flag.Lookup("mnt-path") == nil {
		flag.StringVar(&testCfg.mntPath, "mnt-path", "",
			"Mount point where blobfuse2 will be mounted")
	}
	if flag.Lookup("tmp-path") == nil {
		flag.StringVar(&testCfg.tmpPath, "tmp-path", "",
			"Local cache directory (file_cache/block_cache backing store)")
	}
	if flag.Lookup("dcache-metrics-endpoints") == nil {
		flag.StringVar(&testCfg.metricsEndpointsRaw, "dcache-metrics-endpoints", "",
			"Comma-separated Prometheus scrape URLs, one per cacheserver pod (e.g. http://localhost:9096,http://localhost:9097)")
	}
	if flag.Lookup("storage-account") == nil {
		flag.StringVar(&testCfg.storageAccount, "storage-account", "",
			"Azure Storage account name (falls back to STO_ACC_NAME)")
	}
	if flag.Lookup("storage-key") == nil {
		flag.StringVar(&testCfg.storageKey, "storage-key", "",
			"Azure Storage account key (falls back to STO_ACC_KEY)")
	}
	if flag.Lookup("storage-endpoint") == nil {
		flag.StringVar(&testCfg.storageEndpoint, "storage-endpoint", "",
			"Azure Storage service endpoint URL (falls back to STO_ACC_ENDPOINT)")
	}
	if flag.Lookup("storage-container") == nil {
		flag.StringVar(&testCfg.storageContainer, "storage-container", "",
			"Azure Storage container name (falls back to containerName / DCACHE_E2E_CONTAINER)")
	}
	if flag.Lookup("quick-test") == nil {
		flag.StringVar(&testCfg.quickTest, "quick-test", "true",
			"Skip long-running scenarios when true")
	}
}

// envDefault returns v if it is non-empty, otherwise os.Getenv(k).
func envDefault(v, k string) string {
	if v != "" {
		return v
	}
	return os.Getenv(k)
}

// resolveCfg fills in unset flag values from environment variables and
// splits the metrics endpoints list. It is called once by TestMain, after
// flag.Parse().
func resolveCfg() error {
	testCfg.blobfuseBin = envDefault(testCfg.blobfuseBin, "DCACHE_E2E_BLOBFUSE_BIN")
	if testCfg.blobfuseBin == "" {
		testCfg.blobfuseBin = "blobfuse2"
	}
	testCfg.configFile = envDefault(testCfg.configFile, "DCACHE_E2E_CONFIG")
	testCfg.mntPath = envDefault(testCfg.mntPath, "MOUNT_DIR")
	testCfg.tmpPath = envDefault(testCfg.tmpPath, "TEMP_DIR")
	testCfg.metricsEndpointsRaw = envDefault(testCfg.metricsEndpointsRaw, "DCACHE_METRICS_ENDPOINTS")
	testCfg.storageAccount = envDefault(testCfg.storageAccount, "STO_ACC_NAME")
	testCfg.storageKey = envDefault(testCfg.storageKey, "STO_ACC_KEY")
	testCfg.storageEndpoint = envDefault(testCfg.storageEndpoint, "STO_ACC_ENDPOINT")
	testCfg.storageContainer = envDefault(testCfg.storageContainer, "containerName")
	if testCfg.storageContainer == "" {
		testCfg.storageContainer = os.Getenv("DCACHE_E2E_CONTAINER")
	}

	if testCfg.configFile == "" {
		return fmt.Errorf("dist_cache config file not set (pass -config-file or DCACHE_E2E_CONFIG)")
	}
	if testCfg.mntPath == "" {
		return fmt.Errorf("mount path not set (pass -mnt-path or MOUNT_DIR)")
	}
	if testCfg.tmpPath == "" {
		return fmt.Errorf("temp path not set (pass -tmp-path or TEMP_DIR)")
	}
	if _, err := os.Stat(testCfg.configFile); err != nil {
		return fmt.Errorf("config file %q not accessible: %w", testCfg.configFile, err)
	}

	if testCfg.metricsEndpointsRaw != "" {
		for _, ep := range strings.Split(testCfg.metricsEndpointsRaw, ",") {
			ep = strings.TrimSpace(ep)
			if ep != "" {
				testCfg.metricsEndpoints = append(testCfg.metricsEndpoints, ep)
			}
		}
	}
	return nil
}

func TestMain(m *testing.M) {
	registerFlags()
	flag.Parse()

	if err := resolveCfg(); err != nil {
		fmt.Fprintf(os.Stderr, "dist_cache E2E setup failed: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("dist_cache E2E config:\n"+
		"  blobfuse-bin       = %s\n"+
		"  config-file        = %s\n"+
		"  mnt-path           = %s\n"+
		"  tmp-path           = %s\n"+
		"  metrics-endpoints  = %v\n"+
		"  storage-account    = %s\n"+
		"  storage-endpoint   = %s\n"+
		"  storage-container  = %s\n"+
		"  quick-test         = %s\n",
		testCfg.blobfuseBin, testCfg.configFile, testCfg.mntPath,
		testCfg.tmpPath, testCfg.metricsEndpoints,
		testCfg.storageAccount, testCfg.storageEndpoint, testCfg.storageContainer,
		testCfg.quickTest)

	// Best-effort: ensure we start from a clean, unmounted state so a leaked
	// mount from a prior interrupted run does not poison the first test.
	// Errors here are informational only; unmountBestEffort logs details.
	unmountBestEffort()

	code := m.Run()

	// Leave the environment as we found it. Individual tests are expected
	// to unmount before returning, but do a final sweep in case something
	// panicked mid-flight.
	unmountBestEffort()

	os.Exit(code)
}

func init() {
	registerFlags()
}
