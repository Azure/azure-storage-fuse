/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
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

package cmd

import (
	"blobfuse2/common"
	"blobfuse2/common/log"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var disableVersionCheck bool

var rootCmd = &cobra.Command{
	Use:     "blobfuse2",
	Short:   "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage.",
	Long:    "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the fuse protocol to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.",
	Version: common.Blobfuse2Version,
	Run: func(cmd *cobra.Command, args []string) {
		if !disableVersionCheck {
			VersionCheck()
		}
	},
}

// check if the version file exists in the container
func checkVersionExists(versionUrl string) bool {
	resp, err := http.Get(versionUrl)
	if err != nil {
		log.Err("checkVersionExists: error getting version file from container [%s]", err)
		return false
	}

	return resp.StatusCode != 404
}

func beginDetectNewVersion() chan interface{} {
	completed := make(chan interface{})
	stderr := os.Stderr
	go func() {
		defer close(completed)

		latestVersionUrl := common.Blobfuse2ListContainerURL + "latest/" + common.Blobfuse2Version
		isLatestVersion := checkVersionExists(latestVersionUrl)

		if !isLatestVersion {
			executablePathSegments := strings.Split(strings.Replace(os.Args[0], "\\", "/", -1), "/")
			executableName := executablePathSegments[len(executablePathSegments)-1]
			log.Info("beginDetectNewVersion: A new version of Blobfuse2 is available. Current Version=%s", common.Blobfuse2Version)
			fmt.Fprintf(stderr, "*** "+executableName+": A new version is available. Consider upgrading to latest version for bug-fixes & new features. ***\n")
			log.Info("*** " + executableName + ": A new version is available. Consider upgrading to latest version for bug-fixes & new features. ***\n")

			warningsUrl := common.Blobfuse2ListContainerURL + "securitywarnings/" + common.Blobfuse2Version
			hasWarnings := checkVersionExists(warningsUrl)

			if hasWarnings {
				warningsPage := common.BlobFuse2WarningsURL + "#" + strings.ReplaceAll(common.Blobfuse2Version, ".", "")
				fmt.Fprintf(stderr, "Vist %s to see the list of vulnerabilities assosiated with your current version (%s)\n", warningsPage, common.Blobfuse2Version)
				log.Info("Vist %s to see the list of vulnerabilities associated with your current version (%s)\n", warningsPage, common.Blobfuse2Version)
			}
		}
	}()
	return completed
}

func VersionCheck() error {
	select {
	//either wait till this routine completes or timeout if it exceeds 8 secs
	case <-beginDetectNewVersion():
	case <-time.After(8 * time.Second):
		return fmt.Errorf("unable to obtain latest version information. please check your internet connection")
	}
	return nil
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&disableVersionCheck, "disable-version-check", false, "To disable version check that is performed automatically")
}
