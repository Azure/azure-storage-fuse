// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"encoding/json"
	"maps"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testEvent() Event {
	return Event{Name: "blobfuse.mount.result", Time: time.Now().UTC(),
		Properties: map[string]string{"schema_version": schemaVersion, "event_id": newID(),
			"mount_session_id": newID()},
		Measurements: map[string]float64{"result": 1}}
}

func TestEncodeRejectsUnapprovedSchema(t *testing.T) {
	e := testEvent()
	e.Name = "unexpected.event"
	if _, err := encode(e, "test-key"); err == nil {
		t.Fatal("encoder accepted an unapproved event")
	}
	e.Name = "blobfuse.mount.result"
	for _, field := range []string{"account_name", "raw_error"} {
		e.Properties[field] = "not-allowed"
		if _, err := encode(e, "test-key"); err == nil {
			t.Fatalf("encoder accepted unapproved property %s", field)
		}
		delete(e.Properties, field)
	}
}

func TestStartupFailureCodeReporting(t *testing.T) {
	for _, tc := range []struct {
		name, category, code string
		record               func()
	}{
		{"storage-code", "container_not_found", "ContainerNotFound", func() { StorageFailureCode("ContainerNotFound") }},
		{"identity-status", "authentication", "403", func() { IdentityFailureStatus(403) }},
		{"unknown-storage-code", "unknown", "", func() { StorageFailureCode("InvalidQueryParameterValue") }},
		{"identity-status-as-storage-code", "unknown", "", func() { StorageFailureCode("403") }},
		{"unknown-status-preserves-category", "dns_resolution", "", func() {
			Failure("dns_resolution")
			IdentityFailureStatus(500)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, events := recordingSession(t, Options{})
			tc.record()
			s.Close()

			require.Len(t, events, 1)
			e := (<-events).Data.Data
			require.Equal(t, "blobfuse.mount.result", e.Name)
			require.Equal(t, float64(0), e.Measurements["result"])
			require.Equal(t, tc.category, e.Properties["error_category"])
			if tc.code == "" {
				require.NotContains(t, e.Properties, "error_code")
			} else {
				require.Equal(t, tc.code, e.Properties["error_code"])
			}
		})
	}
}

func TestEncodedEventSizeBoundary(t *testing.T) {
	const limit = 32 * 1024
	e := testEvent()
	e.Properties["error_category"] = ""
	// JSON escaping reaches the envelope cap using only approved fields.
	for _, field := range []string{"pipeline_components", "auth_mode", "distro_name", "distro_version", "kernel_release"} {
		e.Properties[field] = strings.Repeat("<", 1024)
	}
	body, err := encode(e, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	padding := limit - len(body)
	if padding < 0 || padding/6+padding%6 >= 1024 {
		t.Fatalf("fixture cannot reach size boundary within property limits: padding=%d", padding)
	}
	e.Properties["error_category"] = strings.Repeat("<", padding/6) + strings.Repeat("x", padding%6)
	body, err = encode(e, "test-key")
	if err != nil || len(body) != limit {
		t.Fatalf("32-KiB encoded event rejected: bytes=%d err=%v", len(body), err)
	}
	e.Properties["error_category"] += "x"
	if _, err := encode(e, "test-key"); err == nil || err.Error() != "telemetry event exceeds limit" {
		t.Fatalf("expected event-size rejection above 32 KiB: %v", err)
	}
}

func TestMountResultMeasurements(t *testing.T) {
	e := testEvent()
	e.Measurements = map[string]float64{"mount_latency_ms": 12.5}
	_, err := encode(e, "test-key")
	require.EqualError(t, err, "missing mount result")
	e.Measurements["result"] = 2
	_, err = encode(e, "test-key")
	require.EqualError(t, err, "invalid mount result")

	for _, result := range []float64{0, 1} {
		e.Measurements["result"] = result
		want := maps.Clone(e.Measurements)
		body, err := encode(e, "test-key")
		require.NoError(t, err)
		var batch []envelope
		require.NoError(t, json.Unmarshal(body, &batch))
		require.Len(t, batch, 1)
		require.Equal(t, want, batch[0].Data.Data.Measurements)
		require.Equal(t, want, e.Measurements, "encoding mutated the input event")
	}

	e.Measurements["unexpected_measurement"] = 1
	body, err := encode(e, "test-key")
	require.EqualError(t, err, "unapproved telemetry measurement")
	require.Nil(t, body)
}

func TestMeasurementValuesMustBeFinite(t *testing.T) {
	for _, tc := range []struct {
		event, measurement string
	}{
		{"blobfuse.mount.result", "mount_latency_ms"},
	} {
		t.Run(tc.measurement, func(t *testing.T) {
			e := testEvent()
			e.Name = tc.event
			for _, value := range []float64{0, 12.5, math.MaxFloat64, math.NaN(), math.Inf(1), math.Inf(-1)} {
				e.Measurements = map[string]float64{tc.measurement: value}
				if tc.event == "blobfuse.mount.result" {
					e.Measurements["result"] = 1
				}
				body, err := encode(e, "test-key")
				if math.IsNaN(value) || math.IsInf(value, 0) {
					if err == nil || err.Error() != "invalid telemetry measurement" || body != nil {
						t.Fatalf("nonfinite measurement accepted: value=%v err=%v", value, err)
					}
				} else if err != nil {
					t.Fatalf("finite measurement rejected: value=%v err=%v", value, err)
				}
			}
		})
	}
}

func TestDestinationValidation(t *testing.T) {
	const base = "https://eastus-8.in.applicationinsights.azure.com"
	for _, endpoint := range []string{
		"http://eastus-8.in.applicationinsights.azure.com",
		"https://example.com", "https://localhost", "https://127.0.0.1", "https://[::1]",
		base + ".example.invalid", "https://notapplicationinsights.azure.com",
		"https://in.applicationinsights.azure.com", "https://.in.applicationinsights.azure.com",
		"https://extra.eastus-8.in.applicationinsights.azure.com",
		"https://-eastus.in.applicationinsights.azure.com",
		"https://eastus-.in.applicationinsights.azure.com",
		"https://east_us.in.applicationinsights.azure.com",
		"https://" + strings.Repeat("a", 64) + ".in.applicationinsights.azure.com",
		"https://eastus-8.in.applicationinsights.azure.com.",
		"https://dc.services.visualstudio.com", "https://dc.applicationinsights.azure.com",
		"https://eastus.in.applicationinsights.azure.cn", "https://dc.applicationinsights.us",
		base + ":8443", base + ":", base + "/custom", base + "/v2.1/track",
		base + "/%2f", base + "?query=value", base + "?", base + "#fragment", base + "#",
		"https://user:password@eastus-8.in.applicationinsights.azure.com",
	} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := parseDestination("InstrumentationKey=x;IngestionEndpoint=" + endpoint)
			require.Error(t, err, "accepted unsupported ingestion endpoint")
		})
	}
	for _, value := range []string{
		"", "InstrumentationKey=x;IngestionEndpoint=http://localhost",
		"InstrumentationKey=x;IngestionEndpoint=https://user:pass@host",
		"InstrumentationKey=x;IngestionEndpoint=https://host?q=x",
		"InstrumentationKey=x;IngestionEndpoint=https://host;Authorization=aad",
	} {
		if _, err := parseDestination(value); err == nil {
			t.Fatalf("accepted unsafe destination: %s", value)
		}
	}
	for _, endpoint := range []string{
		base, base + "/", base + ":443/",
		"https://EASTUS-8.IN.APPLICATIONINSIGHTS.AZURE.COM/",
		"https://southcentralus.in.applicationinsights.azure.com/",
		"https://a.in.applicationinsights.azure.com",
		"https://" + strings.Repeat("a", 63) + ".in.applicationinsights.azure.com",
	} {
		d, err := parseDestination("InstrumentationKey=x;IngestionEndpoint=" + endpoint)
		require.NoError(t, err)
		require.Equal(t, strings.TrimRight(strings.ToLower(endpoint), "/")+"/v2.1/track", d.endpoint)
		require.Equal(t, "x", d.key)
	}
}

func TestStartRejectsNonMicrosoftDestinationBeforeDelivery(t *testing.T) {
	s, err := Start(Options{
		ConnectionString: "InstrumentationKey=x;IngestionEndpoint=https://example.invalid",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("invalid destination reached delivery")
			return testResponse(200), nil
		})},
	})
	if s != nil {
		s.Close()
	}
	require.Error(t, err)
	require.Nil(t, s)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestInactiveSession(t *testing.T) {
	for _, options := range []Options{{}, {Disabled: true}} {
		s, err := Start(options)
		if err != nil || s != nil {
			t.Fatalf("inactive session: %v", err)
		}
	}
}
