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
	SessionID        string
	StartMonotonic   int64
	AttemptTime      time.Time
	DeferAttempt     bool
	Diagnostic       func(string)
	HTTPClient       *http.Client
}

type sessionState uint8

const (
	sessionCollecting sessionState = iota
	sessionDetached
	sessionClosing
	sessionClosed
)

// Session owns only this process's measurements. It is not a supervisor.
type Session struct {
	// Protect lifecycle and event metadata, never the delivery wait.
	mu               sync.Mutex
	state            sessionState
	closeDone        chan struct{}
	id               string
	version          string
	startMono        int64
	attemptTime      time.Time
	attemptReported  bool
	ready            time.Time
	teardownComplete bool
	config           map[string]string
	stage            string
	failure          string
	reporter         *reporter
	diag             func(string)
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
		attemptTime: time.Now().UTC(),
		config:      map[string]string{}, stage: "configuration", diag: diagnostic,
		state: sessionCollecting, closeDone: make(chan struct{}),
	}
	if !o.AttemptTime.IsZero() {
		s.attemptTime = o.AttemptTime.UTC()
	}
	s.startMono, err = monotonicNow()
	if err != nil {
		return nil, errors.New("telemetry startup clock unavailable")
	}
	if o.StartMonotonic > 0 && o.StartMonotonic <= s.startMono {
		s.startMono = o.StartMonotonic
	}
	if validID(o.SessionID) {
		s.id = o.SessionID
	}
	s.reporter = newReporter(d, o.HTTPClient)
	active.Store(s)
	if !o.DeferAttempt {
		s.ReportAttempt()
	}
	return s, nil
}

// ReportAttempt claims delivery for a foreground invocation once its mode is known.
func (s *Session) ReportAttempt() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() {
		return
	}
	s.reportAttemptLocked()
}

func (s *Session) reportAttemptLocked() {
	if s.attemptReported {
		return
	}
	s.attemptReported = true
	e := s.event("attempted")
	e.Time = s.attemptTime
	s.reportLocked(e)
}

func (s *Session) reportLocked(e Event) {
	if err := s.reporter.report(e); err != nil {
		s.diag("telemetry: invalid or oversized event dropped")
	}
}

func (s *Session) Identity() string { return s.id }

func (s *Session) StartMonotonic() int64 { return s.startMono }

func (s *Session) AttemptTime() time.Time { return s.attemptTime }

func monotonicNow() (int64, error) {
	var ts unix.Timespec
	err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	return ts.Nano(), err
}

func (s *Session) startupMilliseconds() (float64, bool) {
	now, err := monotonicNow()
	return float64(now-s.startMono) / float64(time.Millisecond), err == nil && now >= s.startMono
}

func (s *Session) event(kind string) Event {
	return Event{
		Name: "blobfuse.mount." + kind, Time: time.Now().UTC(),
		Properties: map[string]string{
			"schema_version": "1", "event_id": newID(),
			"mount_session_id": s.id, "blobfuse_version": s.version,
		},
		Measurements: map[string]float64{},
	}
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
		if approvedConfig[k] && len(v) <= 256 {
			s.config[k] = v
		}
	}
}

var approvedConfig = map[string]bool{
	"pipeline_components": true, "stream_mode": true, "auth_mode": true,
	"fuse_api_major": true, "libfuse_runtime_version": true,
	"distro_name": true, "distro_version": true, "kernel_release": true,
	"attr_cache_ttl_seconds": true, "kernel_attr_timeout_seconds": true,
}

func Stage(stage string) {
	if s := active.Load(); s != nil {
		s.mu.Lock()
		if s.collectingLocked() {
			s.stage = stage
			s.failure = ""
		}
		s.mu.Unlock()
	}
}

func Failure(category string) {
	if s := active.Load(); s != nil {
		s.mu.Lock()
		if s.collectingLocked() && s.ready.IsZero() {
			s.failure = category
		}
		s.mu.Unlock()
	}
}

func Ready(properties map[string]string) {
	s := active.Load()
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.collectingLocked() || !s.ready.IsZero() {
		return
	}
	s.configureLocked(properties)
	s.ready = time.Now()
	e := s.event("result")
	for k, v := range s.config {
		e.Properties[k] = v
	}
	e.Properties["outcome"] = "success"
	if elapsed, ok := s.startupMilliseconds(); ok {
		e.Measurements["time_to_mount_ms"] = elapsed
	} else {
		s.diag("telemetry: startup duration unavailable")
	}
	s.reportLocked(e)
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

// TeardownComplete must be called only after every pipeline component stopped.
func TeardownComplete() {
	if s := active.Load(); s != nil {
		s.mu.Lock()
		if s.collectingLocked() && !s.ready.IsZero() {
			s.teardownComplete = true
		}
		s.mu.Unlock()
	}
}

func (s *Session) finish(handoffFailure bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.state == sessionClosed {
		s.mu.Unlock()
		return
	}
	if s.state == sessionClosing {
		s.mu.Unlock()
		// Share the bounded session close, not potentially late worker cleanup.
		<-s.closeDone
		return
	}
	previous := s.state
	s.state = sessionClosing
	if previous == sessionCollecting || previous == sessionDetached && handoffFailure {
		// Before ownership transfers, a failed parent owns both startup events.
		s.reportAttemptLocked()
		if handoffFailure {
			s.stage, s.failure = "daemonization", ""
		}
		kind := "stopped"
		if s.ready.IsZero() {
			kind = "result"
		}
		e := s.event(kind)
		if s.ready.IsZero() {
			e.Properties["outcome"] = "failure"
			e.Properties["failure_stage"] = s.stage
			category := s.failure
			if category == "" {
				category = "unknown"
			}
			e.Properties["failure_category"] = category
			if elapsed, ok := s.startupMilliseconds(); ok {
				e.Measurements["time_to_failure_ms"] = elapsed
			} else {
				s.diag("telemetry: startup duration unavailable")
			}
		} else {
			e.Properties["termination_class"] = "unclean_unknown"
			if s.teardownComplete {
				e.Properties["termination_class"] = "clean"
			}
			uptimeField := "last_known_uptime_seconds"
			if e.Properties["termination_class"] == "clean" {
				uptimeField = "final_uptime_seconds"
			}
			e.Measurements[uptimeField] = time.Since(s.ready).Seconds()
		}
		for k, v := range s.config {
			e.Properties[k] = v
		}
		s.reportLocked(e)
	}
	budget := FlushTimeout
	if previous == sessionDetached && !handoffFailure {
		budget = 0
	}
	s.mu.Unlock()

	s.reporter.close(budget)

	s.mu.Lock()
	s.state = sessionClosed
	active.CompareAndSwap(s, nil)
	close(s.closeDone)
	s.mu.Unlock()
}
