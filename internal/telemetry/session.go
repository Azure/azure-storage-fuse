// Copyright (c) 2026 Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package telemetry

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

type Options struct {
	Disabled         bool
	ConnectionString string
	StateDirectory   string
	MountPath        string
	MountScope       string
	Interval         time.Duration
	Version          string
	SessionID        string
	StartedAt        time.Time
	StartMonotonic   int64
	Child            bool
	Diagnostic       func(string)
	HTTPClient       *http.Client
}

// Session owns only this process's measurements. It is not a supervisor.
type Session struct {
	lifecycleMu      sync.Mutex
	disabled         atomic.Bool
	detached         atomic.Bool
	finishing        atomic.Bool
	closed           bool
	mu               sync.Mutex
	id               string
	version          string
	start            time.Time
	startMono        int64
	ready            time.Time
	teardownComplete bool
	config           map[string]string
	stage            string
	failure          string
	sequence         uint64
	record           sessionRecord
	store            *stateStore
	stateDirectory   string
	mountPath        string
	mountScope       string
	reporter         *reporter
	stop             chan struct{}
	done             chan struct{}
	readySignal      chan struct{}
	once             sync.Once
	diag             func(string)
	lastDiag         atomic.Int64
	native           func() map[string]float64
	onDisable        func()
	http             [2][600]atomic.Uint64
	attempts         [2]atomic.Uint64
	noResponse       [2]atomic.Uint64
	retries          [2]atomic.Uint64
	retried          [2]atomic.Uint64
	requests         [2]atomic.Uint64
	failed           [2]atomic.Uint64
	network          [2][8]atomic.Uint64
	running          atomic.Bool
	baseline         map[string]float64
	child            bool
}

var active atomic.Pointer[Session]

func Current() *Session {
	s := active.Load()
	if s == nil || s.disabled.Load() || s.detached.Load() || s.finishing.Load() {
		return nil
	}
	return s
}

// DisableCurrent also reaches the launching parent's detached policy owner.
func DisableCurrent() {
	if s := active.Load(); s != nil {
		s.Disable()
	}
}

func Start(o Options) (*Session, error) {
	diagnostic := o.Diagnostic
	if diagnostic == nil {
		diagnostic = func(string) {}
	}
	if o.Disabled {
		store, err := openMountStore(o.StateDirectory, o.MountPath, o.MountScope, false, diagnostic)
		if err != nil {
			return nil, err
		}
		if store != nil {
			defer store.root.Close()
			return nil, store.purge()
		}
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
	if o.Interval == 0 {
		o.Interval = 5 * time.Minute
	}
	if o.Interval < time.Minute || o.Interval > time.Hour {
		return nil, errors.New("telemetry interval must be between 60 and 3600 seconds")
	}
	s := &Session{
		id: newID(), version: o.Version, start: time.Now(),
		stop: make(chan struct{}), done: make(chan struct{}),
		readySignal: make(chan struct{}),
		config:      map[string]string{}, stage: "configuration", diag: diagnostic,
		stateDirectory: o.StateDirectory, mountPath: o.MountPath, mountScope: o.MountScope,
		child: o.Child,
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
	if !o.StartedAt.IsZero() {
		s.start = o.StartedAt
	}
	owner, err := identity(os.Getpid())
	if err != nil {
		s.warn("telemetry: process identity unavailable; recovery inactive")
	}
	s.record = sessionRecord{Session: s.id, Owner: owner, Updated: time.Now()}
	if store, err := openMountStore(o.StateDirectory, o.MountPath, o.MountScope, true, s.warn); err != nil {
		s.warn("telemetry: protected state unavailable; reporting is memory-only")
	} else {
		s.store = store
		if o.Child {
			var previous sessionRecord
			if err := store.locked(func() error { return store.read("session-"+s.id+".json", &previous) }); err == nil {
				s.baseline = previous.Baseline
				s.record.Baseline = previous.Baseline
			} else {
				s.warn("telemetry: startup counters unavailable after daemon handoff")
			}
		}
	}
	s.reporter = newReporter(d, s.store, s.warn, o.HTTPClient)
	if s.store != nil {
		pending, err := s.store.recover(owner, s.id, s.event)
		if err != nil {
			s.warn("telemetry: startup recovery incomplete")
		}
		for _, e := range pending {
			s.reporter.reportRetained(e)
		}
		s.checkpoint()
	}
	if !o.Child {
		s.reporter.report(s.event("attempted"), true)
	}
	active.Store(s)
	go s.poll(o.Interval)
	return s, nil
}

// RetryState retries state initialization after mount-path repair, not delivery.
// It does not create another session/reporter or undo opt-out.
func (s *Session) RetryState() {
	if s == nil {
		return
	}
	if s.disabled.Load() {
		s.Disable()
		return
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed || s.disabled.Load() || s.detached.Load() || s.finishing.Load() || s.store != nil {
		return
	}
	store, err := openMountStore(s.stateDirectory, s.mountPath, s.mountScope, true, s.warn)
	if err != nil {
		s.warn("telemetry: state still unavailable after mount-path validation")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = store
	if s.child {
		var previous sessionRecord
		if err := store.locked(func() error { return store.read("session-"+s.id+".json", &previous) }); err == nil {
			s.baseline = previous.Baseline
			s.record.Baseline = previous.Baseline
		} else {
			s.warn("telemetry: startup counters unavailable after delayed state attachment")
		}
	}
	pending, err := store.recover(s.record.Owner, s.id, s.event)
	if err != nil {
		s.warn("telemetry: recovery incomplete after mount-path validation")
	}
	s.checkpoint()
	s.reporter.attachStore(store)
	for _, e := range pending {
		s.reporter.reportRetained(e)
	}
}

func (s *Session) warn(message string) {
	now := time.Now().Unix()
	last := s.lastDiag.Load()
	if (last == 0 || now-last >= 60) && s.lastDiag.CompareAndSwap(last, now) {
		s.diag(message)
	}
}

func (s *Session) Identity() (string, time.Time) {
	return s.id, s.start
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

func (s *Session) NativeHooks(enable, disable func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if Current() != s {
		disable()
		return
	}
	s.onDisable = disable
	enable()
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
	for k, v := range properties {
		if approvedConfig[k] && len(v) <= 256 {
			s.config[k] = v
		}
	}
}

var approvedConfig = map[string]bool{
	"pipeline_components": true, "stream_mode": true, "auth_mode": true,
	"fuse_api_major": true, "libfuse_runtime_version": true, "kernel_fuse_protocol": true,
	"distro_name": true, "distro_version": true, "kernel_release": true, "allow_other": true,
	"attr_cache_ttl_seconds": true, "kernel_attr_timeout_seconds": true,
}

func Stage(stage string) {
	if s := Current(); s != nil {
		s.mu.Lock()
		s.stage = stage
		s.failure = ""
		s.mu.Unlock()
	}
}

func Failure(category string) {
	if s := Current(); s != nil && !s.running.Load() {
		s.mu.Lock()
		s.failure = category
		s.mu.Unlock()
	}
}

func Ready(properties map[string]string, native func() map[string]float64) {
	s := Current()
	if s == nil {
		return
	}
	s.Configure(properties)
	s.mu.Lock()
	defer s.mu.Unlock()
	if Current() != s || !s.ready.IsZero() {
		return
	}
	s.ready, s.native = time.Now(), native
	s.running.Store(true)
	close(s.readySignal)
	e := s.event("result")
	for k, v := range s.config {
		e.Properties[k] = v
	}
	e.Properties["config_fingerprint"] = fingerprint(s.config)
	e.Properties["outcome"] = "success"
	if elapsed, ok := s.startupMilliseconds(); ok {
		e.Measurements["time_to_mount_ms"] = elapsed
	} else {
		s.warn("telemetry: startup duration unavailable")
	}
	s.reporter.report(e, true)
	s.checkpoint()
}

func (s *Session) checkpoint() {
	if s.store == nil {
		return
	}
	s.record.Updated, s.record.Ready = time.Now(), !s.ready.IsZero()
	if s.record.Ready {
		s.record.Uptime = time.Since(s.ready).Seconds()
	}
	if err := s.store.checkpoint(s.record); err != nil {
		s.warn("telemetry: session checkpoint failed")
	}
}

func (s *Session) phase() int {
	if s.running.Load() {
		return 1
	}
	return 0
}

// HTTPAttempt counts transport calls, not physical-wire transmissions.
func HTTPAttempt(hasResponse bool) {
	if s := Current(); s != nil {
		phase := s.phase()
		s.attempts[phase].Add(1)
		if !hasResponse {
			s.noResponse[phase].Add(1)
		}
	}
}

// HTTPResponse counts only responses observed at the instrumented boundary.
func HTTPResponse(status int) {
	if s := Current(); s != nil && status >= 100 && status < 600 {
		s.http[s.phase()][status].Add(1)
	}
}

func SDKRequest(attempts int, failed bool) {
	if s := Current(); s != nil {
		phase := s.phase()
		s.requests[phase].Add(1)
		if failed {
			s.failed[phase].Add(1)
		}
		if attempts > 1 {
			s.retried[phase].Add(1)
			s.retries[phase].Add(uint64(attempts - 1))
		}
	}
}

// Network uses a closed category set: DNS, TLS, timeout, reset, cancelled, other.
func Network(category int, body bool) {
	if s := Current(); s != nil {
		if category < 0 || category > 5 {
			category = 5
		}
		if body {
			s.network[s.phase()][6].Add(1)
		}
		s.network[s.phase()][category].Add(1)
	}
}

func (s *Session) measurements() map[string]float64 {
	m := map[string]float64{}
	if !s.ready.IsZero() {
		m["uptime_seconds"] = time.Since(s.ready).Seconds()
	}
	for phase, label := range []string{"startup", "runtime"} {
		total := uint64(0)
		for code := 100; code < 600; code++ {
			count := s.http[phase][code].Load()
			total += count
			if count > 0 {
				m[label+"_http_response_"+strconv.Itoa(code)] = float64(count)
			}
		}
		m[label+"_http_responses_total"] = float64(total)
		m[label+"_http_transport_attempts"] = float64(s.attempts[phase].Load())
		m[label+"_http_no_response"] = float64(s.noResponse[phase].Load())
		m[label+"_sdk_requests"] = float64(s.requests[phase].Load())
		m[label+"_sdk_terminal_failures"] = float64(s.failed[phase].Load())
		m[label+"_sdk_retry_attempts"] = float64(s.retries[phase].Load())
		m[label+"_sdk_retried_requests"] = float64(s.retried[phase].Load())
		for i, name := range []string{"dns", "tls", "timeout", "reset", "cancelled", "other", "body_read", "unused"} {
			if count := s.network[phase][i].Load(); count > 0 {
				m[label+"_transport_"+name] = float64(count)
			}
		}
	}
	if s.native != nil {
		for k, v := range s.native() {
			m[k] = v
		}
	}
	m["telemetry_dropped_records"] = float64(s.reporter.dropped.Load())
	m["telemetry_delivery_failures"] = float64(s.reporter.failures.Load())
	for key, value := range s.baseline {
		m[key] += value
	}
	return m
}

func (s *Session) snapshot() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ready.IsZero() {
		return
	}
	s.sequence++
	e := s.event("snapshot")
	e.Properties["sequence_number"] = strconv.FormatUint(s.sequence, 10)
	e.Properties["counter_scope"] = "session_cumulative"
	e.Properties["fuse_counters_available"] = strconv.FormatBool(s.native != nil)
	e.Properties["config_fingerprint"] = fingerprint(s.config)
	e.Measurements = s.measurements()
	parts, err := splitSnapshot(e, s.reporter.dest.key)
	if err != nil {
		s.warn("telemetry: snapshot cannot fit event limits")
		return
	}
	for _, part := range parts {
		s.reporter.report(part, false)
	}
	s.checkpoint()
}

func (s *Session) poll(interval time.Duration) {
	defer close(s.done)
	select {
	case <-s.stop:
		return
	case <-s.readySignal:
	}
	for {
		jitter := time.Duration(rand.Int64N(int64(interval/5)+1)) - interval/10
		timer := time.NewTimer(interval + jitter)
		select {
		case <-s.stop:
			timer.Stop()
			return
		case <-timer.C:
			s.snapshot()
		}
	}
}

// Detach stops parent collection but retains its reporter until launch succeeds
// or fails. No replacement session may bypass the parent's opt-out state.
func (s *Session) Detach() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed || s.disabled.Load() || s.detached.Load() {
		return
	}
	s.stopCollection()
	s.mu.Lock()
	s.record.Baseline = s.measurements()
	s.checkpoint()
	s.mu.Unlock()
	s.detached.Store(true)
}

func (s *Session) stopCollection() {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
}

// HandoffFailed reports a definitive launch failure before a worker exists.
func (s *Session) HandoffFailed(err error) {
	if s == nil {
		return
	}
	s.finish(err, true)
}

func (s *Session) Close(mountErr error) { s.finish(mountErr, false) }

// TeardownComplete must be called only after every pipeline component stopped.
func TeardownComplete() {
	if s := Current(); s != nil {
		s.mu.Lock()
		if !s.ready.IsZero() {
			s.teardownComplete = true
		}
		s.mu.Unlock()
	}
}

// Disable cancels delivery before removing retained state; it emits no event.
func (s *Session) Disable() {
	if s == nil {
		return
	}
	// Publish opt-out and cancel network work before waiting on lifecycle
	// serialization, which may currently be flushing a failed launch.
	s.disabled.Store(true)
	active.CompareAndSwap(s, nil)
	s.reporter.cancel()
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.stopCollection()
	s.mu.Lock()
	if s.onDisable != nil {
		s.onDisable()
	}
	s.mu.Unlock()
	s.reporter.close(0)
	if s.closed {
		// A concurrent finish may have closed the store just before it saw
		// opt-out, or initial path resolution may have failed. Retry only
		// this mount's purge without creating state or another reporter.
		s.retryPurge()
		return
	}
	if s.store != nil {
		if err := s.store.purge(); err != nil {
			s.warn("telemetry: opt-out could not delete retained state")
		}
		s.store.root.Close()
	} else {
		s.retryPurge()
	}
	s.closed = true
}

func (s *Session) retryPurge() {
	store, err := openMountStore(s.stateDirectory, s.mountPath, s.mountScope, false, s.warn)
	if err != nil {
		s.warn("telemetry: opt-out could not reopen retained state")
	} else if store != nil {
		if err := store.purge(); err != nil {
			s.warn("telemetry: opt-out could not delete retained state")
		}
		store.root.Close()
	}
}

func (s *Session) finish(mountErr error, handoffFailure bool) {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed {
		return
	}
	s.finishing.Store(true)
	s.stopCollection()
	if !s.disabled.Load() && (!s.detached.Load() || handoffFailure) {
		s.snapshot()
		s.mu.Lock()
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
				s.warn("telemetry: startup duration unavailable")
			}
		} else {
			e.Properties["termination_class"] = "unclean_unknown"
			if mountErr != nil {
				e.Properties["termination_class"] = "runtime_error"
			} else if s.teardownComplete {
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
		e.Properties["config_fingerprint"] = fingerprint(s.config)
		s.record.Terminal = &e
		s.checkpoint()
		s.reporter.report(e, true)
		s.mu.Unlock()
	}
	s.mu.Lock()
	if s.onDisable != nil {
		s.onDisable()
	}
	s.mu.Unlock()
	budget := FlushTimeout
	if s.disabled.Load() || s.detached.Load() && !handoffFailure {
		budget = 0
	}
	s.reporter.close(budget)
	if s.store != nil {
		if s.disabled.Load() {
			if err := s.store.purge(); err != nil {
				s.warn("telemetry: opt-out could not delete retained state")
			}
		}
		s.store.root.Close()
	}
	s.closed = true
	active.CompareAndSwap(s, nil)
}
