// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Package telemetry implements optional Microsoft-facing mount telemetry.
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

const (
	schemaVersion         = "1"
	maxPropertyValueBytes = 1024
	MaxEventBytes         = 32 * 1024
	SendTimeout           = 5 * time.Second
	FlushTimeout          = 5 * time.Second
)

// Event is the internal, destination-independent contract for one-shot delivery.
type Event struct {
	Name         string             `json:"name"`
	Time         time.Time          `json:"time"`
	Properties   map[string]string  `json:"properties"`
	Measurements map[string]float64 `json:"measurements,omitempty"`
}

type envelope struct {
	Name string    `json:"name"`
	Time time.Time `json:"time"`
	Key  string    `json:"iKey"`
	Data struct {
		Type string `json:"baseType"`
		Data struct {
			Version      int                `json:"ver"`
			Name         string             `json:"name"`
			Properties   map[string]string  `json:"properties"`
			Measurements map[string]float64 `json:"measurements,omitempty"`
		} `json:"baseData"`
	} `json:"data"`
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // Go's crypto/rand.Read terminates on entropy failure.
	return hex.EncodeToString(b[:])
}

func validID(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 16 && s == strings.ToLower(s)
}

func encode(e Event, key string) ([]byte, error) {
	switch e.Name {
	case "blobfuse.mount.result", "blobfuse.mount.stopped":
	default:
		return nil, errors.New("invalid telemetry event")
	}
	if e.Properties["schema_version"] != schemaVersion || e.Time.IsZero() ||
		!validID(e.Properties["event_id"]) || !validID(e.Properties["mount_session_id"]) {
		return nil, errors.New("invalid telemetry identity")
	}
	for k, v := range e.Properties {
		if len(v) > maxPropertyValueBytes {
			return nil, errors.New("telemetry property exceeds limit")
		}
		if startupOnlyProperty(k) && e.Name != "blobfuse.mount.result" ||
			!approvedConfig[k] && !approvedProperties[k] && !validStartupParameter(k, v) {
			return nil, errors.New("unapproved telemetry property")
		}
		switch k {
		case "azstorage.account_name":
			if len(v) < 3 || len(v) > 24 || strings.Trim(v, "abcdefghijklmnopqrstuvwxyz0123456789") != "" {
				return nil, errors.New("invalid storage account name")
			}
		case "error_code":
			if entry, ok := startupFailureCodes[v]; !ok || entry.category != e.Properties["error_category"] {
				return nil, errors.New("unapproved error code")
			}
		}
	}
	for k, v := range e.Measurements {
		if !approvedMeasurements[e.Name][k] {
			return nil, errors.New("unapproved telemetry measurement")
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("invalid telemetry measurement")
		}
		if k == "result" && v != 0 && v != 1 {
			return nil, errors.New("invalid mount result")
		}
	}
	if e.Name == "blobfuse.mount.result" {
		if _, ok := e.Measurements["result"]; !ok {
			return nil, errors.New("missing mount result")
		}
	}
	var item envelope
	item.Name, item.Time, item.Key = "Microsoft.ApplicationInsights.Event", e.Time, key
	item.Data.Type = "EventData"
	item.Data.Data.Version, item.Data.Data.Name = 2, e.Name
	item.Data.Data.Properties, item.Data.Data.Measurements = e.Properties, e.Measurements
	b, err := json.Marshal([]envelope{item})
	if err != nil {
		return nil, errors.New("cannot encode telemetry")
	}
	if len(b) > MaxEventBytes {
		return nil, errors.New("telemetry event exceeds limit")
	}
	return b, nil
}

var approvedProperties = map[string]bool{
	"schema_version": true, "event_id": true,
	"mount_session_id": true, "blobfuse_version": true,
	"failure_stage": true, "error_category": true, "error_code": true, "termination_class": true,
}

var approvedMeasurements = map[string]map[string]bool{
	"blobfuse.mount.result": {
		"mount_latency_ms": true,
		"result":           true,
	},
	"blobfuse.mount.stopped": {
		"final_uptime_seconds":      true,
		"last_known_uptime_seconds": true,
	},
}
