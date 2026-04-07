// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache

import "errors"

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
