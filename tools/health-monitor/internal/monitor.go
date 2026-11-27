// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package internal

type Monitor interface {
	GetName() string
	SetName(string)
	Monitor() error
	Validate() error
	ExportStats(string, any)
}
