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
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/spf13/cobra"
)

type VersionFilesList struct {
	XMLName xml.Name `xml:"EnumerationResults"`
	Blobs   []Blob   `xml:"Blobs>Blob"`
}

type Blob struct {
	XMLName xml.Name `xml:"Blob"`
	Name    string   `xml:"Name"`
}

var disableVersionCheck bool

var rootCmd = &cobra.Command{
	Use:               "blobfuse2",
	Short:             "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage.",
	Long:              "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the fuse protocol to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.",
	Version:           common.Blobfuse2Version,
	FlagErrorHandling: cobra.ExitOnError,
	SilenceUsage:      true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !disableVersionCheck {
			err := VersionCheck()
			if err != nil {
				return err
			}
		}
		return errors.New("missing command options\n\nDid you mean this?\n\tblobfuse2 mount\n\nRun 'blobfuse2 --help' for usage.")
	},
}

// check if the version file exists in the container
func checkVersionExists(versionUrl string) bool {
	resp, err := http.Get(versionUrl)
	if err != nil {
		log.Err("checkVersionExists: error getting version file from container [%s]", err.Error())
		return false
	}

	return resp.StatusCode != 404
}

func getRemoteVersion(req string) (string, error) {
	resp, err := http.Get(req)
	if err != nil {
		log.Err("getRemoteVersion: error listing version file from container [%s]", err.Error())
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Err("getRemoteVersion: error reading body of response [%s]", err.Error())
		return "", err
	}

	var versionList VersionFilesList
	err = xml.Unmarshal(body, &versionList)
	if err != nil {
		log.Err("getRemoteVersion: error unmarshalling xml response [%s]", err.Error())
		return "", err
	}

	if len(versionList.Blobs) != 1 {
		return "", fmt.Errorf("unable to get latest version")
	}

	versionName := strings.Split(versionList.Blobs[0].Name, "/")[1]
	return versionName, nil
}

func beginDetectNewVersion() chan interface{} {
	completed := make(chan interface{})
	stderr := os.Stderr
	go func() {
		defer close(completed)

		latestVersionUrl := common.Blobfuse2ListContainerURL + "?restype=container&comp=list&prefix=latest/"
		remoteVersion, err := getRemoteVersion(latestVersionUrl)
		if err != nil {
			log.Err("beginDetectNewVersion: error getting latest version [%s]", err.Error())
			return
		}

		local, err := common.ParseVersion(common.Blobfuse2Version)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing Blobfuse2Version [%s]", err.Error())
			return
		}

		remote, err := common.ParseVersion(remoteVersion)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing remoteVersion [%s]", err.Error())
			return
		}

		if local.OlderThan(*remote) {
			executablePathSegments := strings.Split(strings.Replace(os.Args[0], "\\", "/", -1), "/")
			executableName := executablePathSegments[len(executablePathSegments)-1]
			log.Info("beginDetectNewVersion: A new version of Blobfuse2 is available. Current Version=%s, Latest Version=%s", common.Blobfuse2Version, remoteVersion)
			fmt.Fprintf(stderr, "*** "+executableName+": A new version (%s) is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)
			log.Info("*** "+executableName+": A new version (%s) is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)

			warningsUrl := common.Blobfuse2ListContainerURL + "/securitywarnings/" + common.Blobfuse2Version
			hasWarnings := checkVersionExists(warningsUrl)

			if hasWarnings {
				warningsPage := common.BlobFuse2WarningsURL + "#" + strings.ReplaceAll(common.Blobfuse2Version, ".", "")
				fmt.Fprintf(stderr, "Visit %s to see the list of vulnerabilities associated with your current version (%s)\n", warningsPage, common.Blobfuse2Version)
				log.Warn("Vist %s to see the list of vulnerabilities associated with your current version (%s)\n", warningsPage, common.Blobfuse2Version)
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
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	return err
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&disableVersionCheck, "disable-version-check", false, "To disable version check that is performed automatically")
}
