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

// Package dcache_e2e tests distributed_cache against a kind and Tachyon cluster.
// Tests restart the blobfuse2 pod when they require a cold local cache.
package dcache_e2e

import (
	"flag"
	"fmt"
	"os"
	"testing"
)

// testCfg holds flag and environment configuration resolved by TestMain.
var testCfg struct {
	// Azure credentials seed blobs outside the read-only mount.
	storageAccount   string
	storageKey       string
	storageEndpoint  string
	storageContainer string

	// Coordinates of the pre-existing blobfuse2 Deployment.
	podNamespace  string
	podDeployment string
	podMountPath  string
	kubectlBin    string
	dockerBin     string

	// Coordinates used for cache-server fault injection, recovery, and
	// in-pod Prometheus scrapes.
	cacheserverNamespace   string
	cacheserverSelector    string
	cacheserverStatefulSet string
	cacheserverMetricsPort int

	// Bounded concurrency workload sizing. Defaults are intentionally large
	// enough to exercise contention while remaining viable on the nightly
	// four-node kind cluster.
	concurrencyPods          int
	concurrencyReadersPerPod int
	concurrencyFiles         int

	// Expected userspace identity of the blobfuse2 runtime image.
	runtimeDistro  string
	runtimeVersion string
	runtimeArch    string
}

func registerFlags() {
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
	if flag.Lookup("pod-namespace") == nil {
		flag.StringVar(&testCfg.podNamespace, "pod-namespace", "blobfuse2-dist-cache",
			"pod driver: Kubernetes namespace containing the blobfuse2 Deployment")
	}
	if flag.Lookup("pod-deployment") == nil {
		flag.StringVar(&testCfg.podDeployment, "pod-deployment", "blobfuse2-dist-cache",
			"pod driver: name of the Deployment the test should cycle between reads")
	}
	if flag.Lookup("pod-mount-path") == nil {
		flag.StringVar(&testCfg.podMountPath, "pod-mount-path", "/mnt/blobfuse_mnt",
			"pod driver: absolute path inside the container where blobfuse2 is mounted")
	}
	if flag.Lookup("kubectl-bin") == nil {
		flag.StringVar(&testCfg.kubectlBin, "kubectl-bin", "kubectl",
			"pod driver: path to the kubectl binary")
	}
	if flag.Lookup("docker-bin") == nil {
		flag.StringVar(&testCfg.dockerBin, "docker-bin", "docker",
			"kind fault injection: path to the docker binary")
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
		flag.StringVar(&testCfg.cacheserverStatefulSet, "cacheserver-statefulset", "cacheserver",
			"cache-server: name of the Tachyon StatefulSet (waited on after a pod is deleted)")
	}
	if flag.Lookup("cacheserver-metrics-port") == nil {
		flag.IntVar(&testCfg.cacheserverMetricsPort, "cacheserver-metrics-port", 9096,
			"cache-server: pod-local Prometheus port scraped via kubectl exec curl")
	}
	if flag.Lookup("concurrency-pods") == nil {
		flag.IntVar(&testCfg.concurrencyPods, "concurrency-pods", 5,
			"concurrency tests: blobfuse2 pod count")
	}
	if flag.Lookup("concurrency-readers-per-pod") == nil {
		flag.IntVar(&testCfg.concurrencyReadersPerPod, "concurrency-readers-per-pod", 2,
			"concurrency tests: simultaneous readers per pod")
	}
	if flag.Lookup("concurrency-files") == nil {
		flag.IntVar(&testCfg.concurrencyFiles, "concurrency-files", 3,
			"concurrency tests: number of independent files")
	}
	if flag.Lookup("runtime-distro") == nil {
		flag.StringVar(&testCfg.runtimeDistro, "runtime-distro", "",
			"expected ID from the blobfuse2 pod's /etc/os-release")
	}
	if flag.Lookup("runtime-version") == nil {
		flag.StringVar(&testCfg.runtimeVersion, "runtime-version", "",
			"expected VERSION_ID prefix from the blobfuse2 pod's /etc/os-release")
	}
	if flag.Lookup("runtime-arch") == nil {
		flag.StringVar(&testCfg.runtimeArch, "runtime-arch", "",
			"expected normalized blobfuse2 pod architecture (amd64 or arm64)")
	}
}

// envDefault returns v if it is non-empty, otherwise os.Getenv(k).
func envDefault(v, k string) string {
	if v != "" {
		return v
	}
	return os.Getenv(k)
}

func ensurePodMountArgs() error {
	if testCfg.podNamespace == "" {
		return fmt.Errorf("pod driver requires -pod-namespace")
	}
	if testCfg.podDeployment == "" {
		return fmt.Errorf("pod driver requires -pod-deployment")
	}
	if testCfg.podMountPath == "" {
		return fmt.Errorf("pod driver requires -pod-mount-path")
	}
	if testCfg.kubectlBin == "" {
		return fmt.Errorf("pod driver requires -kubectl-bin")
	}
	if testCfg.dockerBin == "" {
		return fmt.Errorf("kind fault injection requires -docker-bin")
	}
	if testCfg.concurrencyPods < 2 {
		return fmt.Errorf("concurrency tests require -concurrency-pods >= 2")
	}
	if testCfg.concurrencyReadersPerPod < 1 {
		return fmt.Errorf("concurrency tests require -concurrency-readers-per-pod >= 1")
	}
	if testCfg.concurrencyFiles < 2 {
		return fmt.Errorf("concurrency tests require -concurrency-files >= 2")
	}
	return nil
}

// resolveCfg applies environment fallbacks.
func resolveCfg() error {
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
	return nil
}

func TestMain(m *testing.M) {
	flag.Parse()

	if err := resolveCfg(); err != nil {
		fmt.Fprintf(os.Stderr, "distributed_cache E2E setup failed: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("distributed_cache E2E config:\n"+
		"  pod-namespace           = %s\n"+
		"  pod-deployment          = %s\n"+
		"  pod-mount-path          = %s\n"+
		"  docker-bin              = %s\n"+
		"  cacheserver-namespace   = %s\n"+
		"  cacheserver-selector    = %s\n"+
		"  cacheserver-metrics-port= %d\n"+
		"  storage-account         = %s\n"+
		"  storage-endpoint        = %s\n"+
		"  storage-container       = %s\n"+
		"  runtime-distro          = %s\n"+
		"  runtime-version         = %s\n"+
		"  runtime-arch            = %s\n",
		testCfg.podNamespace, testCfg.podDeployment, testCfg.podMountPath,
		testCfg.dockerBin,
		testCfg.cacheserverNamespace, testCfg.cacheserverSelector, testCfg.cacheserverMetricsPort,
		testCfg.storageAccount, testCfg.storageEndpoint, testCfg.storageContainer,
		testCfg.runtimeDistro, testCfg.runtimeVersion, testCfg.runtimeArch)

	os.Exit(m.Run())
}

func init() {
	registerFlags()
}
