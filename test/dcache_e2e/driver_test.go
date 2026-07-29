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
*/

package dcache_e2e

import "fmt"

// activeMounter is the pod-driver mount instance used by every test that
// needs to talk to the mount. Kept as a package-global (rather than
// threaded through every test) to keep call sites tight — each test still
// receives its own *testing.T.
//
// The suite is pod-only for now: dist_cache's discovery-url / k8s-service
// code paths only resolve inside a cluster, and standing up a second
// (host-driver) path just to run the same assertions off-cluster would
// double the surface area of this first E2E PR without adding coverage.
var activeMounter *podMounter

// ensurePodMountArgs validates that the pod driver has all the inputs it
// needs before any test runs. Failing here (rather than deep inside the
// first kubectl call) turns a misconfigured pipeline into a clear TestMain
// error instead of a stack trace from the first test.
func ensurePodMountArgs() error {
	if testCfg.podNamespace == "" {
		return fmt.Errorf("pod driver requires -pod-namespace")
	}
	if testCfg.podDeployment == "" {
		return fmt.Errorf("pod driver requires -pod-deployment")
	}
	if testCfg.podSelector == "" {
		return fmt.Errorf("pod driver requires -pod-selector")
	}
	if testCfg.podMountPath == "" {
		return fmt.Errorf("pod driver requires -pod-mount-path")
	}
	if testCfg.kubectlBin == "" {
		return fmt.Errorf("pod driver requires -kubectl-bin")
	}
	return nil
}
