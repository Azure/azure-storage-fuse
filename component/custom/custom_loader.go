package custom

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/external"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func init() {
	// Environment variable for directory containing the plugin .so files.
	pluginDirPath := os.Getenv("BLOBFUSE_PLUGIN_DIRECTORY")
	if pluginDirPath == "" {
		return
	}

	// Please provide the plugin names as a colon-separated list of `.so` files located in the specified plugin directory. For example:
	// BLOBFUSE_PLUGIN_NAMES=plugin1.so:plugin2.so:plugin3.so
	pluginFilesPath := os.Getenv("BLOBFUSE_PLUGIN_NAMES")

	var pluginFiles []string
	if pluginFilesPath != "" {
		for _, file := range strings.Split(pluginFilesPath, ":") {
			if strings.HasSuffix(file, ".so") {
				pluginFiles = append(pluginFiles, filepath.Join(pluginDirPath, file))
			}
		}
	} else {
		files, err := os.ReadDir(pluginDirPath)
		if err != nil {
			log.Err("custom_loader::Error reading plugin directory:", err)
			return
		}
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".so") {
				pluginFiles = append(pluginFiles, filepath.Join(pluginDirPath, file.Name()))
			}
		}
	}

	for _, file := range pluginFiles {
		log.Info("custom_loader::Opening plugin:", file)
		p, err := plugin.Open(file)
		if err != nil {
			fmt.Println("custom_loader::Error opening plugin:", err.Error())
		}
		fmt.Println("custom_loader::Plugin opened successfully")

		getExternalComponentFunc, err := p.Lookup("GetExternalComponent")
		if err != nil {
			log.Info("custom_loader::Error looking up GetExternalComponent function:", err.Error())
			os.Exit(1)
		}
		getExternalComponent, ok := getExternalComponentFunc.(func() (string, func() external.Component))
		if !ok {
			fmt.Println("custom_loader::Error casting GetExternalComponent function")
			os.Exit(1)
		}
		compName, initExternalComponent := getExternalComponent()
		internal.AddComponent(compName, initExternalComponent)
	}
}
