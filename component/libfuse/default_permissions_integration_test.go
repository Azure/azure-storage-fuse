//go:build integration && !fuse2

// Copyright (c) 2026 Microsoft Corporation.
// Licensed under the MIT License.

package libfuse

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/component/loopback"
	"github.com/Azure/azure-storage-fuse/v2/internal"
	"github.com/stretchr/testify/require"
)

type nativeModeBackend struct {
	internal.Component
}

func (b *nativeModeBackend) GetAttr(options internal.GetAttrOptions) (*internal.ObjAttr, error) {
	attr, err := b.Component.GetAttr(options)
	if err != nil {
		return nil, err
	}
	attr.Flags.Clear(internal.PropFlagModeDefault)
	return attr, nil
}

func TestDefaultPermissionsIntegration(t *testing.T) {
	checkFuseAvailable(t)
	if os.Geteuid() == 0 {
		t.Skip("requires an unprivileged reader without CAP_DAC_OVERRIDE")
	}
	require.NoError(t, log.SetDefaultLogger("silent", common.LogConfig{}))

	uid := uint32(os.Geteuid())
	for _, tt := range []struct {
		name       string
		owner      uint32
		enforce    bool
		wantDenied bool
	}{
		{name: "owner", owner: uid, enforce: true},
		{name: "nonowner", owner: uid + 1, enforce: true, wantDenied: true},
		{name: "checks omitted", owner: uid + 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			storage := t.TempDir()
			require.NoError(t, os.Chmod(storage, 0755))
			require.NoError(t, os.WriteFile(filepath.Join(storage, "private"), []byte("private data"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(storage, "public"), []byte("public data"), 0644))
			backend := &nativeModeBackend{Component: loopback.NewLoopbackFSComponent()}
			cfg := fmt.Sprintf("allow-other: true\nloopbackfs:\n  path: %q\nlibfuse:\n  uid: %d\n  default-permissions: %t\n", storage, tt.owner, tt.enforce)
			h := newIntegrationHarness(t, backend, cfg)
			require.NoError(t, backend.Configure(true))
			require.NoError(t, backend.Start(context.Background()))
			t.Cleanup(func() { require.NoError(t, backend.Stop()) })
			h.start(t)
			defer h.stop(t)

			privatePath := filepath.Join(h.MountDir, "private")
			info, err := os.Stat(privatePath)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			stat, ok := info.Sys().(*syscall.Stat_t)
			require.True(t, ok)
			require.Equal(t, tt.owner, stat.Uid)

			data, err := os.ReadFile(privatePath)
			if tt.wantDenied {
				require.ErrorIs(t, err, syscall.EACCES)
			} else {
				require.NoError(t, err)
				require.Equal(t, "private data", string(data))
			}
			data, err = os.ReadFile(filepath.Join(h.MountDir, "public"))
			require.NoError(t, err)
			require.Equal(t, "public data", string(data))
		})
	}
}
