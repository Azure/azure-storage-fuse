// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package internal

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

// Pipeline: Base pipeline structure holding list of components deployed along with the head of pipeline
type Pipeline struct {
	components []Component
	Header     Component
}

// NewComponent : Function that all components have to register to allow their instantiation
type NewComponent func() Component

// Map holding all possible components along with their respective constructors
var registeredComponents map[string]NewComponent

func GetComponent(name string) Component {
	compInit, ok := registeredComponents[name]
	if ok {
		return compInit()
	}
	return nil
}

// NewPipeline : Using a list of strings holding name of components, create and configure the component objects
func NewPipeline(components []string, isParent bool) (*Pipeline, error) {
	comps := make([]Component, 0)
	lastPriority := EComponentPriority.Producer()
	for _, name := range components {
		if name == "stream" {
			common.IsStream = true
			name = "block_cache"
		}
		//  Search component exists in our registered map or not
		compInit, ok := registeredComponents[name]
		if ok {
			// Call the constructor method registered by the component
			comp := compInit()

			// request component to parse and validate config of its interest
			err := comp.Configure(isParent)
			if err != nil {
				log.Err("Pipeline: error creating pipeline component %s [%s]", comp.Name(), err)
				return nil, err
			}

			if comp.Priority() > lastPriority {
				log.Err("Pipeline::NewPipeline : Invalid Component order [priority of %s higher than above components]", comp.Name())
				return nil, fmt.Errorf("config error in Pipeline [component %s is out of order]", name)
			} else {
				lastPriority = comp.Priority()
			}

			// store the configured object in list of components
			comps = append(comps, comp)
		} else {
			log.Err("Pipeline: error [component %s not registered]", name)
			return nil, fmt.Errorf("config error in Pipeline [component %s not registered]", name)
		}

	}

	// Create pipeline structure holding list of all component objects requested by config file
	return &Pipeline{
		components: comps,
	}, nil
}

// Create : Use the initialized objects to form a pipeline by registering next component to each component
func (p *Pipeline) Create() {
	p.Header = p.components[0]
	curComp := p.Header

	for i := 1; i < len(p.components); i++ {
		nextComp := p.components[i]
		curComp.SetNextComponent(nextComp)
		curComp = nextComp
	}
}

// Start : Start the pipeline by calling 'Start' method of each component in reverse order of chaining
func (p *Pipeline) Start(ctx context.Context) (err error) {
	p.Create()

	var errs []error

	for i := len(p.components) - 1; i >= 0; i-- {
		if err = p.components[i].Start(ctx); err != nil {
			errs = append(errs, err)
			// stop all the upstream components before returning, f.e., this would prevent the upstream components
			// to use the logger after it is destroyed.
			for j := i + 1; j < len(p.components); j++ {
				if err = p.components[j].Stop(); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Stop : Stop the pipeline by calling 'Stop' method of each component
func (p *Pipeline) Stop() (err error) {
	var errs []error
	for i := range p.components {
		if err = p.components[i].Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// AddComponent : Each component calls this method in their init to register the constructor
func AddComponent(name string, init NewComponent) {
	registeredComponents[name] = init
}

func init() {
	registeredComponents = make(map[string]NewComponent)
}
