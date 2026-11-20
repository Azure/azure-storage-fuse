// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package azstorage

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

// Preserve the existing user-facing validation message and its typed cause.
type storageValidationError struct {
	message string
	cause   error
}

func (e *storageValidationError) Error() string { return e.message }
func (e *storageValidationError) Unwrap() error { return e.cause }

var publicStorageHost = regexp.MustCompile(`^([a-z0-9]{3,24})\.(?:(privatelink)\.)?(?:blob|dfs)\.core\.windows\.net$|^([a-z0-9]{3,24})\.z[0-9]+\.(?:blob|dfs)\.storage\.azure\.net$`)

func storageTelemetryIdentity(endpoint, account string) (string, string) {
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "other", ""
	}
	m := publicStorageHost.FindStringSubmatch(strings.TrimSuffix(strings.ToLower(u.Hostname()), "."))
	if m == nil {
		return "other", ""
	}
	name, kind := m[1]+m[3], "public"
	if m[2] != "" {
		kind = "private_link"
	}
	if name != strings.ToLower(account) {
		return kind, ""
	}
	return kind, name
}

func observeStorageConfig(auth azAuthConfig) {
	kind, name := storageTelemetryIdentity(auth.Endpoint, auth.AccountName)
	telemetry.Current().Configure(map[string]string{
		"auth_mode": auth.AuthMode.String(), "azstorage.account_type": auth.AccountType.String(),
		"azstorage.endpoint_type": kind, "azstorage.account_name": name,
	})
}

func observeStartupFailure(err error) {
	if err == nil {
		return
	}
	var response *azcore.ResponseError
	var auth *azidentity.AuthenticationFailedError
	if errors.As(err, &auth) && auth.RawResponse != nil {
		telemetry.IdentityFailureStatus(auth.RawResponse.StatusCode)
		return
	}
	if errors.As(err, &response) {
		telemetry.StorageFailureCode(response.ErrorCode)
		return
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		telemetry.Failure("dns_resolution")
	}
}
