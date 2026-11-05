// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// This explicit, read-only smoke workflow is never run by go test.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	maxRequests = 10
	maxItem     = 8 * 1024
	maxBytes    = maxRequests * maxItem
)

type receipt struct {
	EventID      string             `json:"event_id"`
	Event        string             `json:"event"`
	Session      string             `json:"session"`
	Bytes        int                `json:"bytes"`
	Status       int                `json:"http_status"`
	Accepted     int                `json:"items_accepted"`
	Measurements map[string]float64 `json:"measurements,omitempty"`
}

type verifier struct {
	mu       sync.Mutex
	key      string
	endpoint string
	path     string
	client   *http.Client
	receipts []receipt
	requests int
	bytes    int
}

func (v *verifier) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != v.path {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxItem))
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	var batch []struct {
		Key  string `json:"iKey"`
		Data struct {
			Base struct {
				Name         string             `json:"name"`
				Properties   map[string]string  `json:"properties"`
				Measurements map[string]float64 `json:"measurements"`
			} `json:"baseData"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &batch) != nil || len(batch) != 1 || batch[0].Key != v.key ||
		!strings.HasPrefix(batch[0].Data.Base.Name, "blobfuse.mount.") {
		http.Error(w, "unexpected test event", http.StatusBadRequest)
		return
	}
	v.mu.Lock()
	if v.requests >= maxRequests || v.bytes+len(body) > maxBytes {
		v.mu.Unlock()
		w.Header().Set("Retry-After", "300")
		http.Error(w, "smoke-test budget reached", http.StatusTooManyRequests)
		return
	}
	v.requests++
	v.bytes += len(body)
	v.mu.Unlock()
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, v.endpoint, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "invalid upstream request", http.StatusBadGateway)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := v.client.Do(request)
	item := batch[0].Data.Base
	result := receipt{EventID: item.Properties["event_id"], Event: item.Name,
		Session: item.Properties["mount_session_id"], Bytes: len(body), Measurements: item.Measurements}
	var reply []byte
	if err == nil {
		defer response.Body.Close()
		result.Status = response.StatusCode
		reply, err = io.ReadAll(io.LimitReader(response.Body, 64*1024))
		var acknowledgement struct {
			Accepted int `json:"itemsAccepted"`
		}
		if json.Unmarshal(reply, &acknowledgement) == nil {
			result.Accepted = acknowledgement.Accepted
		}
	}
	v.mu.Lock()
	v.receipts = append(v.receipts, result)
	v.mu.Unlock()
	if err != nil {
		http.Error(w, "upstream delivery failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if value := response.Header.Get("Retry-After"); value != "" {
		w.Header().Set("Retry-After", value)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(reply)
}

type report struct {
	RunDirectory string    `json:"run_directory"`
	Session      string    `json:"session_id"`
	ReadOnly     bool      `json:"read_only"`
	Unmounted    bool      `json:"unmounted"`
	Requests     int       `json:"forwarded_requests"`
	Bytes        int       `json:"forwarded_bytes"`
	Receipts     []receipt `json:"receipts"`
	Failure      string    `json:"failure,omitempty"`
}

func connection(path string) (string, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", errors.New("cannot read the telemetry test environment file")
	}
	const prefix = "export BLOBFUSE_TELEMETRY_CONNECTION_STRING="
	value := ""
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, prefix) {
			value = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), "'\"")
		}
	}
	fields := map[string]string{}
	for _, field := range strings.Split(value, ";") {
		if key, val, ok := strings.Cut(field, "="); ok {
			fields[key] = val
		}
	}
	u, err := url.Parse(fields["IngestionEndpoint"])
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || fields["InstrumentationKey"] == "" {
		return "", "", errors.New("test destination requires an explicit HTTPS endpoint and instrumentation key")
	}
	return fields["InstrumentationKey"], strings.TrimRight(u.String(), "/") + "/v2.1/track", nil
}

func privateFile(path string, body []byte) error {
	return os.WriteFile(path, body, 0600)
}

func isMounted(path string) (bool, error) {
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, errors.New("cannot inspect Linux mount state")
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 4 && fields[4] == path {
			return true, nil
		}
	}
	return false, nil
}

func unmount(binary, path string, lazy bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := []string{"unmount", path}
	if lazy {
		args = append(args, "--lazy")
	}
	return exec.CommandContext(ctx, binary, args...).Run()
}

func run(ctx context.Context, binary, configPath, envPath, output string) (err error) {
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	key, endpoint, err := connection(envPath)
	if err != nil {
		return err
	}
	input, err := os.ReadFile(configPath)
	if err != nil {
		return errors.New("cannot read storage configuration")
	}
	var config map[string]interface{}
	if yaml.Unmarshal(input, &config) != nil {
		return errors.New("cannot parse storage configuration")
	}
	storage, ok := config["azstorage"].(map[interface{}]interface{})
	if !ok || storage["account-name"] == nil || storage["container"] == nil {
		return errors.New("storage configuration needs azstorage account-name and container")
	}
	if output == "" {
		output, err = os.UserCacheDir()
		if err != nil {
			return err
		}
		output = filepath.Join(output, "blobfuse2-telemetry-smoke")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(output, "run-")
	if err != nil {
		return err
	}
	result := report{RunDirectory: directory, ReadOnly: true}
	fmt.Println("Private test artifacts:", directory)
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	prefix := "/" + hex.EncodeToString(token[:])
	v := &verifier{key: key, endpoint: endpoint, path: prefix + "/v2.1/track",
		client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}}
	server := httptest.NewTLSServer(v)
	defer server.Close()
	mountPath := filepath.Join(directory, "mount")
	configCopy := filepath.Join(directory, "mount-config.yaml")
	work := filepath.Join(directory, "work")
	cache := filepath.Join(directory, "cache")
	for _, path := range []string{mountPath, work, cache} {
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
	}
	// Scope trust to this child only; preserve normal Azure certificate roots.
	roots, err := os.ReadFile("/etc/ssl/certs/ca-certificates.crt")
	if err != nil {
		return errors.New("cannot read the VM certificate bundle")
	}
	roots = append(roots, '\n')
	roots = append(roots, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})...)
	caPath := filepath.Join(directory, "test-ca.pem")
	if err := privateFile(caPath, roots); err != nil {
		return err
	}
	config["components"] = []string{"libfuse", "file_cache", "attr_cache", "azstorage"}
	config["foreground"], config["read-only"], config["allow-other"], config["allow-root"] = true, true, false, false
	config["preload"], config["block-cache"], config["streaming"] = false, false, false
	delete(config, "libfuse-options")
	config["default-working-dir"] = work
	config["dynamic-profile"], config["lazy-write"] = false, false
	config["health_monitor"] = map[string]interface{}{"enable-monitoring": false}
	config["file_cache"] = map[string]interface{}{"path": cache, "max-size-mb": 16, "timeout-sec": 1}
	config["logging"] = map[string]interface{}{"type": "base", "level": "LOG_WARNING",
		"file-path": filepath.Join(directory, "blobfuse.log"), "max-file-size-mb": 1, "file-count": 1}
	config["microsoft-telemetry"] = map[string]interface{}{"disabled": false, "snapshot-interval-sec": 60,
		"state-directory": filepath.Join(work, "microsoft-telemetry")}
	delete(config, "distributed_cache")
	delete(config, "mount-all-containers")
	payload, err := yaml.Marshal(config)
	if err != nil {
		return errors.New("cannot encode private test configuration")
	}
	if err := privateFile(configCopy, payload); err != nil {
		return err
	}
	defer func() {
		if removeErr := os.Remove(configCopy); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, errors.New("cannot remove private credential-bearing config copy"))
		}
	}()
	logFile, err := os.OpenFile(filepath.Join(directory, "worker.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command(binary, "mount", mountPath, "--config-file="+configCopy, "--foreground=true",
		"--read-only", "--disable-version-check")
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != "BLOBFUSE_TELEMETRY_CONNECTION_STRING" && name != "BLOBFUSE_DISABLE_TELEMETRY" && name != "SSL_CERT_FILE" {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "BLOBFUSE_TELEMETRY_CONNECTION_STRING=InstrumentationKey="+key+
		";IngestionEndpoint="+server.URL+prefix, "BLOBFUSE_DISABLE_TELEMETRY=false", "SSL_CERT_FILE="+caPath)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		return errors.New("cannot start BlobFuse; check the binary path")
	}
	done := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(done) }()
	defer func() {
		mounted, stateErr := isMounted(mountPath)
		if stateErr != nil {
			err = errors.Join(err, stateErr)
		}
		if mounted || stateErr != nil {
			if cleanupErr := unmount(binary, mountPath, false); cleanupErr != nil {
				err = errors.Join(err, errors.New("normal cleanup unmount failed"))
				if unmount(binary, mountPath, true) != nil {
					err = errors.Join(err, errors.New("lazy cleanup unmount failed"))
				}
			}
		}
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
			err = errors.Join(err, errors.New("worker required forced cleanup"))
		}
		mounted, stateErr = isMounted(mountPath)
		if stateErr != nil {
			err = errors.Join(err, stateErr)
		}
		result.Unmounted = stateErr == nil && !mounted
		v.mu.Lock()
		result.Requests, result.Bytes = v.requests, v.bytes
		result.Receipts = append([]receipt(nil), v.receipts...)
		v.mu.Unlock()
		accepted := map[string]bool{}
		for _, item := range result.Receipts {
			if item.Status >= 200 && item.Status < 300 && item.Accepted == 1 {
				accepted[item.Event] = true
				result.Session = item.Session
			}
		}
		for _, kind := range []string{"attempted", "result", "snapshot", "stopped"} {
			if !accepted["blobfuse.mount."+kind] {
				err = errors.Join(err, fmt.Errorf("Application Insights did not acknowledge a %s event", kind))
			}
		}
		if !result.Unmounted {
			err = errors.Join(err, errors.New("test mount remains mounted"))
		}
		if err != nil {
			result.Failure = err.Error()
		}
		b, encodeErr := json.MarshalIndent(result, "", "  ")
		if encodeErr != nil {
			err = errors.Join(err, errors.New("cannot encode verification report"))
		} else {
			if writeErr := privateFile(filepath.Join(directory, "report.json"), b); writeErr != nil {
				err = errors.Join(err, errors.New("cannot save verification report"))
			}
		}
		fmt.Printf("Forwarded: %d requests, %d bytes; unmounted: %t\n", result.Requests, result.Bytes, result.Unmounted)
		fmt.Println("Verification report:", filepath.Join(directory, "report.json"))
	}()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		mounted, stateErr := isMounted(mountPath)
		if stateErr != nil {
			return stateErr
		}
		if mounted {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return fmt.Errorf("BlobFuse exited before mounting (%v); inspect private worker.log", waitErr)
		case <-deadline.C:
			return errors.New("mount readiness timeout")
		case <-time.After(100 * time.Millisecond):
		}
	}
	fmt.Println("Read-only mount ready; checking one missing entry without downloading file contents.")
	lookup := make(chan error, 1)
	go func() {
		_, statErr := os.Stat(filepath.Join(mountPath, ".telemetry-smoke-missing-"+hex.EncodeToString(token[:])))
		lookup <- statErr
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case statErr := <-lookup:
		if !errors.Is(statErr, os.ErrNotExist) {
			return errors.New("missing-entry lookup did not return ENOENT")
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return fmt.Errorf("BlobFuse stopped during the test (%v)", waitErr)
	case <-time.After(70 * time.Second):
	}
	if err := unmount(binary, mountPath, false); err != nil {
		return errors.New("test unmount failed; cleanup will retry")
	}
	select {
	case <-done:
		if waitErr != nil {
			return errors.New("BlobFuse returned a nonzero exit after unmount")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(15 * time.Second):
		return errors.New("BlobFuse did not finish teardown within 15 seconds")
	}
}

func main() {
	binary := flag.String("binary", "./blobfuse2", "Path to the telemetry-enabled binary")
	config := flag.String("config", "./config.yaml", "Existing storage configuration; never modified")
	env := flag.String("env-file", "./test/scripts/telemetry-test.env", "Explicit test destination file")
	output := flag.String("output", "", "Parent directory for private test artifacts")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := run(ctx, *binary, *config, *env, *output); err != nil {
		fmt.Fprintln(os.Stderr, "Telemetry smoke test failed:", err)
		os.Exit(1)
	}
	fmt.Println("Telemetry smoke test passed; actual BlobFuse events were accepted by Application Insights.")
}
