/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.
   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
*/

package azstorage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestCertificate(t *testing.T, serial int64) (tls.Certificate, []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyDER}),
	)
	require.NoError(t, err)

	return certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
}

func writeTrustBundle(t *testing.T, path string, bundle []byte, modTime time.Time) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, bundle, 0600))
	require.NoError(t, os.Chtimes(path, modTime, modTime))
}

func TestTLSHotReloadUsesUpdatedTrustBundle(t *testing.T) {
	serverCertificate, trustedBundle := createTestCertificate(t, 1)
	_, untrustedBundle := createTestCertificate(t, 2)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("ok"))
	}))
	server.TLS = &tls.Config{ // #nosec G402 -- test server configuration.
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCertificate},
	}
	server.StartTLS()
	defer server.Close()

	trustStorePath := filepath.Join(t.TempDir(), "ca-bundle.pem")
	modTime := time.Now().Add(-time.Minute)
	writeTrustBundle(t, trustStorePath, trustedBundle, modTime)

	roundTripper, err := newTLSHotReloadTransport(&http.Transport{}, trustStorePath, 0)
	require.NoError(t, err)
	client := &http.Client{Transport: roundTripper}

	response, err := client.Get(server.URL)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, response.Body)
	require.NoError(t, response.Body.Close())

	writeTrustBundle(t, trustStorePath, untrustedBundle, modTime.Add(time.Second))
	_, err = client.Get(server.URL)
	require.Error(t, err)
	assert.True(t, isTLSCertificateError(err))

	writeTrustBundle(t, trustStorePath, trustedBundle, modTime.Add(2*time.Second))
	response, err = client.Get(server.URL)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
}

type tlsRetryRoundTripper struct {
	attempts       int
	idleCloseCalls int
}

func (transport *tlsRetryRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	transport.attempts++
	if transport.attempts == 1 {
		return nil, x509.UnknownAuthorityError{}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil
}

func (transport *tlsRetryRoundTripper) CloseIdleConnections() {
	transport.idleCloseCalls++
}

func TestTLSCertificateErrorRetriesAfterBackoff(t *testing.T) {
	_, bundle := createTestCertificate(t, 3)
	trustStorePath := filepath.Join(t.TempDir(), "ca-bundle.pem")
	writeTrustBundle(t, trustStorePath, bundle, time.Now())

	baseTransport := &tlsRetryRoundTripper{}
	store, err := newReloadableTrustStore(trustStorePath)
	require.NoError(t, err)
	transport := &tlsHotReloadTransport{
		transport:  baseTransport,
		trustStore: store,
		backoff:    time.Millisecond,
	}

	request := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	response, err := transport.RoundTrip(request)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, 2, baseTransport.attempts)
	assert.Equal(t, 1, baseTransport.idleCloseCalls)
}

func TestTLSHotReloadTransportClosesIdleConnections(t *testing.T) {
	baseTransport := &tlsRetryRoundTripper{}
	transport := &tlsHotReloadTransport{transport: baseTransport}

	transport.CloseIdleConnections()

	assert.Equal(t, 1, baseTransport.idleCloseCalls)
}

func TestTLSRetryWaitHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForTLSRetry(ctx, time.Hour)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestInvalidTLSBundleFailsTransportCreation(t *testing.T) {
	trustStorePath := filepath.Join(t.TempDir(), "ca-bundle.pem")
	require.NoError(t, os.WriteFile(trustStorePath, []byte("not a certificate"), 0600))

	_, err := newTLSHotReloadTransport(&http.Transport{}, trustStorePath, 0)
	assert.ErrorContains(t, err, "contains no certificates")
}

func TestTLSCertificateErrorDetection(t *testing.T) {
	assert.True(t, isTLSCertificateError(x509.UnknownAuthorityError{}))
	assert.False(t, isTLSCertificateError(errors.New("connection reset")))
}
