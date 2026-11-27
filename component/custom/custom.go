// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package custom

import (
	"fmt"
	"os"
	"plugin"
	"strings"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/exported"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func initializePlugins() error {

	// Environment variable which expects file path as a colon-separated list of `.so` files.
	// Example BLOBFUSE_PLUGIN_PATH="/path/to/plugin1.so:/path/to/plugin2.so"
	pluginFilesPath := os.Getenv("BLOBFUSE_PLUGIN_PATH")
	if pluginFilesPath == "" {
		log.Debug("initializePlugins: No plugins to load.")
		return nil
	}

	pluginFiles := strings.SplitSeq(pluginFilesPath, ":")

	for file := range pluginFiles {
		if !strings.HasSuffix(file, ".so") {
			log.Err("initializePlugins: Invalid plugin file extension: %s", file)
			continue
		}
		log.Info("initializePlugins: loading plugin %s", file)
		startTime := time.Now()
		p, err := plugin.Open(file)
		if err != nil {
			log.Err("initializePlugins: Error opening plugin %s: %s", file, err.Error())
			return fmt.Errorf("error opening plugin %s: %s", file, err.Error())
		}

		getExternalComponentFunc, err := p.Lookup("GetExternalComponent")
		if err != nil {
			log.Err("initializePlugins: GetExternalComponent function lookup error in plugin %s: %s", file, err.Error())
			return fmt.Errorf("GetExternalComponent function lookup error in plugin %s: %s", file, err.Error())
		}

		getExternalComponent, ok := getExternalComponentFunc.(func() (string, func() exported.Component))
		if !ok {
			log.Err("initializePlugins: GetExternalComponent function in %s has some incorrect definition", file)
			return fmt.Errorf("GetExternalComponent function in %s has some incorrect definition", file)
		}

		compName, initExternalComponent := getExternalComponent()
		internal.AddComponent(compName, initExternalComponent)
		log.Info("initializePlugins: Plugin %s loaded in %s", file, time.Since(startTime))
	}
	return nil
}

func init() {
	err := initializePlugins()
	if err != nil {
		log.Err("custom::init : Error initializing plugins: %s", err.Error())
		fmt.Printf("failed to initialize plugin: %s\n", err.Error())
		os.Exit(1)
	}
}
