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
