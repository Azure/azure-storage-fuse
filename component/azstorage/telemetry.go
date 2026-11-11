// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azstorage

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"syscall"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-storage-fuse/v2/internal/telemetry"
)

type telemetryRequestKey struct{}
type telemetryRequest struct{ attempts atomic.Int64 }
type requestObservationPolicy struct{}
type attemptObservationPolicy struct{}

func (requestObservationPolicy) Do(req *policy.Request) (*http.Response, error) {
	if telemetry.Current() == nil {
		return req.Next()
	}
	state := &telemetryRequest{}
	req = req.WithContext(context.WithValue(req.Raw().Context(), telemetryRequestKey{}, state))
	response, err := req.Next()
	telemetry.SDKRequest(int(state.attempts.Load()), err != nil || response != nil && response.StatusCode >= 400)
	observeStartupFailure(err)
	return response, err
}

func (attemptObservationPolicy) Do(req *policy.Request) (*http.Response, error) {
	if state, ok := req.Raw().Context().Value(telemetryRequestKey{}).(*telemetryRequest); ok {
		state.attempts.Add(1)
	}
	return req.Next()
}

type observedTransport struct{ next http.RoundTripper }

func (t observedTransport) CloseIdleConnections() {
	if transport, ok := t.next.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func (t observedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	response, err := t.next.RoundTrip(req)
	if telemetry.Current() == nil {
		return response, err
	}
	telemetry.HTTPAttempt(response != nil)
	if response != nil {
		telemetry.HTTPResponse(response.StatusCode)
		if response.Body != nil {
			response.Body = &observedBody{ReadCloser: response.Body}
		}
	}
	if err != nil {
		telemetry.Network(networkCategory(err), false)
	}
	return response, err
}

type observedBody struct {
	io.ReadCloser
	reported bool
}

func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && err != io.EOF && !b.reported {
		b.reported = true
		telemetry.Network(networkCategory(err), true)
	}
	return n, err
}

func networkCategory(err error) int {
	var dns *net.DNSError
	var cert x509.UnknownAuthorityError
	var host x509.HostnameError
	var invalid x509.CertificateInvalidError
	var tlsErr *tls.CertificateVerificationError
	var record tls.RecordHeaderError
	var netErr net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return 4
	case errors.As(err, &dns):
		return 0
	case errors.As(err, &cert), errors.As(err, &host), errors.As(err, &invalid),
		errors.As(err, &tlsErr), errors.As(err, &record):
		return 1
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return 2
	case errors.Is(err, syscall.ECONNRESET):
		return 3
	default:
		return 5
	}
}

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
	if networkCategory(err) == 0 {
		telemetry.Failure("dns_resolution")
	}
}
