/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
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

package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

func pipelineTracef(format string, args ...any) {
	if os.Getenv("BLOBFUSE2_MOUNT_TRACE") == "" {
		return
	}

	fmt.Fprintf(
		os.Stderr,
		"BLOBFUSE2_MOUNT_TRACE time=%s pid=%d %s\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		os.Getpid(),
		fmt.Sprintf(format, args...),
	)
}

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
	pipelineTracef("Pipeline.NewPipeline: begin components=%v is-parent=%t", components, isParent)
	comps := make([]Component, 0)
	lastPriority := EComponentPriority.Producer()
	for _, name := range components {
		pipelineTracef("Pipeline.NewPipeline: component=%q lookup begin", name)
		if name == "stream" {
			common.IsStream = true
			name = "block_cache"
			pipelineTracef("Pipeline.NewPipeline: stream mapped to block_cache")
		}
		//  Search component exists in our registered map or not
		compInit, ok := registeredComponents[name]
		if ok {
			// Call the constructor method registered by the component
			pipelineTracef("Pipeline.NewPipeline: component=%q constructor begin", name)
			comp := compInit()
			pipelineTracef("Pipeline.NewPipeline: component=%q constructor complete name=%q", name, comp.Name())

			// request component to parse and validate config of its interest
			pipelineTracef("Pipeline.NewPipeline: component=%q Configure begin", comp.Name())
			err := comp.Configure(isParent)
			if err != nil {
				pipelineTracef("Pipeline.NewPipeline: component=%q Configure error=%q", comp.Name(), err.Error())
				log.Err("Pipeline: error creating pipeline component %s [%s]", comp.Name(), err)
				return nil, err
			}
			pipelineTracef("Pipeline.NewPipeline: component=%q Configure complete priority=%d", comp.Name(), comp.Priority())

			if comp.Priority() > lastPriority {
				pipelineTracef("Pipeline.NewPipeline: component=%q invalid priority=%d previous=%d", comp.Name(), comp.Priority(), lastPriority)
				log.Err("Pipeline::NewPipeline : Invalid Component order [priority of %s higher than above components]", comp.Name())
				return nil, fmt.Errorf("config error in Pipeline [component %s is out of order]", name)
			} else {
				lastPriority = comp.Priority()
			}

			// store the configured object in list of components
			comps = append(comps, comp)
		} else {
			pipelineTracef("Pipeline.NewPipeline: component=%q is not registered", name)
			log.Err("Pipeline: error [component %s not registered]", name)
			return nil, fmt.Errorf("config error in Pipeline [component %s not registered]", name)
		}

	}

	// Create pipeline structure holding list of all component objects requested by config file
	pipelineTracef("Pipeline.NewPipeline: complete component-count=%d", len(comps))
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
	pipelineTracef("Pipeline.Start: begin component-count=%d", len(p.components))
	p.Create()
	pipelineTracef("Pipeline.Start: component chain created")

	var errs []error

	for i := len(p.components) - 1; i >= 0; i-- {
		pipelineTracef("Pipeline.Start: component=%q Start begin", p.components[i].Name())
		if err = p.components[i].Start(ctx); err != nil {
			pipelineTracef("Pipeline.Start: component=%q Start error=%q", p.components[i].Name(), err.Error())
			errs = append(errs, err)
			// stop all the upstream components before returning, f.e., this would prevent the upstream components
			// to use the logger after it is destroyed.
			for j := i + 1; j < len(p.components); j++ {
				pipelineTracef("Pipeline.Start: cleanup component=%q Stop begin", p.components[j].Name())
				if err = p.components[j].Stop(); err != nil {
					pipelineTracef("Pipeline.Start: cleanup component=%q Stop error=%q", p.components[j].Name(), err.Error())
					errs = append(errs, err)
				} else {
					pipelineTracef("Pipeline.Start: cleanup component=%q Stop complete", p.components[j].Name())
				}
			}
		} else {
			pipelineTracef("Pipeline.Start: component=%q Start complete", p.components[i].Name())
		}
	}

	if len(errs) > 0 {
		pipelineTracef("Pipeline.Start: returning errors count=%d", len(errs))
		return errors.Join(errs...)
	}

	pipelineTracef("Pipeline.Start: complete")
	return nil
}

// Stop : Stop the pipeline by calling 'Stop' method of each component
func (p *Pipeline) Stop() (err error) {
	pipelineTracef("Pipeline.Stop: begin component-count=%d", len(p.components))
	var errs []error
	for i := range p.components {
		pipelineTracef("Pipeline.Stop: component=%q Stop begin", p.components[i].Name())
		if err = p.components[i].Stop(); err != nil {
			pipelineTracef("Pipeline.Stop: component=%q Stop error=%q", p.components[i].Name(), err.Error())
			errs = append(errs, err)
		} else {
			pipelineTracef("Pipeline.Stop: component=%q Stop complete", p.components[i].Name())
		}
	}

	if len(errs) > 0 {
		pipelineTracef("Pipeline.Stop: returning errors count=%d", len(errs))
		return errors.Join(errs...)
	}

	pipelineTracef("Pipeline.Stop: complete")
	return nil
}

// AddComponent : Each component calls this method in their init to register the constructor
func AddComponent(name string, init NewComponent) {
	registeredComponents[name] = init
}

func init() {
	registeredComponents = make(map[string]NewComponent)
}
