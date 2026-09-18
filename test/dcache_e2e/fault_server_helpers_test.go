//go:build !unittest
// +build !unittest

// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package dcache_e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	faultServerContainer = "dcache-fault-server"
	faultServerPort      = 9065
	faultAzureProxyPort  = 10000
)

// newFaultPodMounter clones the reference deployment, removes its YAML
// configuration, and runs blobfuse2 against a scripted cache-server sidecar.
// When failAzureGET is true, azstorage is routed through the same sidecar,
// which forwards metadata requests but rejects data GETs.
func newFaultPodMounter(t *testing.T, mode string, failAzureGET bool) *podMounter {
	t.Helper()
	if failAzureGET && testCfg.storageEndpoint == "" {
		t.Fatal("fault-clone: Azure GET failure mode requires -storage-endpoint")
	}

	name := uniqueDeploymentName(t)
	m := &podMounter{
		namespace:  testCfg.podNamespace,
		deployment: name,
		selector:   "app=" + name,
		mountPath:  testCfg.podMountPath,
	}

	obj := fetchAndRenameReferenceDeployment(t, testCfg.podDeployment, name)
	spec, _ := obj["spec"].(map[string]any)
	tmpl, _ := spec["template"].(map[string]any)
	podSpec, _ := tmpl["spec"].(map[string]any)
	if podSpec == nil {
		t.Fatal("fault-clone: reference deployment template has no spec")
	}
	containers, _ := podSpec["containers"].([]any)
	if len(containers) == 0 {
		t.Fatal("fault-clone: reference deployment has no containers")
	}
	blobfuse, _ := containers[0].(map[string]any)
	if blobfuse == nil {
		t.Fatal("fault-clone: reference blobfuse2 container is not an object")
	}
	image, _ := blobfuse["image"].(string)
	if image == "" {
		t.Fatal("fault-clone: reference blobfuse2 container has no image")
	}

	endpoint := testCfg.storageEndpoint
	useHTTPS := true
	if failAzureGET {
		endpoint = fmt.Sprintf("http://127.0.0.1:%d", faultAzureProxyPort)
		useHTTPS = false
	}

	blobfuse["args"] = []any{faultMountArgs(useHTTPS)}
	blobfuse["env"] = faultStorageEnv(endpoint)
	blobfuse["volumeMounts"] = filterOutVolumeMount(blobfuse["volumeMounts"], "blobfuse2-config")
	podSpec["volumes"] = filterOutVolume(podSpec["volumes"], "blobfuse2-config")

	sidecarArgs := []any{
		"-mode=" + mode,
		fmt.Sprintf("-listen=:%d", faultServerPort),
	}
	if failAzureGET {
		sidecarArgs = append(sidecarArgs,
			fmt.Sprintf("-azure-listen=:%d", faultAzureProxyPort),
			"-azure-upstream="+testCfg.storageEndpoint,
		)
	}
	podSpec["containers"] = append(containers, map[string]any{
		"name":            faultServerContainer,
		"image":           image,
		"imagePullPolicy": "IfNotPresent",
		"command":         []any{"/usr/local/bin/dcache-fault-server"},
		"args":            sidecarArgs,
	})

	applyDeploymentObject(t, name, testCfg.podDeployment, obj)
	t.Cleanup(func() {
		m.collectBlobfuseLogs(t, "cleanup")
		if logs, err := m.faultServerLogs(); err == nil {
			t.Logf("fault-server logs:\n%s", logs)
		}
		m.deleteDeployment(t)
	})
	m.WaitDeploymentReady(t)
	m.waitFaultServerReady(t)
	m.assertMountReadOnly(t)
	return m
}

func faultMountArgs(useHTTPS bool) string {
	waitForSidecar := fmt.Sprintf(
		"until (echo > /dev/tcp/127.0.0.1/%d) 2>/dev/null; do sleep 0.1; done",
		faultServerPort,
	)
	if !useHTTPS {
		waitForSidecar += fmt.Sprintf(
			"\nuntil (echo > /dev/tcp/127.0.0.1/%d) 2>/dev/null; do sleep 0.1; done",
			faultAzureProxyPort,
		)
	}

	return fmt.Sprintf(`%s
exec blobfuse2 mount /mnt/blobfuse_mnt \
  --foreground=true --read-only --allow-other --ignore-open-flags \
  --use-https=%t \
  --log-level=LOG_DEBUG \
  --log-file-path=%s \
  --container-name=%q \
  --distributed-cache-block-size=16 \
  --distributed-cache-node-memory=4096 \
  --distributed-cache-prefetch=32 \
  --distributed-cache-parallelism=128 \
  --distributed-cache-node-ttl=0 \
  -o allow_other
`, waitForSidecar, useHTTPS, blobfuseDiagnosticLogPath, testCfg.storageContainer)
}

func faultStorageEnv(endpoint string) []any {
	return []any{
		envKV("AZURE_STORAGE_ACCOUNT", testCfg.storageAccount),
		envKV("AZURE_STORAGE_ACCESS_KEY", testCfg.storageKey),
		envKV("AZURE_STORAGE_ACCOUNT_TYPE", "block"),
		envKV("AZURE_STORAGE_ACCOUNT_CONTAINER", testCfg.storageContainer),
		envKV("AZURE_STORAGE_BLOB_ENDPOINT", endpoint),
		envKV("AZURE_STORAGE_AUTH_TYPE", "key"),
		envKV("DISTRIBUTED_CACHE_SERVER_LIST", fmt.Sprintf("127.0.0.1:%d", faultServerPort)),
	}
}

func (m *podMounter) faultServerLogs() (string, error) {
	pods, err := m.listLivePodsE()
	if err != nil {
		return "", err
	}
	if len(pods) == 0 {
		return "", fmt.Errorf("no live pod for %s", m.deployment)
	}
	out, err := exec.Command(testCfg.kubectlBin,
		"-n", m.namespace,
		"logs", pods[0],
		"-c", faultServerContainer,
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kubectl logs %s: %w (out: %s)",
			pods[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (m *podMounter) waitFaultServerReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		logs, err := m.faultServerLogs()
		if err == nil && strings.Contains(logs, "event=ready") {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("fault-server did not report ready within 30s")
}

func assertFaultServerActivity(t *testing.T, m *podMounter, wantAzureFailure bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var logs string
	var err error
	for {
		logs, err = m.faultServerLogs()
		if err == nil && strings.Contains(logs, "event=download") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fault-server logs unavailable or missing download activity: %v\n%s", err, logs)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(logs, "event=download") {
		t.Fatalf("fault-server received no cache download request; test did not exercise L2\n%s", logs)
	}
	time.Sleep(3 * time.Second)
	logs, err = m.faultServerLogs()
	if err != nil {
		t.Fatalf("fault-server logs: %v", err)
	}
	if strings.Contains(logs, "event=upload ") {
		t.Fatalf("fault-server received an unexpected cache upload after an error\n%s", logs)
	}
	if wantAzureFailure && !strings.Contains(logs, "event=azure_get_failed") {
		t.Fatalf("Azure proxy did not reject a data GET; test did not exercise Azure read failure\n%s", logs)
	}
}
