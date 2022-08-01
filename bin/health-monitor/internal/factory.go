package internal

import (
	"fmt"
)

type NewMonitor func() Monitor

var registeredMonitors map[string]NewMonitor

func GetMonitor(name string) (Monitor, error) {
	comp, isPresent := registeredMonitors[name]
	if isPresent {
		return comp(), nil
	} else {
		fmt.Printf("Factory::GetMonitor : monitor %s is not registered", name)
		return nil, fmt.Errorf("monitor %s not registered", name)
	}
}

func AddMonitor(name string, init NewMonitor) {
	registeredMonitors[name] = init
}

func init() {
	fmt.Println("Inside factory")
	registeredMonitors = make(map[string]NewMonitor)
}
