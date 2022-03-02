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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type VNextJson struct {
	Blobfuse2        string              `json:"blobfuse2"`
	SecurityWarnings map[string][]string `json:"securityWarnings"`
}

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

func beginDetectNewVersion() chan interface{} {
	completed := make(chan interface{})
	stderr := os.Stderr
	go func() {
		defer close(completed)
		resp, err := http.Get(common.Blobfuse2NextVersionURL)
		if err != nil {
			log.Err("beginDetectNewVersion: error getting version txt from container [%s]", err)
			return
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Err("beginDetectNewVersion: error reading body of txt result [%s]", err)
			return
		}

		var vJson VNextJson
		err = json.Unmarshal(body, &vJson)
		if err != nil {
			log.Err("beginDetectNewVersion: error unmarshalling VNextJson")
			return
		}

		remoteVersion := vJson.Blobfuse2
		//Pick only first line in case future modifications to the txt adds additional lines
		remoteVersion = strings.Split(remoteVersion, "\n")[0]

		local, err := common.ParseVersion(common.Blobfuse2Version)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing Blobfuse2Version [%s]", err)
			return
		}

		remote, err := common.ParseVersion(remoteVersion)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing remoteVersion [%s]", err)
			return
		}

		if local.OlderThan(*remote) {
			executablePathSegments := strings.Split(strings.Replace(os.Args[0], "\\", "/", -1), "/")
			executableName := executablePathSegments[len(executablePathSegments)-1]
			log.Info("beginDetectNewVersion: A new version of Blobfuse2 is available. Current Version=%s, Latest Version=%s", common.Blobfuse2Version, remoteVersion)
			fmt.Fprintf(stderr, "*** "+executableName+": A new version (%s) is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)
			log.Info("*** "+executableName+": A new version (%s) is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)

			_, isPresent := vJson.SecurityWarnings[common.Blobfuse2Version]
			if isPresent {
				hasWarning := false
				ctr := 1
				for _, msg := range vJson.SecurityWarnings[common.Blobfuse2Version] {
					msg = strings.TrimSpace(msg)
					if len(msg) > 0 {
						if !hasWarning {
							fmt.Fprintf(stderr, "The following vulnerabilities were detected in your current version (%s):\n", common.Blobfuse2Version)
							log.Info("The following vulnerabilities were detected in your current version (%s):\n", common.Blobfuse2Version)
							hasWarning = true
						}
						fmt.Fprintf(stderr, "%v. %s\n", ctr, msg)
						log.Info("%v. %s\n", ctr, msg)
						ctr++
					}
				}
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
