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

package file_cache

import (
	"fmt"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/filecache"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

// The engines of the file_cache component: v1 is the original one, in this package, and v2 is the rewrite, in package
// filecache.
const (
	engineV1 = "v1"
	engineV2 = "v2"
)

// defaultEngine runs when the user does not choose an engine.
const defaultEngine = engineV1

// registerComponent registers the file_cache component, and the option that chooses its engine.
func registerComponent() {
	internal.AddComponent(compName, newComponent)

	engine := config.AddStringFlag("file-cache-engine", defaultEngine, "Engine of the file cache: v1 or v2.")
	config.BindPFlag(compName+".engine", engine)
}

// newComponent builds the engine chosen by the setting file_cache.engine. If no engine can be chosen, it returns a
// component whose Configure fails with the reason, which stops the mount.
func newComponent() internal.Component {
	var choice string
	if err := config.UnmarshalKey(compName+".engine", &choice); err != nil {
		return newFailedComponent(fmt.Errorf("invalid file cache engine: %w", err))
	}

	engine, reason, err := chooseEngine(choice, defaultEngine, filecache.CheckSupported)
	if err != nil {
		return newFailedComponent(err)
	}

	log.Crit("FileCache::newComponent : using engine %s, %s", engine, reason)
	return buildEngine(engine)
}

// chooseEngine returns the engine to run, and why. choice is the engine set by the user, empty if none. byDefault runs
// when choice is empty, unless it is v2 and v2 does not support the config. checkV2 is filecache.CheckSupported; it is
// called only when the result depends on it.
func chooseEngine(choice, byDefault string, checkV2 func() error) (engine, reason string, err error) {
	switch choice {
	case "":
		if byDefault == engineV2 {
			if unsupported := checkV2(); unsupported != nil {
				return engineV1, fmt.Sprintf("because engine v2 does not support the config: %v", unsupported), nil
			}
		}
		return byDefault, "the default", nil
	case engineV1:
		return engineV1, "as configured", nil
	case engineV2:
		if unsupported := checkV2(); unsupported != nil {
			return "", "", fmt.Errorf("file cache engine v2 cannot be used: %w", unsupported)
		}
		return engineV2, "as configured", nil
	default:
		return "", "", fmt.Errorf("invalid file cache engine %q, it must be %s or %s", choice, engineV1, engineV2)
	}
}

// buildEngine returns a new instance of the engine, which still needs to be configured.
func buildEngine(engine string) internal.Component {
	if engine == engineV2 {
		return filecache.New()
	}
	return NewFileCacheComponent()
}

// failedComponent stands in for the file cache when no engine can be chosen. Its Configure returns the reason.
type failedComponent struct {
	internal.BaseComponent
	err error
}

// newFailedComponent returns a component whose Configure fails with err.
func newFailedComponent(err error) *failedComponent {
	c := &failedComponent{err: err}
	c.SetName(compName)
	return c
}

// Configure returns the reason no engine could be chosen.
func (c *failedComponent) Configure(_ bool) error {
	return c.err
}
