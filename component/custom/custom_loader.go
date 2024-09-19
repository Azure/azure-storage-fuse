package custom

import (
	"os"
	"plugin"
	"strings"
)

func init() {
	pluginPath := os.Getenv("PLUGIN_PATH")
	if pluginPath == "" {
		return
	}

	// environment variable can contain multiple paths separated by colon.
	paths := strings.Split(pluginPath, ":")

	for _, file := range paths {
		println("custom_loader::Opening plugin:", file)
		p, err := plugin.Open(file)
		if err != nil {
			println("custom_loader::Error opening plugin:", err.Error())
			return
		}
		println("custom_loader::Plugin opened successfully")

		initFunc, err := p.Lookup("InitPlugin")
		if err != nil {
			println("custom_loader::Error looking up init function:", err.Error())
			return
		}

		if initFunc, ok := initFunc.(func()); ok {
			initFunc()
			println("custom_loader::Plugin init function invoked successfully")
		} else {
			println("custom_loader::Symbol has wrong type")
		}
	}
}
