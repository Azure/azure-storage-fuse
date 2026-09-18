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

   Copyright © 2026 Microsoft Corporation. All rights reserved.
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
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestDistroSmoke_Runtime(t *testing.T) {
	if testCfg.runtimeDistro == "" && testCfg.runtimeVersion == "" && testCfg.runtimeArch == "" {
		t.Skip("runtime identity assertions are enabled by the distro matrix")
	}
	if testCfg.runtimeDistro == "" || testCfg.runtimeVersion == "" || testCfg.runtimeArch == "" {
		t.Fatal("runtime identity requires -runtime-distro, -runtime-version, and -runtime-arch")
	}

	m := newTestPodMounter(t)
	pod := m.resolvePod(t)

	osRelease := execInPod(t, m.namespace, pod, "cat", "/etc/os-release")
	fields := parseOSRelease(osRelease)
	if got := fields["ID"]; got != testCfg.runtimeDistro {
		t.Errorf("runtime distro: got ID=%q, want %q", got, testCfg.runtimeDistro)
	}
	if got := fields["VERSION_ID"]; !strings.HasPrefix(got, testCfg.runtimeVersion) {
		t.Errorf("runtime version: got VERSION_ID=%q, want prefix %q", got, testCfg.runtimeVersion)
	}

	gotArch := normalizeRuntimeArch(strings.TrimSpace(execInPod(t, m.namespace, pod, "uname", "-m")))
	if gotArch != testCfg.runtimeArch {
		t.Errorf("runtime architecture: got %q, want %q", gotArch, testCfg.runtimeArch)
	}

	version := strings.TrimSpace(execInPod(t, m.namespace, pod, "blobfuse2", "--version"))
	if version == "" {
		t.Error("blobfuse2 --version returned empty output")
	}

	linkage := execInPod(t, m.namespace, pod, "ldd", "/usr/local/bin/blobfuse2")
	if !strings.Contains(linkage, "libfuse3") {
		t.Errorf("blobfuse2 linkage does not include libfuse3:\n%s", linkage)
	}
	if strings.Contains(linkage, "not found") {
		t.Errorf("blobfuse2 has unresolved runtime libraries:\n%s", linkage)
	}

	t.Logf("verified distributed-cache runtime: distro=%s version=%s arch=%s binary=%s",
		fields["ID"], fields["VERSION_ID"], gotArch, version)
}

func execInPod(t *testing.T, namespace, pod string, command ...string) string {
	t.Helper()
	args := []string{"-n", namespace, "exec", pod, "--"}
	args = append(args, command...)
	out, err := exec.Command(testCfg.kubectlBin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("pod: execute %q on %s: %v (out: %s)",
			strings.Join(command, " "), pod, err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

func parseOSRelease(contents string) map[string]string {
	fields := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return fields
}

func normalizeRuntimeArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return fmt.Sprintf("unsupported:%s", arch)
	}
}
