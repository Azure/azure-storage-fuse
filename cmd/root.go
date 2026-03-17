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

package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"

	"github.com/spf13/cobra"
)

var disableVersionCheck bool

var rootCmd = &cobra.Command{
	Use:          "blobfuse2",
	Short:        "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage.",
	Long:         "Blobfuse2 is an open source project developed to provide a virtual filesystem backed by the Azure Storage. It uses the fuse protocol to communicate with the Linux FUSE kernel module, and implements the filesystem operations using the Azure Storage REST APIs.",
	Version:      common.Blobfuse2Version,
	SilenceUsage: true,
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

// checkVersionExists checks whether a file exists at the given raw GitHub URL
// by issuing an HTTP HEAD request. This is used to probe files under:
//   - release/latest/<version>           – to determine if the running version is the latest
//   - release/securitywarnings/<version> – to determine if the version has known security issues
//   - release/blockedversions/<version>  – to determine if the version is blocked from use
//
// HEAD is preferred over GET because we only need the HTTP status code, not the
// file body. raw.githubusercontent.com returns 200 when the file exists and 404
// when it does not; no authentication is required for public repos.
func checkVersionExists(fileUrl string) bool {
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: getTransport(),
	}

	req, err := http.NewRequest("HEAD", fileUrl, nil)
	if err != nil {
		log.Err("checkVersionExists: error creating request [%s]", err.Error())
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Err("checkVersionExists: error checking version file [%s]", err.Error())
		return false
	}
	defer resp.Body.Close()

	// 2xx means the file exists on the benchmarks branch.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}
	// 404 is the normal "file does not exist" case.
	if resp.StatusCode == http.StatusNotFound {
		return false
	}

	// For other status codes (e.g., 403, 5xx) log error and treat as non-existent.
	log.Err("checkVersionExists: unexpected status code [%d] for URL %s", resp.StatusCode, fileUrl)
	return false
}

// beginDetectNewVersion checks whether the current blobfuse2 version is the
// latest, has known security warnings, or has been blocked entirely.
//
// Instead of calling the GitHub REST API (which is subject to aggressive
// rate-limiting / 429 errors), we check for sentinel files served by
// raw.githubusercontent.com from the "benchmarks" branch under release/:
//
//	release/latest/<version>           – exists only for the current latest GA version
//	release/securitywarnings/<version> – exists if this version has known issues
//	release/blockedversions/<version>  – exists if this version must not be used
func beginDetectNewVersion() chan any {
	completed := make(chan any)
	stderr := os.Stderr
	go func() {
		defer close(completed)

		// Validate that the compiled-in version string is well-formed.
		_, err := common.ParseVersion(common.Blobfuse2Version)
		if err != nil {
			log.Err("beginDetectNewVersion: error parsing Blobfuse2Version [%s]", err.Error())
			completed <- err.Error()
			return
		}

		// --- Security warnings check ---
		// If a file release/securitywarnings/<version> exists, this version
		// has known issues that the user should be aware of.
		warningsUrl := common.GitHubReleaseBaseURL + "/securitywarnings/" + common.Blobfuse2Version
		hasWarnings := checkVersionExists(warningsUrl)

		if hasWarnings {
			// This version has known issues associated with it.
			// Check whether the version has been blocked by the dev team.
			blockedUrl := common.GitHubReleaseBaseURL + "/blockedversions/" + common.Blobfuse2Version
			isBlocked := checkVersionExists(blockedUrl)

			if isBlocked {
				// This version is blocked and customer shall not be allowed to use it.
				blockedPage := common.BlobFuse2BlockingURL + "#" + strings.ReplaceAll(strings.ReplaceAll(common.Blobfuse2Version, ".", ""), "~", "")
				fmt.Fprintf(stderr, "PANIC: Visit %s to see the list of known issues blocking your current version [%s]\n", blockedPage, common.Blobfuse2Version)
				log.Warn("PANIC: Visit %s to see the list of known issues blocking your current version [%s]\n", blockedPage, common.Blobfuse2Version)
				os.Exit(1)
			} else {
				// This version is not blocked but has a known-issues list which the customer should visit.
				warningsPage := common.BlobFuse2WarningsURL + "#" + strings.ReplaceAll(strings.ReplaceAll(common.Blobfuse2Version, ".", ""), "~", "")
				fmt.Fprintf(stderr, "WARNING: Visit %s to see the list of known issues associated with your current version [%s]\n", warningsPage, common.Blobfuse2Version)
				log.Warn("WARNING: Visit %s to see the list of known issues associated with your current version [%s]\n", warningsPage, common.Blobfuse2Version)
			}
		}

		// --- Latest-version check ---
		// If release/latest/<currentVersion> does NOT exist the running
		// version is outdated and a newer release is available.
		latestUrl := common.GitHubReleaseBaseURL + "/latest/" + common.Blobfuse2Version
		isLatest := checkVersionExists(latestUrl)
		if !isLatest {
			executablePathSegments := strings.Split(strings.ReplaceAll(os.Args[0], "\\", "/"), "/")
			executableName := executablePathSegments[len(executablePathSegments)-1]
			log.Info("beginDetectNewVersion: A new version of Blobfuse2 is available. Current Version=%s", common.Blobfuse2Version)
			fmt.Fprintf(stderr, "*** %s: A new version is available. Current version [%s] is outdated. Consider upgrading to the latest version for bug-fixes & new features. ***\n", executableName, common.Blobfuse2Version)

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
		if slices.Contains(ignoreCmds, cmdArgs[0]) {
			return true
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
				opts := strings.SplitSeq(cmdArgs[i], ",")
				for o := range opts {
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

func getTransport() *http.Transport {
	// Prefer cloning the default transport so we inherit standard
	// proxy/TLS settings (including HTTP(S)_PROXY and NO_PROXY).
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		cp := dt.Clone()
		cp.MaxIdleConns = 10
		cp.IdleConnTimeout = 30 * time.Second
		cp.DisableCompression = true // GitHub API responses are small
		cp.DisableKeepAlives = false // Connections are reused
		return cp
	}

	// Fallback: construct a transport that at least respects proxy env vars.
	return &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,  // GitHub API responses are small
		DisableKeepAlives:  false, // Connections are reused
	}
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
