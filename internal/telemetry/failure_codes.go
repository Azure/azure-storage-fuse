// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import "strconv"

type failureCodeSource uint8

const (
	storageServiceCode failureCodeSource = iota
	identityHTTPStatus
)

var startupFailureCodes = map[string]struct {
	category string
	source   failureCodeSource
}{
	"400":                             {"authentication", identityHTTPStatus},
	"401":                             {"authentication", identityHTTPStatus},
	"403":                             {"authentication", identityHTTPStatus},
	"AuthenticationFailed":            {"authentication", storageServiceCode},
	"InvalidAuthenticationInfo":       {"authentication", storageServiceCode},
	"NoAuthenticationInformation":     {"authentication", storageServiceCode},
	"AuthorizationFailure":            {"authorization", storageServiceCode},
	"AuthorizationPermissionMismatch": {"authorization", storageServiceCode},
	"InsufficientAccountPermissions":  {"authorization", storageServiceCode},
	"ContainerNotFound":               {"container_not_found", storageServiceCode},
	"FilesystemNotFound":              {"container_not_found", storageServiceCode},
}

func StorageFailureCode(code string) {
	recordFailureCode(code, storageServiceCode)
}

func IdentityFailureStatus(status int) {
	recordFailureCode(strconv.Itoa(status), identityHTTPStatus)
}

func recordFailureCode(code string, source failureCodeSource) {
	if entry, ok := startupFailureCodes[code]; ok && entry.source == source {
		recordFailure(entry.category, code)
	}
}
