import argparse
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


def load_module(name: str, relative_path: str):
    spec = importlib.util.spec_from_file_location(name, REPO_ROOT / relative_path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


runner = load_module("run_benchmark", "perf_testing/scripts/run_benchmark.py")
comparator = load_module("compare_benchmarks", "perf_testing/scripts/compare_benchmarks.py")
publisher = load_module("publish_benchmarks", "perf_testing/scripts/publish_benchmarks.py")
regression_checker = load_module("check_regressions", "perf_testing/scripts/check_regressions.py")


def make_summary(
    suite="regression",
    cache_mode="block_cache",
    value=1000.0,
    vm_size="Standard_D96ds_v5",
):
    return {
        "schema_version": 1,
        "generated_at": "2026-07-30T00:00:00Z",
        "run": {
            "id": "run-1",
            "label": "main",
            "suite": suite,
            "commit": "a" * 40,
            "ref": "refs/heads/main",
            "architecture": "X86",
            "account_type": "premium",
            "cache_mode": cache_mode,
            "compute_profile": vm_size,
        },
        "environment": {
            "architecture": "X86",
            "account_type": "premium",
            "cache_mode": cache_mode,
            "runner": "private-runner-name",
            "cpu_count": 96,
            "memory_gib": 384,
            "vm_size": vm_size,
            "azure_region": "eastus2",
            "compute_profile": vm_size,
            "config_sha256": "config-a",
        },
        "benchmarks": [
            {
                "id": "read",
                "name": "Read",
                "direction": "read",
                "primary_metric": "throughput_mib_s",
                "higher_is_better": True,
                "parameters": {"block_size": "4 MiB"},
                "metrics": {
                    "throughput_mib_s": {
                        "median": value,
                        "mad": 10.0,
                        "min": value - 20,
                        "max": value + 20,
                    },
                    "network_rx_mib": {
                        "median": 100.0,
                        "mad": 0.0,
                        "min": 100.0,
                        "max": 100.0,
                    },
                },
                "samples": [{"trial": 1, "metrics": {"raw": 1}}],
            }
        ],
    }


class PublicBenchmarkContractTests(unittest.TestCase):
    def test_public_suite_has_symmetric_single_and_multi_file_cases(self):
        manifest = runner.load_manifest()
        public = manifest["suites"]["public"]
        workloads = {workload["id"]: workload for workload in public["workloads"]}

        self.assertEqual(manifest["fixture_version"], "v3")
        self.assertEqual(
            set(workloads),
            {
                "sequential-read-single-file",
                "sequential-read-multi-file",
                "sequential-write-single-file",
                "sequential-write-multi-file",
            },
        )
        for workload in workloads.values():
            self.assertEqual(workload["parameters"]["block_size"], "10 MiB")
            self.assertEqual(workload["parameters"]["working_set"], "320 GiB")
            self.assertEqual(workload["primary_metric"], "throughput_mib_s")

        for operation in ("read", "write"):
            single = workloads[f"sequential-{operation}-single-file"]["parameters"]
            multi = workloads[f"sequential-{operation}-multi-file"]["parameters"]
            self.assertEqual((single["concurrency"], single["files"], single["file_size"]), (1, 1, "320 GiB"))
            self.assertEqual((multi["concurrency"], multi["files"], multi["file_size"]), (16, 16, "20 GiB"))

    def test_public_jobs_use_fio_direct_without_mount_wide_direct_io(self):
        suite, _ = runner.load_suite("public")
        job_files = [workload["job_file"] for workload in suite["workloads"]]
        job_files.extend(suite["fixture_jobs"])
        for relative_path in job_files:
            job = runner.BENCHMARK_CONFIG_DIR / relative_path
            options = {
                line.strip()
                for line in job.read_text(encoding="utf-8").splitlines()
                if line and not line.startswith("[")
            }
            self.assertIn("direct=1", options)
            self.assertIn("bs=10M", options)
            self.assertIn("group_reporting=1", options)
            if "single" in relative_path:
                self.assertIn("size=320G", options)
                self.assertNotIn("numjobs=16", options)
            else:
                self.assertIn("size=20G", options)
                self.assertIn("numjobs=16", options)

        for config_name in ("azure_block_benchmark.yaml", "azure_file_benchmark.yaml"):
            config = (REPO_ROOT / "testdata/config" / config_name).read_text(encoding="utf-8")
            self.assertNotIn("direct-io:", config)


class WorkflowShapeTests(unittest.TestCase):
    def test_scheduled_workflow_acquires_one_runner_per_architecture(self):
        workflow = (REPO_ROOT / ".github/workflows/benchmark.yml").read_text(encoding="utf-8")
        profiles = (
            "standard block-cache",
            "premium block-cache",
            "standard file-cache",
            "premium file-cache",
            "standard HNS block-cache",
            "premium HNS block-cache",
            "standard HNS file-cache",
            "premium HNS file-cache",
        )

        self.assertEqual(workflow.count("runner: 1ES.Pool=blobfuse2-benchmark\n"), 1)
        self.assertEqual(workflow.count("runner: 1ES.Pool=blobfuse2-benchmark-arm\n"), 1)
        self.assertEqual(workflow.count("uses: ./.github/actions/perftesting"), 9)
        self.assertEqual(workflow.count('SETUP_HOST: "true"'), 1)
        self.assertEqual(workflow.count('SETUP_HOST: "false"'), 8)
        self.assertEqual(workflow.count("continue-on-error: true"), len(profiles))
        for profile in profiles:
            self.assertEqual(workflow.count(f"Run {profile} profile"), 1)
        self.assertIn("always() && steps.host_setup.outcome == 'success'", workflow)
        self.assertIn('"$required" == "true" && "$outcome" != "success"', workflow)
        self.assertIn("name: perf-${{ matrix.arch }}", workflow)
        self.assertNotIn("matrix.account_type", workflow)
        self.assertNotIn("matrix.cache_mode", workflow)

    def test_profile_results_are_isolated_below_architecture_artifacts(self):
        action = (REPO_ROOT / ".github/actions/perftesting/action.yml").read_text(encoding="utf-8")
        result_path = (
            "benchmark-results/${{ inputs.ARCH }}/"
            "${{ inputs.ACCOUNT_TYPE }}/${{ inputs.CACHE_MODE }}"
        )

        self.assertIn(result_path, action)
        self.assertIn("RUN_PROFILE:", action)
        self.assertIn("inputs.RUN_PROFILE == 'true'", action)
        self.assertNotIn("/dev/nvme", action)
        self.assertNotIn("mdadm", action)


class FioResultTests(unittest.TestCase):
    def test_azure_instance_metadata_extracts_vm_size_and_region(self):
        class Response:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return False

            def read(self):
                return b'{"vmSize":"Standard_D192ds_v5","location":"eastus2"}'

        class Opener:
            def open(self, request, timeout):
                self.request = request
                self.timeout = timeout
                return Response()

        opener = Opener()
        metadata = runner.azure_instance_metadata(opener)
        self.assertEqual(metadata, {"vm_size": "Standard_D192ds_v5", "azure_region": "eastus2"})
        self.assertEqual(opener.timeout, 2)
        self.assertEqual(opener.request.get_header("Metadata"), "true")

    def test_parser_uses_wall_time_and_keeps_sync_latency_separate(self):
        fio_result = {
            "jobs": [
                {
                    "jobname": "write",
                    "error": 0,
                    "write": {
                        "total_ios": 8,
                        "io_bytes": 8 * 1024**2,
                        "bw": 4096,
                        "iops": 4,
                        "lat_ns": {
                            "mean": 2_000_000,
                            "N": 8,
                            "percentile": {"99.000000": 5_000_000},
                        },
                        "clat_ns": {"percentile": {"99.000000": 4_000_000}},
                    },
                    "sync": {
                        "lat_ns": {"percentile": {"99.000000": 20_000_000}}
                    },
                }
            ]
        }
        with tempfile.TemporaryDirectory() as temporary:
            result_path = Path(temporary) / "fio.json"
            result_path.write_text(json.dumps(fio_result))
            metrics = runner.parse_fio_result(result_path, "write", wall_seconds=4.0)

        self.assertEqual(metrics["throughput_mib_s"], 2.0)
        self.assertEqual(metrics["fio_throughput_mib_s"], 4.0)
        self.assertEqual(metrics["latency_p99_ms"], 5.0)
        self.assertEqual(metrics["sync_latency_p99_ms"], 20.0)

    def test_summary_reports_median_and_mad(self):
        samples = [
            {"metrics": {"iops": 90.0}},
            {"metrics": {"iops": 100.0}},
            {"metrics": {"iops": 140.0}},
        ]
        summary = runner.summarize_samples(samples)["iops"]
        self.assertEqual(summary["median"], 100.0)
        self.assertEqual(summary["mad"], 10.0)
        self.assertEqual(summary["min"], 90.0)
        self.assertEqual(summary["max"], 140.0)

    def test_network_ratio_uses_the_workload_direction(self):
        self.assertEqual(runner.network_to_io_ratio("read", 1024.0, 10.0, 1.0), 1.0)
        self.assertEqual(runner.network_to_io_ratio("write", 10.0, 1024.0, 1.0), 1.0)
        self.assertEqual(runner.network_to_io_ratio("mixed", 512.0, 512.0, 1.0), 1.0)

    def test_only_ephemeral_non_keep_trial_data_is_removed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            ephemeral = root / "ephemeral"
            fixture = root / "fixture"
            kept = root / "kept"
            for path in (ephemeral, fixture, kept):
                path.mkdir()
                (path / "data").write_text("test")

            runner.remove_trial_data(ephemeral, "ephemeral", keep_data=False)
            runner.remove_trial_data(fixture, "fixture", keep_data=False)
            runner.remove_trial_data(kept, "ephemeral", keep_data=True)

            self.assertFalse(ephemeral.exists())
            self.assertTrue(fixture.exists())
            self.assertTrue(kept.exists())


class ComparisonTests(unittest.TestCase):
    def test_clear_regression_is_detected(self):
        baseline = make_summary(value=1000.0)
        candidate = make_summary(value=850.0)
        candidate["run"].update({"id": "candidate", "commit": "b" * 40})
        comparison = comparator.compare_summaries(baseline, candidate, 10.0, 20.0)
        self.assertEqual(comparison["overall_status"], "regression")
        self.assertEqual(comparison["benchmarks"][0]["status"], "regression")
        markdown = comparator.render_markdown(comparison)
        self.assertIn("xychart-beta", markdown)
        self.assertIn("bar [85.0]", markdown)

    def test_noise_expands_threshold(self):
        baseline_metric = {"median": 1000.0, "mad": 120.0}
        candidate_metric = {"median": 850.0, "mad": 100.0}
        comparison = comparator.compare_metric(
            baseline_metric,
            candidate_metric,
            higher_is_better=True,
            threshold=10.0,
        )
        self.assertEqual(comparison["status"], "noisy")
        self.assertGreater(comparison["effective_threshold_percent"], 30.0)

    def test_zero_baseline_produces_finite_json(self):
        comparison = comparator.compare_metric(
            {"median": 0.0, "mad": 0.0},
            {"median": 10.0, "mad": 0.0},
            higher_is_better=True,
            threshold=10.0,
        )
        self.assertEqual(comparison["delta_percent"], 100.0)
        json.dumps(comparison, allow_nan=False)


class ScheduledRegressionTests(unittest.TestCase):
    def test_initial_history_is_insufficient_but_does_not_fail(self):
        candidate = make_summary()
        report = regression_checker.evaluate_results(
            {"schema_version": 1, "runs": []},
            [candidate],
            window=5,
            minimum_baselines=3,
            primary_threshold=10.0,
            latency_threshold=20.0,
        )
        self.assertEqual(report["overall_status"], "pass")
        self.assertEqual(
            report["checks"][0]["benchmarks"][0]["status"],
            "insufficient-baseline",
        )

    def test_public_suite_is_included_in_scheduled_checks(self):
        candidate = make_summary(suite="public")
        report = regression_checker.evaluate_results(
            {"schema_version": 1, "runs": []},
            [candidate],
            window=5,
            minimum_baselines=3,
            primary_threshold=10.0,
            latency_threshold=20.0,
        )
        self.assertEqual(len(report["checks"]), 1)
        self.assertEqual(report["checks"][0]["run"]["suite"], "public")

    def test_d192_candidate_does_not_use_d96_history(self):
        candidate = make_summary(value=850.0, vm_size="Standard_D192ds_v5")
        history_runs = []
        for index in range(5):
            historical = make_summary(value=1000.0, vm_size="Standard_D96ds_v5")
            historical["key"] = str(index)
            historical["generated_at"] = f"2026-07-{20 + index:02d}T00:00:00Z"
            historical["benchmarks"] = [
                {key: value for key, value in benchmark.items() if key != "samples"}
                for benchmark in historical["benchmarks"]
            ]
            history_runs.append(historical)

        report = regression_checker.evaluate_results(
            {"schema_version": 1, "runs": history_runs},
            [candidate],
            window=5,
            minimum_baselines=3,
            primary_threshold=10.0,
            latency_threshold=20.0,
        )
        self.assertEqual(report["overall_status"], "pass")
        self.assertEqual(report["checks"][0]["benchmarks"][0]["status"], "insufficient-baseline")

    def test_drop_against_five_matching_runs_is_a_regression(self):
        candidate = make_summary(value=850.0)
        candidate["benchmarks"][0]["metrics"]["latency_p99_ms"] = {
            "median": 15.0,
            "mad": 0.5,
            "min": 14.0,
            "max": 16.0,
        }
        history_runs = []
        for index in range(5):
            historical = make_summary(value=1000.0)
            historical["key"] = str(index)
            historical["generated_at"] = f"2026-07-{20 + index:02d}T00:00:00Z"
            historical["benchmarks"] = [
                {key: value for key, value in benchmark.items() if key != "samples"}
                for benchmark in historical["benchmarks"]
            ]
            history_runs.append(historical)
        report = regression_checker.evaluate_results(
            {"schema_version": 1, "runs": history_runs},
            [candidate],
            window=5,
            minimum_baselines=3,
            primary_threshold=10.0,
            latency_threshold=20.0,
        )
        self.assertEqual(report["overall_status"], "regression")
        self.assertEqual(report["checks"][0]["benchmarks"][0]["status"], "regression")


class PublicationTests(unittest.TestCase):
    def test_public_history_is_curated_and_developer_history_is_complete(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            results = root / "results"
            site = root / "site"
            (site / "X86/legacy").mkdir(parents=True)
            (site / "legacy_runs").mkdir(parents=True)
            (site / "release/latest").mkdir(parents=True)
            (site / "release/latest/2.5.5").touch()
            (site / "_config.yaml").write_text("title: legacy\n")
            (results / "public").mkdir(parents=True)
            (results / "regression").mkdir(parents=True)
            (results / "public" / "summary.json").write_text(
                json.dumps(make_summary(suite="public", cache_mode="block_cache"))
            )
            regression = make_summary(suite="regression", cache_mode="file_cache")
            regression["run"]["id"] = "run-2"
            (results / "regression" / "summary.json").write_text(json.dumps(regression))
            args = argparse.Namespace(
                results_dir=results,
                site_dir=site,
                dashboard_dir=REPO_ROOT / "perf_testing" / "dashboard",
                public_retention_days=730,
                developer_retention_days=400,
            )
            public_count, developer_count = publisher.publish(args)
            public_history = json.loads((site / "data/public/history.json").read_text())
            developer_history = json.loads((site / "data/developer/history.json").read_text())
            release_metadata_preserved = (site / "release/latest/2.5.5").exists()
            legacy_performance_removed = not (site / "X86").exists() and not (site / "legacy_runs").exists()
            legacy_jekyll_removed = not (site / "_config.yaml").exists()

        self.assertEqual(public_count, 1)
        self.assertEqual(developer_count, 2)
        self.assertEqual(len(public_history["runs"]), 1)
        self.assertEqual(len(developer_history["runs"]), 2)
        public_benchmark = public_history["runs"][0]["benchmarks"][0]
        self.assertNotIn("samples", public_benchmark)
        self.assertNotIn("network_rx_mib", public_benchmark["metrics"])
        self.assertNotIn("runner", public_history["runs"][0]["environment"])
        self.assertEqual(public_history["runs"][0]["environment"]["vm_size"], "Standard_D96ds_v5")
        self.assertEqual(public_history["runs"][0]["environment"]["azure_region"], "eastus2")
        self.assertTrue(release_metadata_preserved)
        self.assertTrue(legacy_performance_removed)
        self.assertTrue(legacy_jekyll_removed)

    def test_developer_only_bootstrap_preserves_legacy_public_site(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            results = root / "results/regression"
            site = root / "site"
            results.mkdir(parents=True)
            (site / "X86/legacy").mkdir(parents=True)
            (site / "README.md").write_text("legacy public site\n")
            (site / "_config.yaml").write_text("title: legacy\n")
            (results / "summary.json").write_text(
                json.dumps(make_summary(suite="regression", cache_mode="block_cache"))
            )
            args = argparse.Namespace(
                results_dir=root / "results",
                site_dir=site,
                dashboard_dir=REPO_ROOT / "perf_testing" / "dashboard",
                public_retention_days=730,
                developer_retention_days=400,
            )
            public_count, developer_count = publisher.publish(args)

            self.assertEqual(public_count, 0)
            self.assertEqual(developer_count, 1)
            self.assertTrue((site / "X86").exists())
            self.assertTrue((site / "_config.yaml").exists())
            self.assertEqual((site / "README.md").read_text(), "legacy public site\n")
            self.assertFalse((site / "index.html").exists())
            self.assertTrue((site / "developer/index.html").exists())


if __name__ == "__main__":
    unittest.main()
