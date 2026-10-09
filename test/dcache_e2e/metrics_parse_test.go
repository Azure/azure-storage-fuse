//go:build !unittest
// +build !unittest

/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2026 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package dcache_e2e

import (
	"math"
	"testing"
)

func TestParseCacheServerMetrics(t *testing.T) {
	body := `# HELP cache_server_request_counter Counter of requests.
# TYPE cache_server_request_counter counter
cache_server_request_counter{instance="cacheserver-0",request_type="Download",status="Success"} 17
cache_server_request_counter{instance="cacheserver-0",request_type="Download",status="InvalidTransition"} 3
cache_server_request_counter{instance="cacheserver-0",request_type="Upload",status="Success"} 5
cache_server_request_counter{instance="cacheserver-0",request_type="Upload",status="InvalidTransition"} 2
cache_server_request_counter{instance="cacheserver-0",request_type="Download",status="OtherStatus"} 99
cache_server_request_counter{instance="cacheserver-0",request_type="Register",status="Success"} 42
some_other_metric{foo="bar"} 100
cache_server_request_counter{instance="cacheserver-0",request_type="Download",status="Success"} not-a-number
`

	got := parseCacheServerMetrics(body)
	want := CacheServerMetrics{
		DownloadSuccess:           17,
		DownloadInvalidTransition: 3,
		UploadSuccess:             5,
		UploadInvalidTransition:   2,
	}
	if got != want {
		t.Fatalf("parseCacheServerMetrics: got %+v want %+v", got, want)
	}
}

func TestParseCacheServerMetrics_FractionalTruncated(t *testing.T) {
	body := `cache_server_request_counter{instance="a",request_type="Download",status="Success"} 4.9
cache_server_request_counter{instance="b",request_type="Download",status="Success"} 3.1
`
	got := parseCacheServerMetrics(body)
	// 4.9 → 4, 3.1 → 3, summed → 7.
	if got.DownloadSuccess != 7 {
		t.Errorf("fractional values: DownloadSuccess = %d, want 7", got.DownloadSuccess)
	}
}

func TestParseCacheServerMetrics_Empty(t *testing.T) {
	if got := parseCacheServerMetrics(""); got != (CacheServerMetrics{}) {
		t.Errorf("empty body: got %+v, want zero", got)
	}
}

func TestParseCacheServerMetrics_MultiplePodsAggregate(t *testing.T) {
	// The parser only handles a single body; multi-pod aggregation happens
	// in scrapeCacheServerMetrics. Verify that calling the parser twice
	// against different bodies and summing gives the expected total, which
	// is what the scraper does internally.
	bodyA := `cache_server_request_counter{instance="a",request_type="Download",status="Success"} 10
cache_server_request_counter{instance="a",request_type="Upload",status="Success"} 4
`
	bodyB := `cache_server_request_counter{instance="b",request_type="Download",status="Success"} 7
cache_server_request_counter{instance="b",request_type="Download",status="InvalidTransition"} 1
`
	a := parseCacheServerMetrics(bodyA)
	b := parseCacheServerMetrics(bodyB)
	total := CacheServerMetrics{
		DownloadSuccess:           a.DownloadSuccess + b.DownloadSuccess,
		DownloadInvalidTransition: a.DownloadInvalidTransition + b.DownloadInvalidTransition,
		UploadSuccess:             a.UploadSuccess + b.UploadSuccess,
		UploadInvalidTransition:   a.UploadInvalidTransition + b.UploadInvalidTransition,
	}
	want := CacheServerMetrics{DownloadSuccess: 17, DownloadInvalidTransition: 1, UploadSuccess: 4}
	if total != want {
		t.Errorf("aggregate: got %+v want %+v", total, want)
	}
}

func TestDeltaCacheMetrics(t *testing.T) {
	before := CacheServerMetrics{DownloadSuccess: 10, DownloadInvalidTransition: 2, UploadSuccess: 3, UploadInvalidTransition: 1}
	after := CacheServerMetrics{DownloadSuccess: 15, DownloadInvalidTransition: 2, UploadSuccess: 8, UploadInvalidTransition: 4}
	got := deltaCacheMetrics(before, after)
	want := CacheServerMetrics{DownloadSuccess: 5, DownloadInvalidTransition: 0, UploadSuccess: 5, UploadInvalidTransition: 3}
	if got != want {
		t.Errorf("delta: got %+v want %+v", got, want)
	}
}

func TestHitMissRatio(t *testing.T) {
	cases := []struct {
		name   string
		d      CacheServerMetrics
		hits   int
		misses int
		ratio  float64
	}{
		{"all hits", CacheServerMetrics{DownloadSuccess: 10}, 10, 0, 1.0},
		{"all misses", CacheServerMetrics{DownloadInvalidTransition: 5}, 0, 5, 0.0},
		{"mixed", CacheServerMetrics{DownloadSuccess: 3, DownloadInvalidTransition: 1}, 3, 1, 0.75},
		{"none", CacheServerMetrics{}, 0, 0, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits, misses, ratio := hitMissRatio(tc.d)
			if hits != tc.hits || misses != tc.misses || math.Abs(ratio-tc.ratio) > 1e-9 {
				t.Errorf("hitMissRatio(%+v) = (%d, %d, %v), want (%d, %d, %v)",
					tc.d, hits, misses, ratio, tc.hits, tc.misses, tc.ratio)
			}
		})
	}
}

func TestLastFieldInt(t *testing.T) {
	cases := []struct {
		line string
		want int
		ok   bool
	}{
		{`foo{a="b"} 42`, 42, true},
		{`foo{a="b"} 3.9`, 3, true},
		{`foo{a="b"} not-a-number`, 0, false},
		{`foo{a="b"}`, 0, false},
		{``, 0, false},
	}
	for _, tc := range cases {
		got, ok := lastFieldInt(tc.line)
		if got != tc.want || ok != tc.ok {
			t.Errorf("lastFieldInt(%q) = (%d, %v), want (%d, %v)", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}
