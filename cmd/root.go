/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2024 Microsoft Corporation. All rights reserved.
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
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/spf13/cobra"
)

type VersionFilesList struct {
	Version string `xml:"latest"`
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
		return errors.New("missing command options\n\nDid you mean this?\n\tblobfuse2 mount\n\nRun 'blobfuse2 --help' for usage")
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

// getRemoteVersion : From public container get the latest blobfuse version
func getRemoteVersion(req string) (string, error) {
	resp, err := http.Get(req)
	if err != nil {
		log.Err("getRemoteVersion: error listing version file from container [%s]", err.Error())
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Err("getRemoteVersion: error reading body of response [%s]", err.Error())
		return "", err
	}

	if len(body) > 50 {
		log.Err("getRemoteVersion: something suspicious in the contents from remote verison")
		return "", fmt.Errorf("unable to get latest version")
	}

	var versionList VersionFilesList
	err = xml.Unmarshal(body, &versionList)
	if err != nil {
		log.Err("getRemoteVersion: error unmarshalling xml response [%s]", err.Error())
		return "", err
	}

	if len(versionList.Version) < 5 || len(versionList.Version) > 20 {
		return "", fmt.Errorf("unable to get latest version")
	}

	versionName := versionList.Version
	return versionName, nil
}

// beginDetectNewVersion : Get latest release version and compare if user needs an upgrade or not
func beginDetectNewVersion() chan interface{} {
	completed := make(chan interface{})
	stderr := os.Stderr
	go func() {
		defer close(completed)

		latestVersionUrl := common.Blobfuse2ListContainerURL + "/latest/index.xml"
		remoteVersion, err := getRemoteVersion(latestVersionUrl)
		if err != nil {
			log.Err("beginDetectNewVersion: error getting latest version [%s]", err.Error())
			if strings.Contains(err.Error(), "no such host") {
				log.Err("beginDetectNewVersion: check your network connection and proxy settings")
			}
			completed <- err.Error()
			return
		}

		local, err := common.ParseVersion(common.Blobfuse2Version)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing Blobfuse2Version [%s]", err.Error())
			completed <- err.Error()
			return
		}

		remote, err := common.ParseVersion(remoteVersion)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing remoteVersion [%s]", err.Error())
			completed <- err.Error()
			return
		}

		warningsUrl := common.Blobfuse2ListContainerURL + "/securitywarnings/" + common.Blobfuse2Version
		hasWarnings := checkVersionExists(warningsUrl)

		if hasWarnings {
			// This version has known issues associated with it
			// Check whether the version has been blocked by the dev team or not.
			blockedVersions := common.Blobfuse2ListContainerURL + "/blockedversions/" + common.Blobfuse2Version
			isBlocked := checkVersionExists(blockedVersions)

			if isBlocked {
				// This version is blocked and customer shall not be allowed to use this.
				blockedPage := common.BlobFuse2BlockingURL + "#" + strings.ReplaceAll(strings.ReplaceAll(common.Blobfuse2Version, ".", ""), "~", "")
				fmt.Fprintf(stderr, "PANIC: Visit %s to see the list of known issues blocking your current version [%s]\n", blockedPage, common.Blobfuse2Version)
				log.Warn("PANIC: Visit %s to see the list of known issues blocking your current version [%s]\n", blockedPage, common.Blobfuse2Version)
				os.Exit(1)
			} else {
				// This version is not blocked but has know issues list which customer shall visit.
				warningsPage := common.BlobFuse2WarningsURL + "#" + strings.ReplaceAll(strings.ReplaceAll(common.Blobfuse2Version, ".", ""), "~", "")
				fmt.Fprintf(stderr, "WARNING: Visit %s to see the list of known issues associated with your current version [%s]\n", warningsPage, common.Blobfuse2Version)
				log.Warn("WARNING: Visit %s to see the list of known issues associated with your current version [%s]\n", warningsPage, common.Blobfuse2Version)
			}
		}

		if local.OlderThan(*remote) {
			executablePathSegments := strings.Split(strings.Replace(os.Args[0], "\\", "/", -1), "/")
			executableName := executablePathSegments[len(executablePathSegments)-1]
			log.Info("beginDetectNewVersion: A new version of Blobfuse2 is available. Current Version=%s, Latest Version=%s", common.Blobfuse2Version, remoteVersion)
			fmt.Fprintf(stderr, "*** "+executableName+": A new version [%s] is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)
			log.Info("*** "+executableName+": A new version [%s] is available. Consider upgrading to latest version for bug-fixes & new features. ***\n", remoteVersion)

			completed <- "A new version of Blobfuse2 is available"
		}
	}()
	return completed
}

// VersionCheck : Start version check and wait for 8 seconds to complete otherwise just timeout and move on
func VersionCheck() error {
	select {
	//either wait till this routine completes or timeout if it exceeds 8 secs
	case <-beginDetectNewVersion():
	case <-time.After(8 * time.Second):
		return fmt.Errorf("unable to obtain latest version information. please check your internet connection")
	}
	return nil
}

// ignoreCommand : There are command implicitly added by cobra itself, while parsing we need to ignore these commands
func ignoreCommand(cmdArgs []string) bool {
	ignoreCmds := []string{"completion", "help"}
	if len(cmdArgs) > 0 {
		for _, c := range ignoreCmds {
			if c == cmdArgs[0] {
				return true
			}
		}
	}
	return false
}

// parseArgs : Depending upon inputs are coming from /etc/fstab or CLI, parameter style may vary.
// -- /etc/fstab example : blobfuse2 mount <dir> -o suid,nodev,--config-file=config.yaml,--use-adls=true,allow_other
// -- cli command        : blobfuse2 mount <dir> -o suid,nodev --config-file=config.yaml --use-adls=true -o allow_other
// -- As we need to support both the ways, here we convert the /etc/fstab style (comma separated list) to standard cli ways
func parseArgs(cmdArgs []string) []string {
	// Ignore binary name, rest all are arguments to blobfuse2
	cmdArgs = cmdArgs[1:]

	cmd, _, err := rootCmd.Find(cmdArgs)
	if err != nil && cmd == rootCmd && !ignoreCommand(cmdArgs) {
		/* /etc/fstab has a standard format and it goes like "<binary> <mount point> <type> <options>"
		 * as here we can not give any subcommand like "blobfuse2 mount" (giving this will assume mount is mount point)
		 * we need to assume 'mount' being default sub command.
		 * To do so, we just ignore the implicit commands handled by cobra and then try to check if input matches any of
		 * our subcommands or not. If not, we assume user has executed mount command without specifying mount subcommand
		 * so inject mount command in the cli options so that rest of the handling just assumes user gave mount subcommand.
		 */
		cmdArgs = append([]string{"mount"}, cmdArgs...)
	}

	// Check for /etc/fstab style inputs
	args := make([]string, 0)
	for i := 0; i < len(cmdArgs); i++ {
		// /etc/fstab will give everything in comma separated list with -o option
		if cmdArgs[i] == "-o" {
			i++
			if i < len(cmdArgs) {
				bfuseArgs := make([]string, 0)
				lfuseArgs := make([]string, 0)

				// Check if ',' exists in arguments or not. If so we assume it might be coming from /etc/fstab
				opts := strings.Split(cmdArgs[i], ",")
				for _, o := range opts {
					// If we got comma separated list then all blobfuse specific options needs to be extracted out
					//  as those shall not be part of -o list which for us means libfuse options
					if strings.HasPrefix(o, "--") {
						bfuseArgs = append(bfuseArgs, o)
					} else {
						lfuseArgs = append(lfuseArgs, o)
					}
				}

				// Extract and add libfuse options with -o
				if len(lfuseArgs) > 0 {
					args = append(args, "-o", strings.Join(lfuseArgs, ","))
				}

				// Extract and add blobfuse specific options sepratly
				if len(bfuseArgs) > 0 {
					args = append(args, bfuseArgs...)
				}
			}
		} else {
			// If any option is without -o then keep it as is (assuming its directly from cli)
			args = append(args, cmdArgs[i])
		}
	}

	return args
}

// Execute : Actual command execution starts from here
func Execute() error {
	parsedArgs := parseArgs(os.Args)
	rootCmd.SetArgs(parsedArgs)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	return err
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&disableVersionCheck, "disable-version-check", false, "To disable version check that is performed automatically")
}
