//go:build !unittest
// +build !unittest

// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package dcache_e2e

import (
	"slices"
	"testing"
)

func TestParsePodState(t *testing.T) {
	data := []byte(`{
		"items": [
			{
				"metadata": {"name": "ready"},
				"status": {"phase": "Running"}
			},
			{
				"metadata": {
					"name": "terminating",
					"deletionTimestamp": "2026-10-04T15:48:30Z"
				},
				"status": {"phase": "Running"}
			},
			{
				"metadata": {"name": "pending"},
				"status": {"phase": "Pending"}
			},
			{
				"metadata": {"name": "historical-failure"},
				"status": {"phase": "Failed"}
			}
		]
	}`)

	live, total, err := parsePodState(data)
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("total pods = %d, want 3", total)
	}
	if !slices.Equal(live, []string{"ready"}) {
		t.Fatalf("live pods = %v, want [ready]", live)
	}
}

func TestParsePodStateInvalidJSON(t *testing.T) {
	if _, _, err := parsePodState([]byte(`{"items":`)); err == nil {
		t.Fatal("parsePodState accepted invalid JSON")
	}
}
