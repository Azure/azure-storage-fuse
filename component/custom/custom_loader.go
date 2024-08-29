package custom

import (
	"path/filepath"
	"plugin"
)

func init() {
	println("Initializing custom component")
	files, err := filepath.Glob("component/custom/*.so")
	if err != nil {
		println("Error finding plugins:", err.Error())
		return
	}
	for _, file := range files {
		println("Opening plugin:", file)
		p, err := plugin.Open(file)
		if err != nil {
			println("Error opening plugin:", err.Error())
			return
		}
		println("Plugin opened successfully")

		// Lookup the init function
		initFunc, err := p.Lookup("InitPlugin")
		if err != nil {
			println("Error looking up init function:", err.Error())
			return
		}

		// Assert the symbol to the correct type and invoke it
		if initFunc, ok := initFunc.(func()); ok {
			initFunc()
			println("Plugin init function invoked successfully")
		} else {
			println("Symbol has wrong type")
		}
	}
}
