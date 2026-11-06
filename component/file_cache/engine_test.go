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
	"errors"
	"strings"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/component/filecache"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rules that choose the engine: the user's choice wins; without one, the default runs, except that a default of
// v2 falls back to v1 when v2 does not support the config; choosing v2 for such a config, or an unknown engine, fails.
func TestEngineChoice(t *testing.T) {
	unsupported := errors.New("setting x is not supported")
	tests := []struct {
		name       string
		choice     string // engine set by the user, empty if none
		byDefault  string
		v2Support  error // what filecache.CheckSupported returns
		wantEngine string
		wantReason string // part of the reason logged at mount
		wantErr    string // part of the error, empty if there must be none
	}{
		{name: "default v1", byDefault: engineV1, v2Support: unsupported, wantEngine: engineV1, wantReason: "default"},
		{name: "default v2", byDefault: engineV2, wantEngine: engineV2, wantReason: "default"},
		{name: "default v2, unsupported config", byDefault: engineV2, v2Support: unsupported,
			wantEngine: engineV1, wantReason: unsupported.Error()},
		{name: "v1 chosen", choice: engineV1, byDefault: engineV2, wantEngine: engineV1, wantReason: "configured"},
		{name: "v1 chosen, unsupported config", choice: engineV1, byDefault: engineV2, v2Support: unsupported,
			wantEngine: engineV1, wantReason: "configured"},
		{name: "v2 chosen", choice: engineV2, byDefault: engineV1, wantEngine: engineV2, wantReason: "configured"},
		{name: "v2 chosen, unsupported config", choice: engineV2, byDefault: engineV1, v2Support: unsupported,
			wantErr: unsupported.Error()},
		{name: "unknown engine", choice: "v3", byDefault: engineV1, wantErr: `"v3"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, reason, err := chooseEngine(tt.choice, tt.byDefault, func() error { return tt.v2Support })
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEngine, engine)
			assert.Contains(t, reason, tt.wantReason)
		})
	}
}

// buildEngine builds engine v1 of this package, or engine v2 of package filecache.
func TestEngineBuild(t *testing.T) {
	assert.IsType(t, &FileCache{}, buildEngine(engineV1))
	assert.IsType(t, &filecache.FileCache{}, buildEngine(engineV2))
}

// The engine comes from file_cache.engine in the config file, or from --file-cache-engine, which wins.
func TestEngineFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  string // value of file_cache.engine in the config file, empty if not set
		option  string // value of --file-cache-engine, empty if not given
		wantErr string // part of the error of Configure, empty if engine v1 must be built
	}{
		{name: "not set"},
		{name: "v1 in config", config: "v1"},
		{name: "v2 in config", config: "v2", wantErr: "engine v2 cannot be used"},
		{name: "unknown engine in config", config: "v3", wantErr: `"v3"`},
		{name: "not a string in config", config: "{a: b}", wantErr: "invalid file cache engine"},
		{name: "option wins over config", config: "v2", option: "v1"},
		{name: "option v2", config: "v1", option: "v2", wantErr: "engine v2 cannot be used"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := ""
			if tt.config != "" {
				configFile = "file_cache:\n  engine: " + tt.config + "\n"
			}
			require.NoError(t, config.ReadConfigFromReader(strings.NewReader(configFile)))
			if tt.option != "" {
				setEngineOption(t, tt.option)
			}

			component := newComponent()
			if tt.wantErr == "" {
				assert.IsType(t, &FileCache{}, component)
				return
			}
			assert.ErrorContains(t, component.Configure(true), tt.wantErr)
		})
	}
}

// setEngineOption sets --file-cache-engine as if it was given on the command line, until the test ends.
func setEngineOption(t *testing.T, value string) {
	t.Helper()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	config.AttachToFlagSet(flags)
	option := flags.Lookup("file-cache-engine")
	require.NotNil(t, option)
	t.Cleanup(func() {
		require.NoError(t, option.Value.Set(option.DefValue))
		option.Changed = false
	})
	require.NoError(t, flags.Set("file-cache-engine", value))
}
