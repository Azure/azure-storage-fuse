/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
*/

package azstorage

import (
	"context"
	"crypto/sha256"
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
	path        string
	mutex       sync.Mutex
	fingerprint [sha256.Size]byte
	initialized bool
}

func newReloadableTrustStore(path string) (*reloadableTrustStore, error) {
	store := &reloadableTrustStore{path: path}
	if _, _, err := store.reload(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *reloadableTrustStore) load() (*x509.CertPool, [sha256.Size]byte, error) {
	bundle, err := os.ReadFile(store.path)
	if err != nil {
		return nil, [sha256.Size]byte{}, fmt.Errorf("failed to read TLS trust store %q: %w", store.path, err)
	}
	fingerprint := sha256.Sum256(bundle)

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
			return nil, [sha256.Size]byte{}, fmt.Errorf("failed to parse TLS trust store %q: %w", store.path, parseErr)
		}
		for _, certificate := range certificates {
			pool.AddCert(certificate)
			certificateCount++
		}
	}

	if certificateCount == 0 {
		return nil, [sha256.Size]byte{}, fmt.Errorf("TLS trust store %q contains no certificates", store.path)
	}
	return pool, fingerprint, nil
}

func (store *reloadableTrustStore) reload() (*x509.CertPool, bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	roots, fingerprint, err := store.load()
	if err != nil {
		return nil, false, err
	}

	changed := !store.initialized || store.fingerprint != fingerprint
	if changed {
		store.fingerprint = fingerprint
		store.initialized = true
	}
	return roots, changed, nil
}

type closeIdleRoundTripper interface {
	http.RoundTripper
	CloseIdleConnections()
}

type tlsHotReloadTransport struct {
	mutex       sync.RWMutex
	reloadMutex sync.Mutex
	template    *http.Transport
	transport   closeIdleRoundTripper
	trustStore  *reloadableTrustStore
	backoff     time.Duration
}

func newTLSHotReloadTransport(transport *http.Transport, trustStorePath string, backoff time.Duration) (http.RoundTripper, error) {
	store, err := newReloadableTrustStore(trustStorePath)
	if err != nil {
		return nil, err
	}

	roots, _, err := store.reload()
	if err != nil {
		return nil, err
	}

	template := transport.Clone()
	activeTransport := newTransportGeneration(template, roots)

	return &tlsHotReloadTransport{
		template:   template,
		transport:  activeTransport,
		trustStore: store,
		backoff:    backoff,
	}, nil
}

func newTransportGeneration(template *http.Transport, roots *x509.CertPool) *http.Transport {
	transport := template.Clone()
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	if transport.TLSClientConfig != nil {
		tlsConfig = transport.TLSClientConfig.Clone()
		tlsConfig.MinVersion = max(tlsConfig.MinVersion, tls.VersionTLS12)
		tlsConfig.RootCAs = roots
		tlsConfig.InsecureSkipVerify = false
	}
	// Session resumption can bypass certificate verification, so each trust-store
	// generation starts without a cache inherited from the previous generation.
	tlsConfig.ClientSessionCache = nil
	transport.TLSClientConfig = tlsConfig
	return transport
}

func (transport *tlsHotReloadTransport) activeTransport() closeIdleRoundTripper {
	transport.mutex.RLock()
	defer transport.mutex.RUnlock()
	return transport.transport
}

func (transport *tlsHotReloadTransport) reloadTransport() (bool, error) {
	if transport.trustStore == nil {
		return false, nil
	}

	transport.reloadMutex.Lock()
	defer transport.reloadMutex.Unlock()

	roots, changed, err := transport.trustStore.reload()
	if err != nil || !changed {
		return changed, err
	}

	nextTransport := newTransportGeneration(transport.template, roots)
	transport.mutex.Lock()
	retiredTransport := transport.transport
	transport.transport = nextTransport
	transport.mutex.Unlock()
	retiredTransport.CloseIdleConnections()
	return true, nil
}

func (transport *tlsHotReloadTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	changed, reloadErr := transport.reloadTransport()
	if reloadErr != nil {
		return nil, reloadErr
	}
	if changed {
		log.Info("tlsHotReloadTransport::RoundTrip : TLS trust store changed; transport rotated")
	}

	activeTransport := transport.activeTransport()
	response, err := activeTransport.RoundTrip(request)
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
	activeTransport.CloseIdleConnections()
	if waitErr := waitForTLSRetry(request.Context(), transport.backoff); waitErr != nil {
		return nil, waitErr
	}
	if _, reloadErr = transport.reloadTransport(); reloadErr != nil {
		return nil, reloadErr
	}

	return transport.activeTransport().RoundTrip(retryRequest)
}

func (transport *tlsHotReloadTransport) CloseIdleConnections() {
	transport.activeTransport().CloseIdleConnections()
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
