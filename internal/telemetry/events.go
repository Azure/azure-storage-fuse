// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package telemetry implements optional Microsoft-facing mount telemetry.
package telemetry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	MaxEventBytes       = 32 * 1024
	MaxConcurrentSends  = 8
	MaxStateBytes       = 1024 * 1024
	MaxStateRecordBytes = 64 * 1024
	// Reserve room for one maximum-size recovery checkpoint within the hard cap.
	RecoveryReserveBytes = MaxStateRecordBytes
	MaxStateRecords      = 512
	Retention            = 7 * 24 * time.Hour
	SendTimeout          = 5 * time.Second
	FlushTimeout         = 5 * time.Second
)

// Event is the internal, destination-independent contract. IDs survive retries.
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
	case "blobfuse.mount.attempted", "blobfuse.mount.result", "blobfuse.mount.snapshot",
		"blobfuse.mount.stopped", "blobfuse.mount.recovery":
	default:
		return nil, errors.New("invalid telemetry event")
	}
	if e.Properties["schema_version"] != "1" || e.Time.IsZero() ||
		!validID(e.Properties["event_id"]) || !validID(e.Properties["mount_session_id"]) {
		return nil, errors.New("invalid telemetry identity")
	}
	properties := make(map[string]string, len(e.Properties))
	for k, v := range e.Properties {
		if len(v) > 1024 {
			return nil, errors.New("telemetry property exceeds limit")
		}
		if retiredProperties[k] {
			continue
		}
		if !approvedConfig[k] && !approvedProperties[k] {
			return nil, errors.New("unapproved telemetry property")
		}
		properties[k] = v
	}
	for _, v := range e.Measurements {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("invalid telemetry measurement")
		}
	}
	var item envelope
	item.Name, item.Time, item.Key = "Microsoft.ApplicationInsights.Event", e.Time, key
	item.Data.Type = "EventData"
	item.Data.Data.Version, item.Data.Data.Name = 2, e.Name
	item.Data.Data.Properties, item.Data.Data.Measurements = properties, e.Measurements
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
	"config_fingerprint": true, "outcome": true, "failure_stage": true,
	"failure_category": true, "termination_class": true,
	"sequence_number": true, "counter_scope": true,
	"snapshot_id": true, "part_index": true, "part_count": true,
	"previous_session_id": true, "previous_ready": true,
	"fuse_counters_available": true,
}

// Older retained events remain replayable, without uploading retired metadata.
var retiredProperties = map[string]bool{
	"observation_source": true, "shutdown_outcome": true, "attr_cache_enabled": true,
	"http_scope": true, "fuse_observation_scope": true, "retry_exhaustion_available": true,
	"mount_attempt_id": true, "counter_encoding": true, "retry_scope": true,
	"exact_backoff_available": true,
}

// splitSnapshot retains every measurement and identifies parts of one snapshot.
func splitSnapshot(e Event, key string) ([]Event, error) {
	keys := make([]string, 0, len(e.Measurements))
	for k := range e.Measurements {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	newPart := func() Event {
		p := make(map[string]string, len(e.Properties)+3)
		for k, v := range e.Properties {
			p[k] = v
		}
		p["snapshot_id"], p["event_id"] = e.Properties["event_id"], newID()
		p["part_index"], p["part_count"] = "9999", "9999"
		return Event{Name: e.Name, Time: e.Time, Properties: p, Measurements: map[string]float64{}}
	}
	current := newPart()
	var parts []Event
	for _, k := range keys {
		current.Measurements[k] = e.Measurements[k]
		if _, err := encode(current, key); err != nil {
			delete(current.Measurements, k)
			if len(current.Measurements) == 0 {
				return nil, err
			}
			parts = append(parts, current)
			current = newPart()
			current.Measurements[k] = e.Measurements[k]
			if _, err := encode(current, key); err != nil {
				return nil, err
			}
		}
	}
	parts = append(parts, current)
	for i := range parts {
		parts[i].Properties["part_index"] = strconv.Itoa(i + 1)
		parts[i].Properties["part_count"] = strconv.Itoa(len(parts))
	}
	return parts, nil
}

func fingerprint(properties map[string]string) string {
	b, _ := json.Marshal(properties)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}
