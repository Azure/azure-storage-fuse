// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package telemetrytest provides local-only receivers for telemetry tests.
package telemetrytest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const hostname = "local-test.in.applicationinsights.azure.com"

type Receiver struct {
	ConnectionString string
	Client           *http.Client
	Env              []string
}

func NewReceiver(t *testing.T, handler http.Handler) *Receiver {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), DNSNames: []string{hostname},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:        true, BasicConstraintsValid: true,
	}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{cert}, PrivateKey: key}}}
	server.StartTLS()
	t.Cleanup(server.Close)

	// Tunnel only the fixture hostname to the local receiver, never to the network.
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != hostname+":443" {
			t.Errorf("unexpected telemetry proxy target: %s %s", r.Method, r.Host)
			http.Error(w, "unexpected proxy target", http.StatusForbidden)
			return
		}
		upstream, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
		if err != nil {
			t.Error(err)
			http.Error(w, "local receiver unavailable", http.StatusBadGateway)
			return
		}
		defer upstream.Close()
		client, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer client.Close()
		if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			t.Error(err)
			return
		}
		if err := buffered.Flush(); err != nil {
			t.Error(err)
			return
		}
		go func() {
			_, _ = io.Copy(upstream, buffered)
			_ = upstream.Close()
		}()
		_, _ = io.Copy(client, upstream)
	}))
	t.Cleanup(proxy.Close)
	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	client := server.Client()
	client.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)
	t.Cleanup(client.CloseIdleConnections)
	certPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: cert,
	}), 0600))
	connection := "InstrumentationKey=test;IngestionEndpoint=https://" + hostname
	return &Receiver{
		ConnectionString: connection, Client: client,
		Env: []string{"BLOBFUSE_TELEMETRY_CONNECTION_STRING=" + connection,
			"SSL_CERT_FILE=" + certPath, "HTTPS_PROXY=" + proxy.URL, "NO_PROXY=", "no_proxy="},
	}
}
