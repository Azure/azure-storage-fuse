---
description: "Use when writing or changing Go tests: testify suites, build tags, test config, gomock MockComponent, loopbackfs, httptest fakes for Azure Storage, flaky or slow tests."
applyTo: "**/*_test.go"
---
# Go tests

- Unit tests run with `-tags=unittest,fuse3` (or `unittest,fuse2`); use `make test PKG=<dir> TEST_FLAGS='-run <Regex> -v'`. Files under `test/` that need a mounted container have `//go:build !unittest`, and the live Azure suites in `component/azstorage` read `~/azuretest.json`. New unit tests must pass with neither. libfuse tests that mount FUSE in-process have `//go:build integration && !fuse2` and run with `make test-integration`.
- Follow the existing testify suite shape: a `suite.Suite` struct with an `assert *assert.Assertions` field set in `SetupTest` with `assert.New(suite.T())`, and `func TestXxx(t *testing.T) { suite.Run(t, new(xxxTestSuite)) }`. Set the logger to `silent` in `SetupTest`. testify calls only correctly spelled hooks: `TearDownTest` runs, `TeardownTest` never does.
- `testifylint` runs with every checker except `require-error` and `suite-thelper`, so use the specific assertion: `NoError`, `ErrorIs`, `Len`, `Empty`, `True`, `InDelta`, and expected-before-actual in `Equal`.
- Configuration is process-global. Load each test's config with `config.ReadConfigFromReader(strings.NewReader(cfg))` and don't call `t.Parallel()` in suites that read config or use libfuse's `fuseFS`.
- Create files and directories under `t.TempDir()` (or `suite.T().TempDir()`), not under `$HOME` or fixed `/tmp` paths, so runs can't collide. Control environment variables with `t.Setenv`; a test must not depend on what is installed on the machine, such as `bfusemon` on `PATH`.
- Choose the next component by what the test checks: `internal.NewMockComponent(gomock.NewController(suite.T()))` with `EXPECT()` calls to check the exact calls a component makes, a struct that embeds `internal.BaseComponent` and overrides only the calls the test needs when the calls don't matter (`attrCacheNoopNext` in `attr_cache_test.go`), `loopbackfs` to check real file behavior, and an `httptest.Server` to stand in for Azure Storage in `azstorage` tests.
- Wait for asynchronous work with channels or `assert.Eventually` rather than a fixed `time.Sleep`, which makes suites slow and flaky on loaded machines. Test time-based logic such as expiry by creating entries with timestamps in the past and calling the worker function directly, as `sweeperTestSuite` in `attr_cache_test.go` does. Run a concurrency test with `-race -count=20` before you rely on it.
- A new `_test.go` file needs the two-line license header like any Go file (`// Copyright (c) Microsoft Corporation.` and `// Licensed under the MIT License.`), below the `//go:build` line if there is one.
