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
	Blobfuse2 string `json:"blobfuse2"`
}

var disableVersionCheck bool

var rootCmd = &cobra.Command{
	Use:     "blobfuse2",
	Short:   "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage.",
	Long:    "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the fuse protocol to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.",
	Version: common.Blobfuse2Version,
	Run: func(cmd *cobra.Command, args []string) {
		if !disableVersionCheck {
			select {
			//either wait till this routine completes or timeout if it exceeds 8 secs
			case <-beginDetectNewVersion():
			case <-time.After(time.Second * 8):
			}
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
			log.Info("beginDetectNewVersion: A newer version of Blobfuse2 is available. Current Version=%s, Latest Version=%s", common.Blobfuse2Version, remoteVersion)
			fmt.Fprintf(stderr, "\n\t*** "+executableName+": A newer version %s is available. Kindly update to latest version for bug-fixes & new features. ***\n\n", remoteVersion)
		}
	}()
	return completed
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&disableVersionCheck, "disable-version-check", false, "To disable version check that is performed automatically")
}
