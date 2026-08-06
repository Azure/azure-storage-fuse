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

// ensurePodMountArgs validates pod configuration before tests run.
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
	if testCfg.dockerBin == "" {
		return fmt.Errorf("kind fault injection requires -docker-bin")
	}
	return nil
}
