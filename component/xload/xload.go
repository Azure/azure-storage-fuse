/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2023 Microsoft Corporation. All rights reserved.
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

package xload

import (
	"context"
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

/* NOTES:
   - Component shall have a structure which inherits "internal.BaseComponent" to participate in pipeline
   - Component shall register a name and its constructor to participate in pipeline  (add by default by generator)
   - Order of calls : Constructor -> Configure -> Start ..... -> Stop
   - To read any new setting from config file follow the Configure method default comments
*/

// Common structure for Component
type Xload struct {
	internal.BaseComponent
}

// Structure defining your config parameters
type XloadOptions struct {
	// e.g. var1 uint32 `config:"var1"`
}

const compName = "xload"

// Verification to check satisfaction criteria with Component Interface
var _ internal.Component = &Xload{}

func (c *Xload) Name() string {
	return compName
}

func (c *Xload) SetName(name string) {
	c.BaseComponent.SetName(name)
}

func (c *Xload) SetNextComponent(nc internal.Component) {
	c.BaseComponent.SetNextComponent(nc)
}

// Start : Pipeline calls this method to start the component functionality
func (c *Xload) Start(ctx context.Context) error {
	log.Trace("Xload::Start : Starting component %s", c.Name())

	// Xload : start code goes here

	return nil
}

// Stop : Stop the component functionality and kill all threads started
func (c *Xload) Stop() error {
	log.Trace("Xload::Stop : Stopping component %s", c.Name())

	return nil
}

// Configure : Pipeline will call this method after constructor so that you can read config and initialize yourself
func (c *Xload) Configure(_ bool) error {
	log.Trace("Xload::Configure : %s", c.Name())

	// >> If you do not need any config parameters remove below code and return nil
	conf := XloadOptions{}
	err := config.UnmarshalKey(c.Name(), &conf)
	if err != nil {
		log.Err("Xload::Configure : config error [invalid config attributes]")
		return fmt.Errorf("Xload: config error [invalid config attributes]")
	}
	// Extract values from 'conf' and store them as you wish here

	return nil
}

// ------------------------- Factory -------------------------------------------

// Pipeline will call this method to create your object, initialize your variables here
func NewXloadComponent() internal.Component {
	comp := &Xload{}
	comp.SetName(compName)
	return comp
}

// On init register this component to pipeline and supply your constructor
func init() {
	internal.AddComponent(compName, NewXloadComponent)
}
