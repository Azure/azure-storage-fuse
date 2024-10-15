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

package custom

import (
	"fmt"
	"os"
	"plugin"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/exported"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func initializePlugins() error {

	// Environment variable which expects file path as a colon-separated list of `.so` files.
	// Example BLOBFUSE_PLUGIN_PATH="/path/to/plugin1.so:/path/to/plugin2.so"
	pluginFilesPath := os.Getenv("BLOBFUSE_PLUGIN_PATH")
	if pluginFilesPath == "" {
		return nil
	}

	pluginFiles := strings.Split(pluginFilesPath, ":")

	for _, file := range pluginFiles {
		if strings.HasSuffix(file, ".so") {
			p, err := plugin.Open(file)
			if err != nil {
				return fmt.Errorf("error opening plugin %s: %s", file, err.Error())
			}

			getExternalComponentFunc, err := p.Lookup("GetExternalComponent")
			if err != nil {
				return fmt.Errorf("error looking up GetExternalComponent function in %s: %s", file, err.Error())
			}

			getExternalComponent, ok := getExternalComponentFunc.(func() (string, func() exported.Component))
			if !ok {
				return fmt.Errorf("error casting GetExternalComponent function in %s", file)
			}

			compName, initExternalComponent := getExternalComponent()
			internal.AddComponent(compName, initExternalComponent)
		} else {
			return fmt.Errorf("invalid plugin file extension: %s", file)
		}
	}
	return nil
}

func init() {
	err := initializePlugins()
	if err != nil {
		log.Err("custom::Error initializing plugins: %s", err.Error())
		os.Exit(1)
	}
}
