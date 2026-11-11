// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azstorage

import (
	"errors"
	"net"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

func observeStartupFailure(err error) {
	if err == nil {
		return
	}
	var response *azcore.ResponseError
	var auth *azidentity.AuthenticationFailedError
	if errors.As(err, &auth) && auth.RawResponse != nil {
		switch auth.RawResponse.StatusCode {
		case 400, 401, 403:
			telemetry.Failure("authentication")
		}
		return
	}
	if errors.As(err, &response) {
		switch response.ErrorCode {
		case "ContainerNotFound", "FilesystemNotFound":
			telemetry.Failure("container_not_found")
		case "AuthenticationFailed", "InvalidAuthenticationInfo", "NoAuthenticationInformation":
			telemetry.Failure("authentication")
		case "AuthorizationFailure", "AuthorizationPermissionMismatch", "InsufficientAccountPermissions":
			telemetry.Failure("authorization")
		}
		return
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		telemetry.Failure("dns_resolution")
	}
}
