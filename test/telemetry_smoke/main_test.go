// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVerifierLimits(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"itemsReceived":1,"itemsAccepted":1,"errors":[]}`)
	}))
	defer upstream.Close()
	v := &verifier{key: "test", endpoint: upstream.URL, path: "/test", client: upstream.Client()}
	body := `{"iKey":"test","data":{"baseData":{"name":"blobfuse.mount.result","properties":{},"measurements":{}}}}`
	for i := 0; i < maxRequests+1; i++ {
		w := httptest.NewRecorder()
		v.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("["+body+"]")))
		want := 200
		if i == maxRequests {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("request %d: status %d want %d", i, w.Code, want)
		}
	}
	if calls.Load() != maxRequests || v.bytes > maxBytes || len(v.receipts) != maxRequests {
		t.Fatal("upstream request/volume cap failed")
	}
	for _, result := range v.receipts {
		if result.Accepted != 1 || result.Status != 200 {
			t.Fatal("acceptance acknowledgement lost")
		}
	}
	w := httptest.NewRecorder()
	v.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(strings.Repeat("x", maxItem+1))))
	if w.Code != 413 || calls.Load() != maxRequests {
		t.Fatal("oversized payload forwarded")
	}
	w = httptest.NewRecorder()
	v.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/wrong-path", strings.NewReader("["+body+"]")))
	if w.Code != 404 {
		t.Fatal("unexpected path accepted")
	}
}

func TestVerifierLargeEventsRespectTotalBudget(t *testing.T) {
	var calls atomic.Int64
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"itemsReceived":1,"itemsAccepted":1,"errors":[]}`)
	}))
	defer upstream.Close()
	v := &verifier{key: "test", endpoint: upstream.URL, path: "/test", client: upstream.Client()}
	payload := `[{"iKey":"test","data":{"baseData":{"name":"blobfuse.mount.snapshot","properties":{},"measurements":{}}}}]`
	body := payload + strings.Repeat(" ", 32*1024-len(payload))
	for i, want := range []int{200, 200, 429} {
		w := httptest.NewRecorder()
		v.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body)))
		if w.Code != want {
			t.Fatalf("large request %d: status=%d want=%d", i, w.Code, want)
		}
	}
	if calls.Load() != 2 || v.bytes != 64*1024 {
		t.Fatalf("total forwarding budget changed: requests=%d bytes=%d", calls.Load(), v.bytes)
	}
	w := httptest.NewRecorder()
	v.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body+" ")))
	if w.Code != 413 || calls.Load() != 2 {
		t.Fatal("event above 32 KiB was forwarded")
	}
}

func TestConnectionStringValidation(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"", false},
		{"InstrumentationKey=test;IngestionEndpoint=https://example.test/", true},
		{"InstrumentationKey=test;IngestionEndpoint=http://example.test/", false},
		{"InstrumentationKey=test;IngestionEndpoint=https://user:pass@example.test/", false},
		{"InstrumentationKey=test;IngestionEndpoint=https://example.test/?q=1", false},
	} {
		key, endpoint, err := connection(tc.value)
		if (err == nil) != tc.valid {
			t.Fatalf("valid=%t error=%v", tc.valid, err)
		}
		if tc.valid && (key != "test" || endpoint != "https://example.test/v2.1/track") {
			t.Fatal("incorrect destination")
		}
	}
}
