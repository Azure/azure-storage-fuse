/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
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
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Azure/azure-storage-fuse/v2/component/distributed_cache"
	"github.com/spf13/cobra"
)

var dcacheCmd = &cobra.Command{
	Use:               "dcache",
	Short:             "Manage distributed cache",
	Long:              "Manage distributed cache",
	Args:              cobra.ExactArgs(1),
	Hidden:            true,
	FlagErrorHandling: cobra.ExitOnError,
	RunE: func(cmd *cobra.Command, args []string) error {

		listNodes()
		return nil
	},
}

func listNodes() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	// Print header
	fmt.Fprintln(w, "Node ID\tIP\tHostname\tTotal (Bytes)\tUsed (Bytes)\tTotal (GB)\tUsed (GB)")
	for nodeID, peer := range distributed_cache.PeersByNodeId {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%d\t%d\t%d\t%d\n",
			nodeID,
			peer.IPAddr,
			peer.Hostname,
			peer.TotalSpace,
			peer.UsedSpace,
			peer.TotalSpace/1024/1024/1024,
			peer.UsedSpace/1024/1024/1024,
		)
	}
	w.Flush()
}

func init() {
	rootCmd.AddCommand(dcacheCmd)

}
