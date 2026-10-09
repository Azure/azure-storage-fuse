/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
*/

package azstorage

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-storage-fuse/v2/common/log"
)

type reloadableTrustStore struct {
	path     string
	mutex    sync.Mutex
	fileInfo os.FileInfo
}

func newReloadableTrustStore(path string) (*reloadableTrustStore, error) {
	store := &reloadableTrustStore{path: path}
	if _, err := store.load(); err != nil {
		return nil, err
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect TLS trust store %q: %w", path, err)
	}
	store.fileInfo = fileInfo
	return store, nil
}

func (store *reloadableTrustStore) load() (*x509.CertPool, error) {
	// Build a fresh pool so each TLS handshake observes the current bundle.
	bundle, err := os.ReadFile(store.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS trust store %q: %w", store.path, err)
	}

	pool := x509.NewCertPool()
	certificateCount := 0
	for len(bundle) > 0 {
		var block *pem.Block
		block, bundle = pem.Decode(bundle)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificates, parseErr := x509.ParseCertificates(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse TLS trust store %q: %w", store.path, parseErr)
		}
		for _, certificate := range certificates {
			pool.AddCert(certificate)
			certificateCount++
		}
	}

	if certificateCount == 0 {
		return nil, fmt.Errorf("TLS trust store %q contains no certificates", store.path)
	}
	return pool, nil
}

func (store *reloadableTrustStore) changed() (bool, error) {
	fileInfo, err := os.Stat(store.path)
	if err != nil {
		return true, fmt.Errorf("failed to inspect TLS trust store %q: %w", store.path, err)
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	// SameFile detects atomic replacement; mtime and size detect in-place updates.
	changed := store.fileInfo == nil ||
		!os.SameFile(store.fileInfo, fileInfo) ||
		!store.fileInfo.ModTime().Equal(fileInfo.ModTime()) ||
		store.fileInfo.Size() != fileInfo.Size()
	store.fileInfo = fileInfo
	return changed, nil
}

func (store *reloadableTrustStore) verifyConnection(state tls.ConnectionState) error {
	if len(state.PeerCertificates) == 0 {
		return errors.New("TLS peer provided no certificates")
	}

	roots, err := store.load()
	if err != nil {
		return err
	}

	intermediates := x509.NewCertPool()
	for _, certificate := range state.PeerCertificates[1:] {
		intermediates.AddCert(certificate)
	}

	_, err = state.PeerCertificates[0].Verify(x509.VerifyOptions{
		DNSName:       state.ServerName,
		Intermediates: intermediates,
		Roots:         roots,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	return err
}

type closeIdleRoundTripper interface {
	http.RoundTripper
	CloseIdleConnections()
}

type tlsHotReloadTransport struct {
	transport  closeIdleRoundTripper
	trustStore *reloadableTrustStore
	backoff    time.Duration
}

func newTLSHotReloadTransport(transport *http.Transport, trustStorePath string, backoff time.Duration) (http.RoundTripper, error) {
	store, err := newReloadableTrustStore(trustStorePath)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{ // #nosec G402 -- VerifyConnection performs full chain and hostname verification.
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // Verification is performed below with the current trust bundle.
		VerifyConnection:   store.verifyConnection,
	}
	transport.TLSClientConfig = tlsConfig

	return &tlsHotReloadTransport{
		transport:  transport,
		trustStore: store,
		backoff:    backoff,
	}, nil
}

func (transport *tlsHotReloadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	// Retire pooled connections when the configured bundle changes.
	if changed, changeErr := transport.trustStore.changed(); changed {
		transport.transport.CloseIdleConnections()
		if changeErr != nil {
			log.Warn("tlsHotReloadTransport::RoundTrip : TLS trust store change check failed: %s", changeErr)
		} else {
			log.Info("tlsHotReloadTransport::RoundTrip : TLS trust store changed; idle connections closed")
		}
	}

	response, err := transport.transport.RoundTrip(request)
	if err == nil || !isTLSCertificateError(err) {
		return response, err
	}

	retryRequest, retryErr := cloneRequestForRetry(request)
	if retryErr != nil {
		log.Warn("tlsHotReloadTransport::RoundTrip : TLS authentication failed, but request cannot be replayed: %s", retryErr)
		return response, err
	}

	// Retry once outside the SDK retry policy so a rotated bundle can recover
	// the connection that first observed the new certificate chain.
	log.Warn("tlsHotReloadTransport::RoundTrip : TLS authentication failed; reloading trust store and reconnecting after %s", transport.backoff)
	transport.transport.CloseIdleConnections()
	if waitErr := waitForTLSRetry(request.Context(), transport.backoff); waitErr != nil {
		return nil, waitErr
	}

	return transport.transport.RoundTrip(retryRequest)
}

func (transport *tlsHotReloadTransport) CloseIdleConnections() {
	transport.transport.CloseIdleConnections()
}

func cloneRequestForRetry(request *http.Request) (*http.Request, error) {
	retryRequest := request.Clone(request.Context())
	if request.Body == nil || request.Body == http.NoBody {
		return retryRequest, nil
	}
	if request.GetBody == nil {
		return nil, errors.New("request body does not provide GetBody")
	}

	body, err := request.GetBody()
	if err != nil {
		return nil, fmt.Errorf("failed to recreate request body: %w", err)
	}
	retryRequest.Body = body
	return retryRequest, nil
}

func waitForTLSRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isTLSCertificateError(err error) bool {
	var unknownAuthorityError x509.UnknownAuthorityError
	var hostnameError x509.HostnameError
	var certificateInvalidError x509.CertificateInvalidError
	var verificationError *tls.CertificateVerificationError

	return errors.As(err, &unknownAuthorityError) ||
		errors.As(err, &hostnameError) ||
		errors.As(err, &certificateInvalidError) ||
		errors.As(err, &verificationError)
}

var _ http.RoundTripper = (*tlsHotReloadTransport)(nil)
