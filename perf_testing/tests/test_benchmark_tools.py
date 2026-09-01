import argparse
import importlib.util
import json
import os
import tempfile
import threading
import time
import unittest
from pathlib import Path
from unittest import mock


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
    trials=5,
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
            "trials": trials,
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
                    "end_to_end_throughput_mib_s": {
                        "median": value,
                        "mad": 10.0,
                        "min": value - 20,
                        "max": value + 20,
                    },
                    "end_to_end_seconds": {
                        "median": 10.0,
                        "mad": 0.5,
                        "min": 9.5,
                        "max": 10.5,
                    },
                    "fio_throughput_mib_s": {
                        "median": value * 1.1,
                        "mad": 10.0,
                        "min": value,
                        "max": value * 1.2,
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

        self.assertEqual(manifest["fixture_version"], "v4")
        self.assertTrue(
            all(suite["trials"] == 5 for suite in manifest["suites"].values())
        )
        self.assertEqual(
            set(workloads),
            {
                "sequential-read-single-file",
                "sequential-read-four-file",
                "sequential-write-single-file",
                "sequential-write-four-file",
            },
        )
        for workload in workloads.values():
            self.assertEqual(workload["parameters"]["block_size"], "10 MiB")
            self.assertEqual(workload["parameters"]["working_set"], "320 GiB")
            self.assertEqual(workload["primary_metric"], "throughput_mib_s")

            job = (
                runner.BENCHMARK_CONFIG_DIR / workload["job_file"]
            ).read_text(encoding="utf-8")
            if workload["parameters"]["durability"] == "end fsync":
                self.assertIn("end_fsync=1", job)

        for operation in ("read", "write"):
            single = workloads[f"sequential-{operation}-single-file"]["parameters"]
            multi = workloads[f"sequential-{operation}-four-file"]["parameters"]
            self.assertEqual((single["concurrency"], single["files"], single["file_size"]), (1, 1, "320 GiB"))
            self.assertEqual((multi["concurrency"], multi["files"], multi["file_size"]), (4, 4, "80 GiB"))

    def test_public_jobs_use_fio_direct_without_mount_wide_direct_io(self):
        suite, _ = runner.load_suite("public")
        job_files = [workload["job_file"] for workload in suite["workloads"]]
        job_files.extend(suite["fixture_jobs"])
        for relative_path in job_files:
            job = runner.BENCHMARK_CONFIG_DIR / relative_path
            lines = job.read_text(encoding="utf-8").splitlines()
            section_headers = [line.strip() for line in lines if line.strip().startswith("[")]
            self.assertTrue(section_headers, relative_path)
            self.assertTrue(
                all(
                    header.startswith("[")
                    and not header.startswith("[[")
                    and header.endswith("]")
                    and not header.endswith("]]")
                    for header in section_headers
                ),
                relative_path,
            )
            options = {
                line.strip()
                for line in lines
                if line and not line.startswith("[")
            }
            self.assertIn("direct=1", options)
            self.assertIn("bs=10M", options)
            self.assertIn("group_reporting=1", options)
            if "single" in relative_path:
                self.assertIn("size=320G", options)
                self.assertNotIn("numjobs=4", options)
            else:
                self.assertIn("size=80G", options)
                self.assertIn("numjobs=4", options)

        for config_name in ("azure_block_benchmark.yaml", "azure_file_benchmark.yaml"):
            config = (REPO_ROOT / "testdata/config" / config_name).read_text(encoding="utf-8")
            self.assertNotIn("direct-io:", config)

        block_config = (REPO_ROOT / "testdata/config/azure_block_benchmark.yaml").read_text(
            encoding="utf-8"
        )
        file_config = (REPO_ROOT / "testdata/config/azure_file_benchmark.yaml").read_text(
            encoding="utf-8"
        )
        self.assertIn("sync-to-flush: true", file_config)
        self.assertNotIn("max-fuse-threads:", block_config)
        self.assertNotIn("block_cache:", block_config)
        for setting in ("block-size-mb:", "mem-size-mb:", "prefetch:", "parallelism:"):
            self.assertNotIn(setting, block_config)
        self.assertIn("timeout-sec: 7200", block_config)

        dashboard = (REPO_ROOT / "perf_testing/dashboard/public/index.html").read_text(
            encoding="utf-8"
        )
        self.assertIn("end_to_end_throughput_mib_s", dashboard)
        self.assertIn("including file open and close", dashboard)
        self.assertIn("sequential-read-four-file", dashboard)
        self.assertIn("sequential-write-four-file", dashboard)
        self.assertNotIn("sequential-read-multi-file", dashboard)
        self.assertNotIn("sequential-write-multi-file", dashboard)


class DeveloperBenchmarkContractTests(unittest.TestCase):
    def test_missing_workload_reference_has_actionable_error(self):
        manifest = {
            "fixture_version": "test",
            "suites": {
                "regression": {"workloads": [{"id": "known-workload"}]},
                "quick": {"workloads": [{"ref": "missing-workload"}]},
            },
        }
        with (
            mock.patch.object(runner, "load_manifest", return_value=manifest),
            self.assertRaisesRegex(
                ValueError,
                "Suite 'quick' references unknown regression workload "
                "'missing-workload' at index 0",
            ),
        ):
            runner.load_suite("quick")

    def test_metadata_workloads_use_larger_operation_samples(self):
        manifest = runner.load_manifest()
        regression = manifest["suites"]["regression"]
        workloads = {workload["id"]: workload for workload in regression["workloads"]}
        metadata_jobs = {
            "metadata-create-2k": ("metadata_create.fio", 2000, "nrfiles=250", None),
            "metadata-stat-100k": ("metadata_stat.fio", 100000, "nrfiles=125", "loops=100"),
            "metadata-delete-2k": ("metadata_delete.fio", 2000, "nrfiles=250", None),
        }

        for workload_id, (filename, operations, file_count, loops) in metadata_jobs.items():
            workload = workloads[workload_id]
            self.assertEqual(workload["parameters"]["concurrency"], 8)
            self.assertEqual(workload["parameters"]["operations"], operations)
            self.assertEqual(workload["primary_metric"], "operations_per_second")
            job = (
                runner.BENCHMARK_CONFIG_DIR / "regression" / filename
            ).read_text(encoding="utf-8")
            self.assertIn(file_count, job)
            self.assertIn("numjobs=8", job)
            self.assertNotIn("lat_percentiles", job)
            self.assertNotIn("percentile_list", job)
            if loops is not None:
                self.assertIn(loops, job)
            self.assertIn(f"[{workload_id}]", job)

        stat_parameters = workloads["metadata-stat-100k"]["parameters"]
        self.assertEqual(stat_parameters["cache_state"], "warm")
        self.assertEqual(stat_parameters["unique_files"], 1000)
        self.assertEqual(stat_parameters["repetitions_per_file"], 100)
        dashboard = (
            REPO_ROOT / "perf_testing/dashboard/developer/index.html"
        ).read_text(encoding="utf-8")
        self.assertIn("operations_per_second:'ops/s'", dashboard)
        self.assertEqual(
            comparator.METRIC_LABELS["operations_per_second"],
            ("Operation rate", "ops/s"),
        )

        quick_refs = {
            item["ref"] for item in manifest["suites"]["quick"]["workloads"]
        }
        self.assertIn("metadata-create-2k", quick_refs)
        self.assertNotIn("metadata-create-1k", quick_refs)


class WorkflowShapeTests(unittest.TestCase):
    def test_scheduled_workflow_acquires_one_runner_per_architecture(self):
        workflow = (REPO_ROOT / ".github/workflows/benchmark.yml").read_text(encoding="utf-8")
        comparison_workflow = (
            REPO_ROOT / ".github/workflows/benchmark-compare.yml"
        ).read_text(encoding="utf-8")
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
        self.assertIn("expected_vm_size: Standard_D96ds_v5", workflow)
        self.assertNotIn("expected_cpu_count: 192", workflow)
        self.assertNotIn("expected_vm_size: Standard_D192ds_v6", workflow)
        self.assertEqual(workflow.count("uses: ./.github/actions/perftesting"), 9)
        self.assertEqual(workflow.count('SETUP_HOST: "true"'), 1)
        self.assertEqual(workflow.count('SETUP_HOST: "false"'), 8)
        self.assertEqual(workflow.count("continue-on-error: true"), len(profiles))
        for profile in profiles:
            self.assertEqual(workflow.count(f"Run {profile} profile"), 1)
        self.assertIn("always() && steps.host_setup.outcome == 'success'", workflow)
        self.assertIn('"$required" == "true" && "$outcome" != "success"', workflow)
        self.assertNotIn("actions/upload-artifact", workflow)
        self.assertNotIn("actions/download-artifact", workflow)
        self.assertNotIn("actions/upload-artifact", comparison_workflow)
        self.assertNotIn("actions/download-artifact", comparison_workflow)
        self.assertIn("blobfuse2-perf-${{ matrix.arch }}-suite-${requested_suite}-status-${profile_status}", workflow)
        self.assertIn("benchmark-runs/run-${GITHUB_RUN_ID}-attempt-${GITHUB_RUN_ATTEMPT}", workflow)
        self.assertIn("benchmark-comparisons/run-${GITHUB_RUN_ID}-attempt-${GITHUB_RUN_ATTEMPT}", comparison_workflow)
        self.assertIn("PERF_RESULTS_ACCOUNT", workflow)
        self.assertIn("PERF_RESULTS_KEY", workflow)
        self.assertIn("PERF_RESULTS_ACCOUNT", comparison_workflow)
        self.assertIn("PERF_RESULTS_KEY", comparison_workflow)
        self.assertNotIn("matrix.account_type", workflow)
        self.assertNotIn("matrix.cache_mode", workflow)
        self.assertIn('default: "5"', comparison_workflow)
        python_check = "sys.version_info >= (3, 10)"
        self.assertIn(python_check, comparison_workflow)
        self.assertIn(python_check, workflow)
        self.assertIn(
            python_check,
            (REPO_ROOT / ".github/actions/perftesting/action.yml").read_text(
                encoding="utf-8"
            ),
        )

    def test_comparison_provisions_runner_owned_mountpoints(self):
        comparison_workflow = (
            REPO_ROOT / ".github/workflows/benchmark-compare.yml"
        ).read_text(encoding="utf-8")

        self.assertIn(
            'mount_prefix="/mnt/blobfuse-benchmark-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"',
            comparison_workflow,
        )
        self.assertIn(
            "for label in fixture-setup baseline candidate",
            comparison_workflow,
        )
        self.assertIn(
            'sudo -n install -d -m 0755 -o "$(id -u)" -g "$(id -g)" "$mount_dir"',
            comparison_workflow,
        )
        self.assertIn(
            '--mount-dir "${PERF_MOUNT_PREFIX}-fixture-setup"',
            comparison_workflow,
        )
        self.assertIn(
            '--mount-dir "${PERF_MOUNT_PREFIX}-${label}"',
            comparison_workflow,
        )
        self.assertIn(
            'sudo -n rmdir "${PERF_MOUNT_PREFIX}-${label}"',
            comparison_workflow,
        )

    def test_comparison_runs_quick_and_public_suites(self):
        comparison_workflow = (
            REPO_ROOT / ".github/workflows/benchmark-compare.yml"
        ).read_text(encoding="utf-8")

        self.assertIn("timeout-minutes: 480", comparison_workflow)
        self.assertIn("suites=(quick public)", comparison_workflow)
        self.assertEqual(comparison_workflow.count('--suite "$suite"'), 2)
        self.assertIn(
            '--output-dir "$PERF_ROOT/results/fixture-setup/${suite}"',
            comparison_workflow,
        )
        self.assertIn(
            '--output-dir "$PERF_ROOT/results/${label}/${suite}"',
            comparison_workflow,
        )
        self.assertEqual(comparison_workflow.count('--cache-dir "$PERF_ROOT/cache"'), 2)
        self.assertNotIn('--cache-dir "$PERF_ROOT/cache-${label}"', comparison_workflow)
        self.assertIn(
            '--baseline "$PERF_ROOT/results/baseline/quick/summary.json"',
            comparison_workflow,
        )
        self.assertIn(
            '--candidate "$PERF_ROOT/results/candidate/public/summary.json"',
            comparison_workflow,
        )
        self.assertIn("1024 * 1024 * 1024 * 1024", comparison_workflow)
        self.assertIn("suites=quick,public", comparison_workflow)

    def test_profile_results_are_isolated_below_architecture_artifacts(self):
        action = (REPO_ROOT / ".github/actions/perftesting/action.yml").read_text(encoding="utf-8")
        comparison_workflow = (
            REPO_ROOT / ".github/workflows/benchmark-compare.yml"
        ).read_text(encoding="utf-8")
        runner_source = (REPO_ROOT / "perf_testing/scripts/run_benchmark.py").read_text(
            encoding="utf-8"
        )
        result_path = (
            "benchmark-results/${{ inputs.ARCH }}/"
            "${{ inputs.ACCOUNT_TYPE }}/${{ inputs.CACHE_MODE }}"
        )

        self.assertIn(result_path, action)
        self.assertIn("RUN_PROFILE:", action)
        self.assertIn("inputs.RUN_PROFILE == 'true'", action)
        self.assertIn('sudo -n install -d -m 0755 -o "$(id -u)" -g "$(id -g)" "$PERF_MOUNT_DIR"', action)
        self.assertIn("findmnt -rn -o TARGET -T", action)
        self.assertIn("PERF_LOCAL_CACHE_ROOT", action)
        self.assertIn("200 * 1024 * 1024 * 1024", action)
        self.assertIn('$cache_root/blobfuse-benchmark-cache', action)
        self.assertNotIn("mountpoint -q /mnt/localssd", action)
        self.assertIn("findmnt -rn -o TARGET -T", comparison_workflow)
        self.assertNotIn("mountpoint -q /mnt/localssd", comparison_workflow)
        self.assertIn("Collect benchmark failure diagnostics", action)
        self.assertIn("Archive redacted benchmark configuration", action)
        self.assertIn("mount-config.redacted.yaml", action)
        self.assertIn("$PERF_RESULTS/diagnostics", action)
        self.assertIn('self.output_dir / "blobfuse2.log"', runner_source)
        self.assertIn('f"--log-file-path={self.blobfuse_log_path}"', runner_source)
        self.assertIn('f"--default-working-dir={self.blobfuse_work_dir}"', runner_source)
        self.assertIn("wait_for_blobfuse_exit", runner_source)
        self.assertNotIn("/dev/nvme", action)
        self.assertNotIn("mdadm", action)

        file_cache_config = (
            REPO_ROOT / "testdata/config/azure_file_benchmark.yaml"
        ).read_text(encoding="utf-8")
        self.assertNotIn("cleanup-on-start:", file_cache_config)

    def test_private_zip_transport_keeps_credentials_out_of_archives(self):
        scheduled = (REPO_ROOT / ".github/workflows/benchmark.yml").read_text(
            encoding="utf-8"
        )
        comparison = (
            REPO_ROOT / ".github/workflows/benchmark-compare.yml"
        ).read_text(encoding="utf-8")
        helper = (REPO_ROOT / "perf_testing/scripts/blob_results.sh").read_text(
            encoding="utf-8"
        )
        results_config = (
            REPO_ROOT / "testdata/config/azure_results_storage.yaml"
        ).read_text(encoding="utf-8")

        self.assertIn('zip -q -r "$bundle" benchmark-results', scheduled)
        self.assertIn("for directory in report results", comparison)
        self.assertNotIn('cp -a "$PERF_ROOT/cache', comparison)
        self.assertIn("mount-config.redacted.yaml", comparison)
        self.assertIn("<redacted>", comparison)
        self.assertIn("container-name=results", helper)
        self.assertIn('cache_dir="$transfer_root/cache"', helper)
        self.assertIn('--temp-path="$cache_dir"', helper)
        self.assertIn('chmod 0600 "$config_file"', helper)
        self.assertIn("umask 077", helper)
        self.assertIn("sha256sum", helper)
        self.assertIn("checksum verification failed", helper)
        self.assertIn("trap cleanup EXIT", helper)
        self.assertNotIn("block_cache", results_config)
        self.assertIn("  - file_cache", results_config)
        self.assertIn("file_cache:", results_config)
        self.assertIn("  path: { 1 }", results_config)
        self.assertIn("  sync-to-flush: true", results_config)
        self.assertIn('sync "$remote_file"', helper)
        self.assertIn('sync "$checksum_file"', helper)
        self.assertIn("  - azstorage", results_config)


class FioResultTests(unittest.TestCase):
    def test_cache_path_validation_rejects_destructive_locations(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            safe_cache = root / "blobfuse-benchmark" / "cache"
            common = {
                "mount_dir": root / "mount",
                "output_dir": root / "results",
                "binary": root / "bin" / "blobfuse2",
                "config": root / "config" / "mount.yaml",
            }
            runner.validate_cache_path(safe_cache, **common)

            unsafe_paths = (
                Path("/"),
                Path("/mnt"),
                Path("/var/lib/application/cache"),
                common["mount_dir"],
                common["mount_dir"] / "cache",
                common["output_dir"].parent,
                runner.REPO_ROOT,
                runner.REPO_ROOT.parent,
            )
            for unsafe_path in unsafe_paths:
                with self.subTest(cache_dir=unsafe_path):
                    with self.assertRaisesRegex(ValueError, "Unsafe cache directory"):
                        runner.validate_cache_path(unsafe_path, **common)

            repository_link = root / "blobfuse-repository-link"
            repository_link.symlink_to(runner.REPO_ROOT, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "repository checkout"):
                runner.validate_cache_path(repository_link, **common)

    def test_clear_local_state_checks_cache_path_before_deleting(self):
        benchmark_runner = runner.BenchmarkRunner.__new__(runner.BenchmarkRunner)
        benchmark_runner.cache_dir = Path("/mnt")
        benchmark_runner.mount_dir = Path("/mnt/blobfuse-benchmark")
        benchmark_runner.output_dir = runner.REPO_ROOT / "benchmark-results"
        benchmark_runner.binary = runner.REPO_ROOT / "blobfuse2"
        benchmark_runner.config = Path("/tmp/blobfuse/config.yaml")

        with (
            mock.patch.object(runner.shutil, "rmtree") as rmtree,
            self.assertRaisesRegex(ValueError, "Unsafe cache directory"),
        ):
            benchmark_runner.clear_local_state()
        rmtree.assert_not_called()

    def test_run_fio_times_the_complete_fio_process(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            benchmark_runner = runner.BenchmarkRunner.__new__(runner.BenchmarkRunner)
            benchmark_runner.output_dir = root / "output"
            benchmark_runner.output_dir.mkdir()
            result_path = root / "raw" / "result.json"
            work_dir = root / "work"
            command_result = mock.Mock(returncode=0, stdout="")

            with (
                mock.patch.object(runner, "run_command", return_value=command_result) as run,
                mock.patch.object(runner.time, "monotonic", side_effect=(10.0, 14.5)),
            ):
                elapsed = benchmark_runner.run_fio(
                    "public/seq_write_single.fio", work_dir, result_path
                )

        self.assertEqual(elapsed, 4.5)
        command = run.call_args.args[0]
        self.assertEqual(command[0], "fio")
        self.assertIn(f"--output={result_path}", command)

    def test_file_cache_validation_requires_sync_to_flush(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / "blobfuse2"
            binary.touch()
            config = root / "config.yaml"
            benchmark_runner = runner.BenchmarkRunner.__new__(runner.BenchmarkRunner)
            benchmark_runner.binary = binary
            benchmark_runner.config = config
            benchmark_runner.cache_dir = root / "blobfuse-test" / "cache"
            benchmark_runner.mount_dir = root / "mount"
            benchmark_runner.output_dir = root / "results"
            benchmark_runner.args = argparse.Namespace(cache_mode="file_cache", suite="regression")
            benchmark_runner.suite = {"workloads": [], "fixture_jobs": []}

            with mock.patch.object(runner.shutil, "which", return_value="/usr/bin/fio"):
                config.write_text("file_cache:\n  path: /tmp/cache\n")
                with self.assertRaisesRegex(ValueError, "sync-to-flush"):
                    benchmark_runner.validate()

                config.write_text("file_cache:\n  path: /tmp/cache\n  sync-to-flush: true\n")
                benchmark_runner.validate()

    def test_durable_workload_validation_requires_end_fsync(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / "blobfuse2"
            binary.touch()
            config = root / "config.yaml"
            config.write_text("components:\n  - libfuse\n")
            jobs = root / "jobs"
            jobs.mkdir()
            job = jobs / "write.fio"
            benchmark_runner = runner.BenchmarkRunner.__new__(runner.BenchmarkRunner)
            benchmark_runner.binary = binary
            benchmark_runner.config = config
            benchmark_runner.cache_dir = root / "blobfuse-test" / "cache"
            benchmark_runner.mount_dir = root / "mount"
            benchmark_runner.output_dir = root / "results"
            benchmark_runner.args = argparse.Namespace(cache_mode="block_cache", suite="regression")
            benchmark_runner.suite = {
                "workloads": [
                    {
                        "id": "durable-write",
                        "job_file": "write.fio",
                        "parameters": {"durability": "end fsync"},
                    }
                ],
                "fixture_jobs": [],
            }

            with (
                mock.patch.object(runner, "BENCHMARK_CONFIG_DIR", jobs),
                mock.patch.object(runner.shutil, "which", return_value="/usr/bin/fio"),
            ):
                job.write_text("[global]\nrw=write\n\n[durable-write]\n")
                with self.assertRaisesRegex(ValueError, "end_fsync=1"):
                    benchmark_runner.validate()

                job.write_text("[global]\nrw=write\nend_fsync=1\n\n[durable-write]\n")
                benchmark_runner.validate()

    def test_run_records_resolved_trial_count(self):
        with tempfile.TemporaryDirectory() as temporary:
            benchmark_runner = runner.BenchmarkRunner.__new__(runner.BenchmarkRunner)
            benchmark_runner.output_dir = Path(temporary) / "results"
            benchmark_runner.raw_dir = benchmark_runner.output_dir / "raw"
            benchmark_runner.suite = {"trials": 5, "workloads": []}
            benchmark_runner.args = argparse.Namespace(
                trials=None,
                validate_only=False,
                prepare_only=False,
                run_id="run-1",
                label="main",
                suite="public",
                commit="a" * 40,
                ref="refs/heads/main",
                architecture="X86",
                account_type="premium",
                cache_mode="block_cache",
            )
            benchmark_runner.validate = mock.Mock()
            benchmark_runner.environment = mock.Mock(
                return_value={"compute_profile": "Standard_D96ds_v5"}
            )
            benchmark_runner.ensure_fixtures = mock.Mock()
            benchmark_runner.unmount = mock.Mock()
            benchmark_runner.cleanup_remote_run = mock.Mock()

            result = benchmark_runner.run()

        self.assertEqual(result["run"]["trials"], 5)

    def test_mount_writes_blobfuse_log_to_the_suite_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            benchmark_runner = runner.BenchmarkRunner.__new__(runner.BenchmarkRunner)
            benchmark_runner.binary = root / "blobfuse2"
            benchmark_runner.config = root / "config.yaml"
            benchmark_runner.mount_dir = root / "mount"
            benchmark_runner.output_dir = root / "results"
            benchmark_runner.blobfuse_log_path = benchmark_runner.output_dir / "blobfuse2.log"
            benchmark_runner.blobfuse_work_dir = benchmark_runner.output_dir / "blobfuse-work"
            benchmark_runner.blobfuse_pid_path = benchmark_runner.blobfuse_work_dir / "mount.pid"
            benchmark_runner.mounted = False
            benchmark_runner.output_dir.mkdir()
            benchmark_runner.unmount = mock.Mock()
            benchmark_runner.clear_local_state = mock.Mock()
            benchmark_runner.is_mounted = mock.Mock(return_value=True)

            command_result = mock.Mock(returncode=0, stdout="")
            with mock.patch.object(runner, "run_command", return_value=command_result) as run:
                benchmark_runner.mount()

            command = run.call_args.args[0]
            self.assertIn(
                f"--log-file-path={benchmark_runner.blobfuse_log_path}",
                command,
            )
            self.assertIn(
                f"--default-working-dir={benchmark_runner.blobfuse_work_dir}",
                command,
            )
            self.assertEqual(run.call_args.kwargs["cwd"], benchmark_runner.output_dir)

    def test_wait_for_blobfuse_exit_waits_for_pid_file_removal(self):
        with tempfile.TemporaryDirectory() as temporary:
            benchmark_runner = runner.BenchmarkRunner.__new__(runner.BenchmarkRunner)
            benchmark_runner.mount_dir = Path(temporary) / "mount"
            benchmark_runner.blobfuse_pid_path = Path(temporary) / "blobfuse.pid"
            benchmark_runner.blobfuse_pid_path.touch()

            def remove_pid_file():
                time.sleep(0.1)
                os.remove(benchmark_runner.blobfuse_pid_path)

            remover = threading.Thread(target=remove_pid_file)
            remover.start()
            benchmark_runner.wait_for_blobfuse_exit(timeout=2)
            remover.join()

        self.assertFalse(benchmark_runner.blobfuse_pid_path.exists())

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
        self.assertEqual(metrics["end_to_end_throughput_mib_s"], 2.0)
        self.assertEqual(metrics["end_to_end_seconds"], 4.0)
        self.assertEqual(metrics["fio_throughput_mib_s"], 4.0)
        self.assertEqual(metrics["completed_operations"], 8.0)
        self.assertEqual(metrics["operations_per_second"], 4.0)
        self.assertEqual(metrics["operation_duration_seconds"], 2.0)
        self.assertEqual(metrics["latency_p99_ms"], 5.0)
        self.assertEqual(metrics["sync_latency_p99_ms"], 20.0)

    def test_parser_ignores_zero_sample_metadata_percentiles(self):
        fio_result = {
            "jobs": [
                {
                    "jobname": "metadata-create-2k",
                    "error": 0,
                    "write": {
                        "total_ios": 2000,
                        "io_bytes": 0,
                        "iops": 125.0,
                        "lat_ns": {
                            "mean": 0,
                            "N": 0,
                            "percentile": {"99.000000": 0},
                        },
                        "clat_ns": {
                            "N": 0,
                            "percentile": {"99.000000": 0},
                        },
                    },
                }
            ]
        }
        with tempfile.TemporaryDirectory() as temporary:
            result_path = Path(temporary) / "fio.json"
            result_path.write_text(json.dumps(fio_result))
            metrics = runner.parse_fio_result(result_path, "write", wall_seconds=20.0)

        self.assertEqual(metrics["operations_per_second"], 125.0)
        self.assertEqual(metrics["completed_operations"], 2000.0)
        self.assertNotIn("latency_p99_ms", metrics)

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

    def test_metadata_operation_count_and_rate_are_required(self):
        workload = {"id": "metadata-create-2k", "parameters": {"operations": 2000}}
        runner.validate_operation_metrics(
            workload,
            {"completed_operations": 2000.0, "operations_per_second": 125.0},
        )

        with self.assertRaisesRegex(RuntimeError, "completed 1999, expected 2000"):
            runner.validate_operation_metrics(
                workload,
                {"completed_operations": 1999.0, "operations_per_second": 125.0},
            )
        with self.assertRaisesRegex(RuntimeError, "0.0 ops/s"):
            runner.validate_operation_metrics(
                workload,
                {"completed_operations": 2000.0, "operations_per_second": 0.0},
            )

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
    def test_cli_combines_repeated_summary_pairs(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            arguments = ["compare_benchmarks.py"]
            for suite, benchmark_id, candidate_value in (
                ("quick", "quick-read", 980.0),
                ("public", "public-read", 850.0),
            ):
                baseline = make_summary(suite=suite, value=1000.0)
                candidate = make_summary(suite=suite, value=candidate_value)
                baseline["benchmarks"][0]["id"] = benchmark_id
                candidate["benchmarks"][0]["id"] = benchmark_id
                baseline_path = root / f"baseline-{suite}.json"
                candidate_path = root / f"candidate-{suite}.json"
                baseline_path.write_text(json.dumps(baseline))
                candidate_path.write_text(json.dumps(candidate))
                arguments.extend(
                    [
                        "--baseline",
                        str(baseline_path),
                        "--candidate",
                        str(candidate_path),
                    ]
                )
            output_dir = root / "report"
            arguments.extend(["--output-dir", str(output_dir)])

            with mock.patch.object(comparator.sys, "argv", arguments):
                status = comparator.main()

            comparison = json.loads((output_dir / "comparison.json").read_text())
            self.assertEqual(status, 0)
            self.assertEqual(comparison["suites"], ["quick", "public"])
            self.assertEqual(comparison["overall_status"], "regression")
            self.assertTrue((output_dir / "comparison.html").is_file())
            self.assertTrue((output_dir / "summary.md").is_file())

    def test_quick_and_public_comparisons_are_combined(self):
        comparisons = []
        for suite, benchmark_id, baseline_value, candidate_value in (
            ("quick", "quick-read", 1000.0, 980.0),
            ("public", "public-read", 1000.0, 850.0),
        ):
            baseline = make_summary(suite=suite, value=baseline_value)
            candidate = make_summary(suite=suite, value=candidate_value)
            baseline["benchmarks"][0]["id"] = benchmark_id
            candidate["benchmarks"][0]["id"] = benchmark_id
            comparisons.append(
                comparator.compare_summaries(baseline, candidate, 10.0, 20.0)
            )

        combined = comparator.combine_comparisons(comparisons)

        self.assertEqual(combined["suites"], ["quick", "public"])
        self.assertEqual(combined["baseline"]["suite"], "quick+public")
        self.assertEqual(combined["overall_status"], "regression")
        self.assertEqual(
            [(item["suite"], item["id"]) for item in combined["benchmarks"]],
            [("quick", "quick-read"), ("public", "public-read")],
        )
        self.assertIn("| public |", comparator.render_markdown(combined))
        self.assertIn('<span class="suite">public</span>', comparator.render_html(combined))

    def test_comparison_rejects_mismatched_suites(self):
        with self.assertRaisesRegex(
            ValueError,
            "Cannot compare baseline suite 'quick' with candidate suite 'public'",
        ):
            comparator.compare_summaries(
                make_summary(suite="quick"),
                make_summary(suite="public"),
                10.0,
                20.0,
            )

    def test_metadata_operation_rate_regression_uses_ops_per_second(self):
        baseline = make_summary(value=1000.0)
        candidate = make_summary(value=1000.0)
        for summary, value in ((baseline, 1000.0), (candidate, 850.0)):
            benchmark = summary["benchmarks"][0]
            benchmark["id"] = "metadata-create-2k"
            benchmark["name"] = "Parallel file create, 2,000 operations"
            benchmark["primary_metric"] = "operations_per_second"
            benchmark["metrics"] = {
                "operations_per_second": {
                    "median": value,
                    "mad": 10.0,
                    "min": value - 20.0,
                    "max": value + 20.0,
                }
            }

        comparison = comparator.compare_summaries(baseline, candidate, 10.0, 20.0)
        self.assertEqual(comparison["overall_status"], "regression")
        self.assertEqual(comparison["benchmarks"][0]["status"], "regression")
        self.assertIn("1,000 ops/s", comparator.render_markdown(comparison))

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
    def test_comparator_loader_reports_missing_spec_or_loader(self):
        comparator_path = Path("/missing/compare_benchmarks.py")
        for spec in (None, mock.Mock(loader=None)):
            with (
                self.subTest(spec=spec),
                mock.patch.object(
                    regression_checker.importlib.util,
                    "spec_from_file_location",
                    return_value=spec,
                ),
                mock.patch.object(
                    regression_checker.importlib.util,
                    "module_from_spec",
                ) as module_from_spec,
                self.assertRaisesRegex(
                    ImportError,
                    "Unable to load benchmark comparator from "
                    "/missing/compare_benchmarks.py",
                ),
            ):
                regression_checker.load_comparator(comparator_path)
            module_from_spec.assert_not_called()

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

    def test_four_file_candidate_does_not_use_legacy_sixteen_file_history(self):
        candidate = make_summary(suite="public")
        candidate["benchmarks"][0]["id"] = "sequential-read-four-file"
        history_runs = []
        for index in range(5):
            historical = make_summary(suite="public")
            historical["key"] = str(index)
            historical["generated_at"] = f"2026-07-{20 + index:02d}T00:00:00Z"
            historical["benchmarks"][0]["id"] = "sequential-read-multi-file"
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

    def test_five_trial_candidate_does_not_use_three_trial_history(self):
        candidate = make_summary(trials=5)
        history_runs = []
        for index in range(5):
            historical = make_summary(trials=3)
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
        self.assertIn("end_to_end_throughput_mib_s", public_benchmark["metrics"])
        self.assertIn("end_to_end_seconds", public_benchmark["metrics"])
        self.assertNotIn("fio_throughput_mib_s", public_benchmark["metrics"])
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
