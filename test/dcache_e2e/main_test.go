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

	// Pod-driver configuration. The suite drives blobfuse2 exclusively as
	// a pre-existing Kubernetes Deployment, cycling the pod between reads
	// so block_cache starts cold. Required to exercise dist_cache's
	// discovery-url / k8s-service code paths, which need in-cluster DNS.
	podNamespace  string
	podDeployment string
	podSelector   string
	podMountPath  string
	kubectlBin    string

	// Cache-server (Tachyon) StatefulSet coordinates. Needed by tests
	// that intentionally disrupt an L2 node -- they delete a cacheserver
	// pod and then wait for the StatefulSet to reconcile in cleanup so
	// the cluster is left healthy for subsequent tests. Defaults mirror
	// test/scripts/dcache/config/nightly.config (NAMESPACE=cache-server,
	// app=cacheserver label, StatefulSet named cache-server).
	cacheserverNamespace   string
	cacheserverSelector    string
	cacheserverStatefulSet string
}

func registerFlags() {
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
	if flag.Lookup("pod-namespace") == nil {
		flag.StringVar(&testCfg.podNamespace, "pod-namespace", "blobfuse2-dist-cache",
			"pod driver: Kubernetes namespace containing the blobfuse2 Deployment")
	}
	if flag.Lookup("pod-deployment") == nil {
		flag.StringVar(&testCfg.podDeployment, "pod-deployment", "blobfuse2-dist-cache",
			"pod driver: name of the Deployment the test should cycle between reads")
	}
	if flag.Lookup("pod-selector") == nil {
		flag.StringVar(&testCfg.podSelector, "pod-selector", "app=blobfuse2-dist-cache",
			"pod driver: label selector used to resolve the current Ready pod after each rollout")
	}
	if flag.Lookup("pod-mount-path") == nil {
		flag.StringVar(&testCfg.podMountPath, "pod-mount-path", "/mnt/blobfuse_mnt",
			"pod driver: absolute path inside the container where blobfuse2 is mounted")
	}
	if flag.Lookup("kubectl-bin") == nil {
		flag.StringVar(&testCfg.kubectlBin, "kubectl-bin", "kubectl",
			"pod driver: path to the kubectl binary")
	}
	if flag.Lookup("cacheserver-namespace") == nil {
		flag.StringVar(&testCfg.cacheserverNamespace, "cacheserver-namespace", "cache-server",
			"cache-server: Kubernetes namespace containing the Tachyon StatefulSet")
	}
	if flag.Lookup("cacheserver-selector") == nil {
		flag.StringVar(&testCfg.cacheserverSelector, "cacheserver-selector", "app=cacheserver",
			"cache-server: label selector used to enumerate cache-server pods")
	}
	if flag.Lookup("cacheserver-statefulset") == nil {
		flag.StringVar(&testCfg.cacheserverStatefulSet, "cacheserver-statefulset", "cache-server",
			"cache-server: name of the Tachyon StatefulSet (waited on after a pod is deleted)")
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
	testCfg.metricsEndpointsRaw = envDefault(testCfg.metricsEndpointsRaw, "DCACHE_METRICS_ENDPOINTS")
	testCfg.storageAccount = envDefault(testCfg.storageAccount, "STO_ACC_NAME")
	testCfg.storageKey = envDefault(testCfg.storageKey, "STO_ACC_KEY")
	testCfg.storageEndpoint = envDefault(testCfg.storageEndpoint, "STO_ACC_ENDPOINT")
	testCfg.storageContainer = envDefault(testCfg.storageContainer, "containerName")
	if testCfg.storageContainer == "" {
		testCfg.storageContainer = os.Getenv("DCACHE_E2E_CONTAINER")
	}

	if err := ensurePodMountArgs(); err != nil {
		return err
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

	activeMounter = &podMounter{}

	fmt.Printf("dist_cache E2E config:\n"+
		"  pod-namespace      = %s\n"+
		"  pod-deployment     = %s\n"+
		"  pod-selector       = %s\n"+
		"  pod-mount-path     = %s\n"+
		"  metrics-endpoints  = %v\n"+
		"  storage-account    = %s\n"+
		"  storage-endpoint   = %s\n"+
		"  storage-container  = %s\n"+
		"  quick-test         = %s\n",
		testCfg.podNamespace, testCfg.podDeployment, testCfg.podSelector, testCfg.podMountPath,
		testCfg.metricsEndpoints,
		testCfg.storageAccount, testCfg.storageEndpoint, testCfg.storageContainer,
		testCfg.quickTest)

	// The Deployment lifecycle is owned by the operator, not the test
	// binary — tests only cycle pods within it — so there is no host-side
	// mount to sweep here.

	os.Exit(m.Run())
}

func init() {
	registerFlags()
}
