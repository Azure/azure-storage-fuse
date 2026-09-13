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

import (
	"fmt"
	"testing"
)

// CLI-mode coverage for the distributed-cache flags.
// The reference Deployment applied by test/scripts/dcache/deploy-blobfuse2.sh
// mounts a config.yaml Secret at /usr/share/blobfuse2/config.yaml and passes
// --config-file=<that path>. The helpers in this file instead clone that
// Deployment, drop the config-file volume+arg entirely, and launch blobfuse2
// with:
//
//   * `--distributed-cache-discovery-endpoint` to enable L2 plus the remaining
//     `--distributed-cache-*` tuning flags, and
//   * a small set of first-class CLI flags for the non-distcache bits the
//     reference YAML had (`--foreground`, `--read-only`, `--allow-other`,
//     `--ignore-open-flags`, `--log-level`, `--log-file-path`,
//     `--container-name`), and
//   * azstorage credentials via env vars (AZURE_STORAGE_ACCOUNT,
//     AZURE_STORAGE_ACCESS_KEY, AZURE_STORAGE_ACCOUNT_TYPE,
//     AZURE_STORAGE_ACCOUNT_CONTAINER, AZURE_STORAGE_BLOB_ENDPOINT,
//     AZURE_STORAGE_AUTH_TYPE) sourced from testCfg.
//
// The result is a pod that mounts distributed_cache with no config.yaml at
// all, exercising the cobra -> viper -> distributed_cache wiring end-to-end.
// cmd/mount.go auto-populates the pipeline
// ([libfuse, block_cache, distributed_cache, attr_cache, azstorage]) when
// a distributed-cache discovery method is set without a components: list.

// cliFlagSet captures the exact CLI values one CLI-mode pod is launched with.
// Defaults mirror the reference YAML Secret so parity with the YAML path can
// be reasoned about; tests override individual fields when they want to
// prove a specific CLI value drove distributed_cache.Configure.
type cliFlagSet struct {
	discoveryEndpoint string
	blockSizeMB       int
	memoryMB          int
	prefetch          int
	parallelism       int
	ttlSeconds        int
}

// defaultCLIFlags returns the CLI flag values that mirror the reference YAML
// Secret, so the CLI-mode read-path scenario configures distributed_cache
// identically to the YAML-mode read-path scenario.
func defaultCLIFlags() cliFlagSet {
	return cliFlagSet{
		discoveryEndpoint: fmt.Sprintf(
			"cacheserver-discovery.%s.svc.cluster.local:9065",
			testCfg.cacheserverNamespace),
		blockSizeMB: 16,
		memoryMB:    4096,
		prefetch:    32,
		parallelism: 128,
		ttlSeconds:  0,
	}
}

// cliArgs returns the shell script the blobfuse2 container will run. It
// launches blobfuse2 with no --config-file: every setting the reference
// config.yaml provided is passed as a CLI flag (distributed-cache tuning,
// libfuse/logging knobs, container name) or an env var (azstorage creds,
// injected by newCLIPodMounter's container env: block).
func (f cliFlagSet) cliArgs() string {
	return fmt.Sprintf(`exec blobfuse2 mount /mnt/blobfuse_mnt \
  --foreground=true --read-only --allow-other --ignore-open-flags \
  --log-level=LOG_DEBUG \
  --log-file-path=%s \
  --container-name=%q \
  --distributed-cache-discovery-endpoint=%q \
  --distributed-cache-block-size=%d \
  --distributed-cache-node-memory=%d \
  --distributed-cache-prefetch=%d \
  --distributed-cache-parallelism=%d \
  --distributed-cache-node-ttl=%d \
  -o allow_other
`,
		blobfuseDiagnosticLogPath,
		testCfg.storageContainer,
		f.discoveryEndpoint, f.blockSizeMB, f.memoryMB, f.prefetch,
		f.parallelism, f.ttlSeconds)
}

// azStorageEnv returns the container env: entries that stand in for the
// azstorage: block of the reference config.yaml. blobfuse2 reads these env
// vars directly (component/azstorage/config.go). Values come from testCfg,
// which is already resolved from CLI flags with STO_ACC_* fallback envs.
func azStorageEnv() []any {
	return []any{
		envKV("AZURE_STORAGE_ACCOUNT", testCfg.storageAccount),
		envKV("AZURE_STORAGE_ACCESS_KEY", testCfg.storageKey),
		envKV("AZURE_STORAGE_ACCOUNT_TYPE", "block"),
		envKV("AZURE_STORAGE_ACCOUNT_CONTAINER", testCfg.storageContainer),
		envKV("AZURE_STORAGE_BLOB_ENDPOINT", testCfg.storageEndpoint),
		envKV("AZURE_STORAGE_AUTH_TYPE", "key"),
	}
}

// envKV builds one entry for a k8s container env: list.
func envKV(name, value string) map[string]any {
	return map[string]any{"name": name, "value": value}
}

// newCLIPodMounter clones the reference Deployment configured in testCfg into
// a per-test Deployment (unique name, unique labels/selector) and rewrites
// the blobfuse2 container so it launches through the distributed-cache CLI
// flags with no config.yaml. Follows the same lifecycle contract as
// newTestPodMounter: Ready + read-only mount asserted before returning,
// teardown registered in Cleanup.
func newCLIPodMounter(t *testing.T, flags cliFlagSet) *podMounter {
	t.Helper()
	name := uniqueDeploymentName(t)
	m := &podMounter{
		namespace:  testCfg.podNamespace,
		deployment: name,
		selector:   "app=" + name,
		mountPath:  testCfg.podMountPath,
	}
	cloneReferenceDeploymentAsCLI(t, testCfg.podDeployment, name, flags.cliArgs())
	t.Cleanup(func() {
		m.collectBlobfuseLogs(t, "cleanup")
		m.deleteDeployment(t)
	})
	m.WaitDeploymentReady(t)
	m.assertMountReadOnly(t)
	return m
}

// cloneReferenceDeploymentAsCLI clones the reference Deployment via the
// shared fetch+rename helper in pod_mount_test.go, then:
//
//   - replaces the blobfuse2 container's args with the CLI-flag script,
//   - injects azstorage credentials as container env vars,
//   - removes the config.yaml volumeMount and its backing Secret volume,
//     so the pod truly has no config.yaml available.
//
// Everything else (image, security context, probes, log volumeMounts) is
// inherited unchanged from the reference Deployment. Metadata rewriting,
// selector fix-up, replica reset, and the final kubectl apply live in the
// shared helpers so both clone paths stay in lockstep.
func cloneReferenceDeploymentAsCLI(t *testing.T, refName, newName, args string) {
	t.Helper()
	obj := fetchAndRenameReferenceDeployment(t, refName, newName)

	spec, _ := obj["spec"].(map[string]any)
	tmpl, _ := spec["template"].(map[string]any)
	tspec, _ := tmpl["spec"].(map[string]any)
	if tspec == nil {
		t.Fatalf("cli-clone: reference deployment template has no spec")
	}
	containers, _ := tspec["containers"].([]any)
	if len(containers) == 0 {
		t.Fatalf("cli-clone: reference deployment has no containers")
	}
	container, _ := containers[0].(map[string]any)
	if container == nil {
		t.Fatalf("cli-clone: reference deployment container is not an object")
	}

	// Swap the container args and inject azstorage credentials as env.
	container["args"] = []any{args}
	container["env"] = azStorageEnv()

	// Drop the config.yaml volumeMount so /usr/share/blobfuse2/config.yaml
	// does not exist inside the container. Leaves any other mounts (e.g. the
	// blobfuse2 log emptyDir) intact.
	container["volumeMounts"] = filterOutVolumeMount(container["volumeMounts"], "blobfuse2-config")

	// Drop the backing config Secret volume so nothing on the pod
	// references a config.yaml source at all.
	tspec["volumes"] = filterOutVolume(tspec["volumes"], "blobfuse2-config")

	applyDeploymentObject(t, newName, refName, obj)
	t.Logf("cli-clone: rewrote container args + env for deployment %s (no config.yaml, azstorage via env)",
		newName)
}

// filterOutVolumeMount removes the volumeMount whose name matches. Safe to
// call on a nil or non-slice value; returns whatever the caller can assign
// back to the container spec.
func filterOutVolumeMount(v any, name string) []any {
	mounts, _ := v.([]any)
	out := make([]any, 0, len(mounts))
	for _, mnt := range mounts {
		if m, ok := mnt.(map[string]any); ok {
			if n, _ := m["name"].(string); n == name {
				continue
			}
		}
		out = append(out, mnt)
	}
	return out
}

// filterOutVolume removes the pod volume whose name matches.
func filterOutVolume(v any, name string) []any {
	vols, _ := v.([]any)
	out := make([]any, 0, len(vols))
	for _, vol := range vols {
		if m, ok := vol.(map[string]any); ok {
			if n, _ := m["name"].(string); n == name {
				continue
			}
		}
		out = append(out, vol)
	}
	return out
}
