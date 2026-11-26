// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package telemetry

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

type Options struct {
	Disabled         bool
	ConnectionString string
	Version          string
	StartMonotonic   int64
	Diagnostic       func(string)
	HTTPClient       *http.Client
}

type sessionState uint8

const (
	sessionCollecting sessionState = iota
	sessionDetached
	sessionClosed
)

// Session owns only this process's measurements. It is not a supervisor.
type Session struct {
	// Protect session state and result metadata, never the delivery wait.
	mu                 sync.Mutex
	state              sessionState
	id                 string
	version            string
	startMono          int64
	resultRecorded     bool
	config             map[string]string
	explicitParameters map[string]bool
	stage              string
	failure            string
	errorCode          string
	reporter           *reporter
	diag               func(string)
}

var active atomic.Pointer[Session]

func Current() *Session {
	s := active.Load()
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() {
		return nil
	}
	return s
}

func (s *Session) collectingLocked() bool {
	return s.state == sessionCollecting && active.Load() == s
}

func Start(o Options) (*Session, error) {
	diagnostic := o.Diagnostic
	if diagnostic == nil {
		diagnostic = func(string) {}
	}
	if o.Disabled {
		return nil, nil
	}
	if o.ConnectionString == "" {
		diagnostic("telemetry: no Application Insights destination configured; collection inactive")
		return nil, nil
	}
	d, err := parseDestination(o.ConnectionString)
	if err != nil {
		return nil, err
	}
	s := &Session{
		id: newID(), version: o.Version,
		config: map[string]string{}, stage: "configuration", diag: diagnostic,
		state: sessionCollecting,
	}
	s.startMono, err = monotonicNow()
	if err != nil {
		return nil, errors.New("telemetry startup clock unavailable")
	}
	if o.StartMonotonic > 0 && o.StartMonotonic <= s.startMono {
		s.startMono = o.StartMonotonic
	}
	s.reporter = newReporter(d, o.HTTPClient)
	active.Store(s)
	return s, nil
}

func (s *Session) reportLocked(e Event) {
	s.resultRecorded = true
	if err := s.reporter.report(e); err != nil {
		s.diag("telemetry: invalid or oversized event dropped")
	}
}

func (s *Session) StartMonotonic() int64 { return s.startMono }

func monotonicNow() (int64, error) {
	var ts unix.Timespec
	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	return ts.Nano(), err
}

func (s *Session) startupMilliseconds() (float64, bool) {
	now, err := monotonicNow()
	return float64(now-s.startMono) / float64(time.Millisecond), err == nil && now >= s.startMono
}

func (s *Session) resultEvent(result float64) Event {
	e := Event{
		Name: mountResultEvent, Time: time.Now().UTC(),
		Properties: map[string]string{
			"schema_version": schemaVersion, "event_id": newID(),
			"mount_session_id": s.id, "blobfuse_version": s.version,
		},
		Measurements: map[string]float64{"result": result},
	}
	for k, v := range s.config {
		e.Properties[k] = v
	}
	if elapsed, ok := s.startupMilliseconds(); ok {
		e.Measurements["mount_latency_ms"] = elapsed
	} else {
		s.diag("telemetry: startup duration unavailable")
	}
	return e
}

// Configure takes only reviewed effective metadata, never raw config or errors.
func (s *Session) Configure(properties map[string]string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() {
		return
	}
	s.configureLocked(properties)
}

func (s *Session) configureLocked(properties map[string]string) {
	for k, v := range properties {
		if k == "azstorage.account_name" && v != "" && !validStorageAccountName(v) {
			s.diag("telemetry: invalid storage account name omitted")
			delete(s.config, k)
			continue
		}
		if approvedConfig[k] && len(v) <= 256 {
			if v == "" {
				delete(s.config, k)
			} else {
				s.config[k] = v
			}
		}
	}
}

var approvedConfig = map[string]bool{
	"pipeline_components": true, "stream_mode": true, "auth_mode": true,
	"fuse_api_major": true, "libfuse_runtime_version": true,
	"distro_name": true, "distro_version": true, "kernel_release": true,
	"config_source": true, "deployment_environment": true,
	"azstorage.account_type": true, "azstorage.endpoint_type": true, "azstorage.account_name": true,
}

func Stage(stage string) {
	if s := active.Load(); s != nil {
		s.mu.Lock()
		if s.collectingLocked() && !s.resultRecorded {
			s.stage = stage
			s.failure = ""
			s.errorCode = ""
		}
		s.mu.Unlock()
	}
}

func Failure(category string) {
	recordFailure(category, "")
}

func recordFailure(category, code string) {
	if s := active.Load(); s != nil {
		s.mu.Lock()
		if s.collectingLocked() && !s.resultRecorded {
			s.failure = category
			s.errorCode = code
		}
		s.mu.Unlock()
	}
}

// PipelineStartFailed records failure before the pipeline can continue to FUSE readiness.
func PipelineStartFailed() {
	if s := active.Load(); s != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.collectingLocked() && !s.resultRecorded {
			s.stage, s.failure, s.errorCode = "pipeline_start", "component_start_failed", ""
			s.reportFailureLocked()
		}
	}
}

func (s *Session) reportFailureLocked() {
	e := s.resultEvent(0)
	e.Properties["failure_stage"] = s.stage
	category := s.failure
	if category == "" {
		category = "unknown"
	}
	e.Properties["error_category"] = category
	if s.errorCode != "" {
		e.Properties["error_code"] = s.errorCode
	}
	s.reportLocked(e)
}

func Ready(properties map[string]string) {
	s := active.Load()
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() || s.resultRecorded {
		return
	}
	s.configureLocked(properties)
	s.reportLocked(s.resultEvent(1))
}

// Detach stops parent collection but retains its reporter until launch succeeds
// or fails.
func (s *Session) Detach() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() {
		return
	}
	s.state = sessionDetached
}

// HandoffFailed reports a definitive launch failure before a worker exists.
func (s *Session) HandoffFailed() { s.finish(true) }

func (s *Session) Close() { s.finish(false) }

func (s *Session) finish(handoffFailure bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	previous := s.state
	if previous == sessionClosed {
		s.mu.Unlock()
		return
	}
	s.state = sessionClosed
	active.CompareAndSwap(s, nil)
	if !s.resultRecorded && (previous == sessionCollecting || handoffFailure) {
		if handoffFailure {
			s.stage, s.failure, s.errorCode = "daemonization", "", ""
		}
		s.reportFailureLocked()
	}
	budget := FlushTimeout
	if previous == sessionDetached && !handoffFailure {
		budget = 0
	}
	s.mu.Unlock()

	// Wait outside the mutex so late callbacks never block on delivery.
	s.reporter.close(budget)
}
