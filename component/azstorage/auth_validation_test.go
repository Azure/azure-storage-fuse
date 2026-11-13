// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azstorage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/config"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureValidationTest(t *testing.T, endpoint, accountType string, skip *bool) (*AzStorage, error) {
	t.Helper()
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)
	require.NoError(t, log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()}))

	var skipConfig string
	if skip != nil {
		skipConfig = fmt.Sprintf("\n  skip-mount-validation: %t", *skip)
	}
	cfg := fmt.Sprintf(`
azstorage:
  type: %s
  account-name: account
  container: container
  endpoint: %s
  mode: sas
  sas: "?sv=2020-08-04&sig=invalid"
  max-retries: 1%s
`, accountType, endpoint, skipConfig)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(cfg)))

	az := &AzStorage{}
	az.SetName(compName)
	return az, az.Configure(true)
}

func writeAuthenticationFailure(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-ms-error-code", "AuthenticationFailed")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("<Error><Code>AuthenticationFailed</Code><Message>invalid credentials</Message></Error>"))
}

func TestAuthValidationDefaultRejectsInvalidCredentials(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeAuthenticationFailure(w)
	}))
	defer server.Close()

	_, err := configureValidationTest(t, server.URL, "block", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to authenticate")
	assert.Positive(t, requests.Load())
}

func TestSkipMountValidationDefersInvalidCredentialsToStorageOperation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeAuthenticationFailure(w)
	}))
	defer server.Close()

	skip := true
	az, err := configureValidationTest(t, server.URL, "block", &skip)
	require.NoError(t, err)
	assert.Zero(t, requests.Load(), "mount initialization must not make a validation request")

	_, err = az.storage.GetAttr("file")
	assert.Error(t, err)
	assert.Positive(t, requests.Load(), "later Storage access must still use the configured authentication")
}

func TestSkipMountValidationRequiresExplicitAccountType(t *testing.T) {
	config.ResetConfig()
	t.Cleanup(config.ResetConfig)
	require.NoError(t, config.ReadConfigFromReader(strings.NewReader(`
azstorage:
  account-name: account
  container: container
  mode: sas
  sas: "?sv=2020-08-04&sig=invalid"
  skip-mount-validation: true
`)))

	az := &AzStorage{}
	az.SetName(compName)
	err := az.Configure(true)
	assert.EqualError(t, err, "config error in azstorage [account type must be explicitly provided when skip-mount-validation is enabled]")
}

func TestADLSAuthValidationRemainsParallel(t *testing.T) {
	var requests atomic.Int32
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	bothStarted := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			maximum := maxInFlight.Load()
			if current <= maximum || maxInFlight.CompareAndSwap(maximum, current) {
				break
			}
		}
		if requests.Add(1) == 2 {
			close(bothStarted)
		}
		select {
		case <-bothStarted:
		case <-time.After(2 * time.Second):
		}
		writeAuthenticationFailure(w)
	}))
	defer server.Close()

	skip := false
	_, err := configureValidationTest(t, server.URL, "adls", &skip)
	assert.Error(t, err)
	assert.GreaterOrEqual(t, requests.Load(), int32(2))
	assert.Equal(t, int32(2), maxInFlight.Load(), "Blob and DFS validation requests must overlap")
}
