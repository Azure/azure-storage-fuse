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
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// metricsHTTPTimeout bounds a single scrape. Cache-server metric endpoints
// are lightweight; anything over a second is a symptom of a broken
// port-forward rather than a slow server.
const metricsHTTPTimeout = 3 * time.Second

// metricSnapshot is the aggregate value of every counter/gauge exposed by
// every cache-server endpoint at a point in time. Keys are metric names
// (labels stripped and values summed across label combinations). Values are
// the sum across all endpoints. This shape lets us compute a delta between
// two snapshots without caring about label cardinality or which server
// received which chunk -- both of which vary across runs because of the
// consistent-hash ring.
type metricSnapshot struct {
	values map[string]float64
	// timestamps used only for debug logging
	scrapedAt time.Time
	source    []string // endpoints that contributed to this snapshot
}

// scrapeMetrics fetches Prometheus text-format output from every endpoint
// in testCfg.metricsEndpoints and returns their aggregated snapshot. It
// returns (nil, false) when the pipeline did not expose metrics -- the
// caller should treat that as "skip metric assertions" rather than "fail".
func scrapeMetrics(t *testing.T) (*metricSnapshot, bool) {
	t.Helper()
	if len(testCfg.metricsEndpoints) == 0 {
		return nil, false
	}
	snap := &metricSnapshot{
		values:    make(map[string]float64),
		scrapedAt: time.Now(),
		source:    testCfg.metricsEndpoints,
	}
	client := &http.Client{Timeout: metricsHTTPTimeout}
	for _, ep := range testCfg.metricsEndpoints {
		url := strings.TrimRight(ep, "/") + "/metrics"
		if err := scrapeInto(client, url, snap.values); err != nil {
			// Metrics are diagnostic; a single bad endpoint should not
			// tank the whole test. Log and continue.
			t.Logf("metrics scrape %s failed: %v", url, err)
		}
	}
	return snap, true
}

// scrapePerEndpoint returns one counter map per endpoint in
// testCfg.metricsEndpoints, preserving order. A failed scrape yields nil
// for that endpoint's slot (not an error) -- callers that assert against a
// specific endpoint interpret nil as "endpoint silent", which is the
// natural outcome after a pod has been force-deleted and its
// `kubectl port-forward` has torn down.
//
// Returns (nil, false) when no metric endpoints were configured -- the
// caller should skip per-endpoint assertions in that case, exactly like
// scrapeMetrics does for aggregated ones.
func scrapePerEndpoint(t *testing.T) ([]map[string]float64, bool) {
	t.Helper()
	if len(testCfg.metricsEndpoints) == 0 {
		return nil, false
	}
	out := make([]map[string]float64, len(testCfg.metricsEndpoints))
	client := &http.Client{Timeout: metricsHTTPTimeout}
	for i, ep := range testCfg.metricsEndpoints {
		url := strings.TrimRight(ep, "/") + "/metrics"
		values := make(map[string]float64)
		if err := scrapeInto(client, url, values); err != nil {
			t.Logf("metrics scrape %s failed: %v (endpoint treated as silent)", url, err)
			out[i] = nil
			continue
		}
		out[i] = values
	}
	return out, true
}

// deltaOne computes after - before for two per-endpoint snapshots. Either
// side may be nil (endpoint was silent for that scrape); nil is treated
// as "all counters at zero" for the purposes of the diff, so a nil→nil
// endpoint produces an empty (all-zero) delta and the survivor/victim
// assertions can be uniformly expressed as "hit-family sum > 0" vs "<= 0".
func deltaOne(before, after map[string]float64) map[string]float64 {
	if before == nil && after == nil {
		return map[string]float64{}
	}
	if before == nil {
		out := make(map[string]float64, len(after))
		for k, v := range after {
			out[k] = v
		}
		return out
	}
	if after == nil {
		out := make(map[string]float64, len(before))
		for k, v := range before {
			out[k] = -v
		}
		return out
	}
	out := make(map[string]float64, len(after))
	for k, v := range after {
		out[k] = v - before[k]
	}
	for k, v := range before {
		if _, ok := after[k]; !ok {
			out[k] = -v
		}
	}
	return out
}

// scrapeInto fetches a single endpoint's metrics and folds every numeric
// sample it exposes into `into`, summing across label combinations.
func scrapeInto(client *http.Client, url string, into map[string]float64) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	sc := bufio.NewScanner(resp.Body)
	// Prometheus exposition lines can be long (label-heavy). Bump the
	// scanner buffer to accommodate them without triggering ErrTooLong.
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		name, val, ok := parseMetricLine(sc.Text())
		if !ok {
			continue
		}
		into[name] += val
	}
	return sc.Err()
}

// parseMetricLine parses one non-comment line of the Prometheus text
// exposition format. Returns the metric's base name (label set stripped),
// its numeric value, and whether the line was a sample.
//
// The spec allows an optional trailing timestamp; we ignore it because we
// only diff counter/gauge values, never observe rate over wall time inside
// the process. See:
//
//	https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
func parseMetricLine(line string) (string, float64, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", 0, false
	}
	// Split base name from the rest. The name ends at the first space, '{',
	// or end of line.
	end := len(line)
	for i, r := range line {
		if r == '{' || r == ' ' || r == '\t' {
			end = i
			break
		}
	}
	name := line[:end]
	rest := strings.TrimSpace(line[end:])
	// Skip over labels if present.
	if strings.HasPrefix(rest, "{") {
		close := strings.Index(rest, "}")
		if close < 0 {
			return "", 0, false
		}
		rest = strings.TrimSpace(rest[close+1:])
	}
	// The value token is the first whitespace-separated field.
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		// Prometheus permits +Inf / -Inf / NaN literals -- treat as
		// non-samples for our aggregation purposes.
		return "", 0, false
	}
	return name, v, true
}

// delta returns after - before for every metric present in either snapshot.
// A cache-server pod restart between scrapes can drop a counter's value;
// that shows up as a negative delta.
func delta(before, after *metricSnapshot) map[string]float64 {
	out := make(map[string]float64, len(after.values))
	for k, v := range after.values {
		out[k] = v - before.values[k]
	}
	for k, v := range before.values {
		if _, ok := after.values[k]; !ok {
			out[k] = -v
		}
	}
	return out
}

// sumMatching returns the sum of counter deltas whose name contains any of
// `substrings` (case-insensitive). Callers should assert `sum > 0` and log
// the matching metric names for diagnosability.
func sumMatching(d map[string]float64, substrings ...string) (float64, []string) {
	var total float64
	var matched []string
	lower := make([]string, len(substrings))
	for i, s := range substrings {
		lower[i] = strings.ToLower(s)
	}
	for name, v := range d {
		nl := strings.ToLower(name)
		for _, s := range lower {
			if strings.Contains(nl, s) {
				total += v
				matched = append(matched, fmt.Sprintf("%s=%g", name, v))
				break
			}
		}
	}
	return total, matched
}

// logTopDeltas prints the k metrics with the largest absolute delta between
// two snapshots. Diagnostic helper used from failing tests so an operator
// can see whether *some* counter moved even if it did not match the
// substring filter.
func logTopDeltas(t *testing.T, d map[string]float64, k int) {
	t.Helper()
	type kv struct {
		name string
		val  float64
	}
	kvs := make([]kv, 0, len(d))
	for n, v := range d {
		if v == 0 {
			continue
		}
		kvs = append(kvs, kv{n, v})
	}
	// Simple partial sort: not worth pulling in sort for k=20.
	for i := 0; i < len(kvs); i++ {
		for j := i + 1; j < len(kvs); j++ {
			if absVal(kvs[j].val) > absVal(kvs[i].val) {
				kvs[i], kvs[j] = kvs[j], kvs[i]
			}
		}
	}
	if k > len(kvs) {
		k = len(kvs)
	}
	if k == 0 {
		t.Logf("metric delta: no counters moved")
		return
	}
	var sb strings.Builder
	sb.WriteString("top metric deltas:")
	for i := 0; i < k; i++ {
		fmt.Fprintf(&sb, "\n  %s = %g", kvs[i].name, kvs[i].val)
	}
	t.Log(sb.String())
}

func absVal(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
