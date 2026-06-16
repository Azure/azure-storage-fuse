// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import (
	"errors"
	"io"
	"net"
	"os"
)

// Sentinel errors returned by the distributed cache client.
var (
	ErrNotFound              = errors.New("dcache: not found")
	ErrNotFoundGotLock       = errors.New("dcache: not found, lock acquired")
	ErrNotFoundAlreadyLocked = errors.New("dcache: not found, locked by another client")
	ErrAuthFailed            = errors.New("dcache: authentication failed")
	ErrServerError           = errors.New("dcache: server internal error")
	ErrBadRequest            = errors.New("dcache: bad request")
	ErrFileExists            = errors.New("dcache: file already exists")
	ErrInvalidOffset         = errors.New("dcache: invalid offset or length")
	ErrFilenameTooLong       = errors.New("dcache: filename limit exceeded")
	ErrNoServers             = errors.New("dcache: no servers available")
	ErrConnectionFailed      = errors.New("dcache: connection failed")
	ErrClosed                = errors.New("dcache: client closed")
)

// IsRecoverableNetErr returns true if the error is a network timeout or
// transient connection error that should be treated as a per-chunk recoverable
// failure rather than a fatal download error.
func IsRecoverableNetErr(err error) bool {
	if err == nil {
		return false
	}

	// Check for connection-related sentinel errors
	if errors.Is(err, ErrConnectionFailed) {
		return true
	}

	// Check for EOF errors (server closed connection mid-stream)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// Check for timeout errors (net.Error with Timeout() or os.ErrDeadlineExceeded)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	// Check for connection reset/refused/aborted
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	return false
}
