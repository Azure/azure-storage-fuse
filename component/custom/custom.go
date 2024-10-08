package custom

import (
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/exported"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

func init() {
	// Environment variable for directory containing the plugin .so files.
	pluginDirPath := os.Getenv("BLOBFUSE_PLUGIN_DIRECTORY")
	if pluginDirPath == "" {
		return
	}

	// Environment variable which expects file names as a colon-separated list of `.so` files located in
	// the specified plugin directory. For example: BLOBFUSE_PLUGIN_NAMES=plugin1.so:plugin2.so:plugin3.so
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
			log.Err("custom::Error reading plugin directory: %s", err.Error())
			return
		}
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".so") {
				pluginFiles = append(pluginFiles, filepath.Join(pluginDirPath, file.Name()))
			}
		}
	}

	for _, file := range pluginFiles {
		log.Info("custom::Opening plugin: %s", file)
		p, err := plugin.Open(file)
		if err != nil {
			log.Err("custom::Error opening plugin: %s", err.Error())
			os.Exit(1)
		}
		log.Info("custom::Plugin opened successfully")

		getExternalComponentFunc, err := p.Lookup("GetExternalComponent")
		if err != nil {
			log.Err("custom::Error looking up GetExternalComponent function: %s", err.Error())
			os.Exit(1)
		}
		getExternalComponent, ok := getExternalComponentFunc.(func() (string, func() exported.Component))
		if !ok {
			log.Err("custom::Error casting GetExternalComponent function")
			os.Exit(1)
		}
		compName, initExternalComponent := getExternalComponent()
		internal.AddComponent(compName, initExternalComponent)
	}
}
