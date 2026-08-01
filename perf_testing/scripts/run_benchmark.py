#!/usr/bin/env python3

import argparse
import copy
import hashlib
import json
import os
import platform
import shutil
import statistics
import subprocess
import sys
import time
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
BENCHMARK_CONFIG_DIR = REPO_ROOT / "perf_testing" / "config" / "benchmark"
SUITE_MANIFEST = BENCHMARK_CONFIG_DIR / "suites.json"
GIB = 1024**3

FIXTURE_LAYOUTS = {
    "setup/max_read.fio": [(f"max-read.{index}", 8 * GIB) for index in range(16)],
    "setup/warm_read.fio": [("warm-read.data", 512 * 1024**2)],
    "setup/public_single_read.fio": [("public-single-read.data", 320 * GIB)],
    "setup/public_multi_read.fio": [
        (f"public-multi-read.{index}", 80 * GIB) for index in range(4)
    ],
}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def log(message: str) -> None:
    print(f"[{datetime.now().strftime('%H:%M:%S')}] {message}", flush=True)


def run_command(
    command: list[str],
    *,
    cwd: Path | None = None,
    env: dict[str, str] | None = None,
    timeout: int = 300,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        command,
        cwd=cwd,
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        timeout=timeout,
        check=False,
    )
    if check and result.returncode != 0:
        output = result.stdout.strip()
        raise RuntimeError(
            f"Command failed with exit code {result.returncode}: {' '.join(command)}\n{output}"
        )
    return result


def load_manifest() -> dict[str, Any]:
    with SUITE_MANIFEST.open(encoding="utf-8") as manifest_file:
        manifest = json.load(manifest_file)
    if manifest.get("schema_version") != 1:
        raise ValueError("Unsupported benchmark suite manifest schema")
    return manifest


def load_suite(name: str) -> tuple[dict[str, Any], str]:
    manifest = load_manifest()
    suites = manifest["suites"]
    if name not in suites:
        raise ValueError(f"Unknown benchmark suite: {name}")

    suite = copy.deepcopy(suites[name])
    if any("ref" in workload for workload in suite["workloads"]):
        referenced = {
            workload["id"]: workload for workload in suites["regression"]["workloads"]
        }
        suite["workloads"] = [copy.deepcopy(referenced[item["ref"]]) for item in suite["workloads"]]
    return suite, manifest["fixture_version"]


def percentile_value(stats: dict[str, Any], percentile: str) -> float | None:
    percentile_map = stats.get("percentile", {})
    value = percentile_map.get(percentile)
    if value is None:
        return None
    return float(value) / 1_000_000


def direction_stats(job: dict[str, Any], direction: str) -> list[dict[str, Any]]:
    if direction == "mixed":
        return [job[item] for item in ("read", "write") if job.get(item, {}).get("total_ios", 0)]
    stats = job.get(direction, {})
    if stats.get("total_ios", 0) or stats.get("iops", 0):
        return [stats]
    return []


def parse_fio_result(result_path: Path, direction: str, wall_seconds: float) -> dict[str, float]:
    with result_path.open(encoding="utf-8") as result_file:
        fio_result = json.load(result_file)

    jobs = fio_result.get("jobs", [])
    if not jobs:
        raise ValueError(f"FIO produced no jobs in {result_path}")

    errors = [int(job.get("error", 0)) for job in jobs]
    if any(errors):
        raise RuntimeError(f"FIO reported job errors in {result_path}: {errors}")

    io_bytes = 0.0
    fio_bandwidth_mib = 0.0
    iops = 0.0
    latency_sum = 0.0
    latency_count = 0.0
    latency_percentiles: dict[str, list[float]] = {"50.000000": [], "95.000000": [], "99.000000": [], "99.900000": []}
    sync_percentiles: list[float] = []

    for job in jobs:
        stats_groups = direction_stats(job, direction)
        if not stats_groups:
            raise ValueError(f"FIO job {job.get('jobname')} has no {direction} statistics")
        for stats in stats_groups:
            io_bytes += float(stats.get("io_bytes", 0))
            if "bw_bytes" in stats:
                fio_bandwidth_mib += float(stats["bw_bytes"]) / 1024**2
            else:
                fio_bandwidth_mib += float(stats.get("bw", 0)) / 1024
            iops += float(stats.get("iops", 0))

            latency = stats.get("lat_ns", {})
            count = float(latency.get("N", stats.get("total_ios", 0)))
            if count > 0 and latency.get("mean") is not None:
                latency_sum += float(latency["mean"]) * count
                latency_count += count

            completion = stats.get("lat_ns", {}).get("percentile", {})
            if not completion:
                completion = stats.get("clat_ns", {}).get("percentile", {})
            for percentile in latency_percentiles:
                if percentile in completion:
                    latency_percentiles[percentile].append(float(completion[percentile]) / 1_000_000)

        sync_p99 = percentile_value(job.get("sync", {}).get("lat_ns", {}), "99.000000")
        if sync_p99 is not None:
            sync_percentiles.append(sync_p99)

    metrics = {
        "io_gib": io_bytes / GIB,
        "elapsed_seconds": wall_seconds,
        "throughput_mib_s": (io_bytes / 1024**2) / wall_seconds if io_bytes and wall_seconds else 0.0,
        "fio_throughput_mib_s": fio_bandwidth_mib,
        "iops": iops,
    }
    if latency_count:
        metrics["latency_mean_ms"] = (latency_sum / latency_count) / 1_000_000
    for percentile, values in latency_percentiles.items():
        if values:
            label = percentile.split(".")[0]
            if percentile == "99.900000":
                label = "99_9"
            metrics[f"latency_p{label}_ms"] = max(values)
    if sync_percentiles:
        metrics["sync_latency_p99_ms"] = max(sync_percentiles)
    return metrics


def summarize_samples(samples: list[dict[str, Any]]) -> dict[str, dict[str, float]]:
    metric_names = sorted(
        {
            metric
            for sample in samples
            for metric, value in sample["metrics"].items()
            if isinstance(value, (int, float))
        }
    )
    summary: dict[str, dict[str, float]] = {}
    for metric in metric_names:
        values = [float(sample["metrics"][metric]) for sample in samples if metric in sample["metrics"]]
        median = statistics.median(values)
        absolute_deviations = [abs(value - median) for value in values]
        summary[metric] = {
            "median": median,
            "mad": statistics.median(absolute_deviations),
            "min": min(values),
            "max": max(values),
        }
    return summary


def network_to_io_ratio(direction: str, rx_mib: float, tx_mib: float, io_gib: float) -> float:
    if io_gib <= 0:
        return 0.0
    if direction == "read":
        network_mib = rx_mib
    elif direction == "write":
        network_mib = tx_mib
    else:
        network_mib = rx_mib + tx_mib
    return network_mib / (io_gib * 1024)


def azure_instance_metadata(opener: Any | None = None) -> dict[str, str]:
    if opener is None:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    request = urllib.request.Request(
        "http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01",
        headers={"Metadata": "true"},
    )
    try:
        with opener.open(request, timeout=2) as response:
            payload = json.load(response)
    except (OSError, ValueError):
        return {}
    return {
        key: str(payload[source])
        for key, source in (("vm_size", "vmSize"), ("azure_region", "location"))
        if payload.get(source)
    }


def compute_profile(environment: dict[str, Any]) -> str:
    vm_size = environment.get("vm_size")
    if vm_size:
        return str(vm_size)
    cpu_count = environment.get("cpu_count", "unknown")
    memory_gib = environment.get("memory_gib", "unknown")
    return f"{cpu_count} CPUs / {memory_gib} GiB"


def remove_trial_data(work_dir: Path, target: str, keep_data: bool) -> None:
    if target == "ephemeral" and not keep_data and work_dir.exists():
        shutil.rmtree(work_dir)


class BenchmarkRunner:
    def __init__(self, args: argparse.Namespace) -> None:
        self.args = args
        self.binary = Path(args.binary).resolve()
        self.mount_dir = Path(args.mount_dir).resolve()
        self.cache_dir = Path(args.cache_dir).resolve()
        self.config = Path(args.config).resolve()
        self.output_dir = Path(args.output_dir).resolve()
        self.raw_dir = self.output_dir / "raw"
        self.blobfuse_log_path = self.output_dir / "blobfuse2.log"
        self.suite, fixture_version = load_suite(args.suite)
        self.suite["workloads"] = [
            workload
            for workload in self.suite["workloads"]
            if args.cache_mode in workload.get("cache_modes", ("block_cache", "file_cache"))
        ]
        self.fixture_dir_name = f".blobfuse-benchmark-fixtures/{fixture_version}"
        self.run_dir_name = f".blobfuse-benchmark-runs/{args.run_id}/{args.label}/{args.suite}"
        self.network_interface = self.find_network_interface()
        self.mounted = False

    def validate(self) -> None:
        if not self.binary.is_file():
            raise FileNotFoundError(f"Blobfuse2 binary not found: {self.binary}")
        if not self.config.is_file():
            raise FileNotFoundError(f"Mount config not found: {self.config}")
        if shutil.which("fio") is None:
            raise RuntimeError("fio is not installed")
        config_text = self.config.read_text(encoding="utf-8")
        if "direct-io: true" in config_text:
            raise ValueError("Benchmark mount config must not enable mount-wide libfuse direct-io")
        for workload in self.suite["workloads"]:
            for field in ("job_file", "prepare_job"):
                if field in workload and not (BENCHMARK_CONFIG_DIR / workload[field]).is_file():
                    raise FileNotFoundError(f"Missing FIO job: {workload[field]}")
            if self.args.suite == "public":
                job_lines = (BENCHMARK_CONFIG_DIR / workload["job_file"]).read_text().splitlines()
                if not any(line.strip() == "direct=1" for line in job_lines):
                    raise ValueError(
                        f"Public workload {workload['id']} must open files with FIO direct=1"
                    )
        for fixture_job in self.suite.get("fixture_jobs", []):
            if fixture_job not in FIXTURE_LAYOUTS:
                raise ValueError(f"No fixture layout declared for {fixture_job}")

    def find_network_interface(self) -> str:
        route = run_command(["ip", "-j", "route", "show", "default"], check=False)
        if route.returncode != 0:
            raise RuntimeError(f"Unable to inspect the default network route: {route.stdout.strip()}")
        try:
            routes = json.loads(route.stdout)
        except json.JSONDecodeError as error:
            raise RuntimeError("Unable to parse the default network route") from error
        candidates = sorted(
            (
                item
                for item in routes
                if item.get("dev") and (Path("/sys/class/net") / item["dev"]).is_dir()
            ),
            key=lambda item: int(item.get("metric", 0)),
        )
        if not candidates:
            raise RuntimeError("No valid default-route interface found for benchmark validation")
        return str(candidates[0]["dev"])

    def network_bytes(self) -> tuple[int, int]:
        base = Path("/sys/class/net") / self.network_interface / "statistics"
        return int((base / "rx_bytes").read_text()), int((base / "tx_bytes").read_text())

    def clear_local_state(self) -> None:
        self.cache_dir.mkdir(parents=True, exist_ok=True)
        for child in self.cache_dir.iterdir():
            if child.is_dir() and not child.is_symlink():
                shutil.rmtree(child)
            else:
                child.unlink()
        drop = run_command(
            ["sudo", "-n", "sh", "-c", "sync; echo 3 > /proc/sys/vm/drop_caches"],
            timeout=60,
            check=False,
        )
        if drop.returncode != 0:
            raise RuntimeError(f"Unable to clear kernel page cache: {drop.stdout.strip()}")

    def is_mounted(self) -> bool:
        result = run_command(["findmnt", "-rn", "-T", str(self.mount_dir)], check=False)
        return result.returncode == 0 and str(self.mount_dir) in result.stdout

    def unmount(self) -> None:
        if not self.is_mounted():
            self.mounted = False
            return
        run_command([str(self.binary), "unmount", str(self.mount_dir)], timeout=90, check=False)
        for _ in range(30):
            if not self.is_mounted():
                self.mounted = False
                return
            time.sleep(1)
        run_command(["fusermount3", "-u", "-z", str(self.mount_dir)], timeout=30, check=False)
        if self.is_mounted():
            raise RuntimeError(f"Unable to unmount {self.mount_dir}")
        self.mounted = False

    def mount(self) -> None:
        self.unmount()
        self.clear_local_state()
        self.mount_dir.mkdir(parents=True, exist_ok=True)
        result = run_command(
            [
                str(self.binary),
                "mount",
                str(self.mount_dir),
                f"--config-file={self.config}",
                f"--log-file-path={self.blobfuse_log_path}",
            ],
            cwd=self.output_dir,
            timeout=120,
            check=False,
        )
        if result.returncode != 0:
            raise RuntimeError(f"Blobfuse2 mount failed: {result.stdout.strip()}")
        for _ in range(30):
            if self.is_mounted():
                self.mounted = True
                return
            time.sleep(1)
        raise RuntimeError(f"Blobfuse2 mount did not become ready: {result.stdout.strip()}")

    def run_fio(self, job_file: str, work_dir: Path, result_path: Path, timeout: int = 1200) -> float:
        work_dir.mkdir(parents=True, exist_ok=True)
        result_path.parent.mkdir(parents=True, exist_ok=True)
        command = [
            "fio",
            "--thread",
            "--eta=never",
            "--output-format=json+",
            f"--output={result_path}",
            f"--directory={work_dir}",
            str(BENCHMARK_CONFIG_DIR / job_file),
        ]
        start = time.monotonic()
        result = run_command(command, cwd=self.output_dir, timeout=timeout, check=False)
        elapsed = time.monotonic() - start
        if result.returncode != 0:
            raise RuntimeError(
                f"FIO job {job_file} failed with exit code {result.returncode}: {result.stdout.strip()}"
            )
        return elapsed

    def fixtures_ready(self, fixture_job: str, fixture_path: Path) -> bool:
        for relative_path, expected_size in FIXTURE_LAYOUTS[fixture_job]:
            path = fixture_path / relative_path
            try:
                if path.stat().st_size != expected_size:
                    return False
            except FileNotFoundError:
                return False
        return True

    def ensure_fixtures(self) -> None:
        fixture_jobs = self.suite.get("fixture_jobs", [])
        if not fixture_jobs:
            return
        log("Checking immutable read fixtures")
        self.mount()
        fixture_path = self.mount_dir / self.fixture_dir_name
        fixture_path.mkdir(parents=True, exist_ok=True)
        for fixture_job in fixture_jobs:
            if self.fixtures_ready(fixture_job, fixture_path):
                log(f"Fixture is ready: {fixture_job}")
                continue
            log(f"Preparing fixture: {fixture_job}")
            for relative_path, _ in FIXTURE_LAYOUTS[fixture_job]:
                path = fixture_path / relative_path
                if path.exists():
                    path.unlink()
            self.run_fio(
                fixture_job,
                fixture_path,
                self.raw_dir / "fixtures" / f"{Path(fixture_job).stem}.json",
                timeout=3600,
            )
            if not self.fixtures_ready(fixture_job, fixture_path):
                raise RuntimeError(f"Fixture validation failed after running {fixture_job}")
        self.unmount()

    def trial_work_dir(self, workload: dict[str, Any], trial: int) -> Path:
        if workload["target"] == "fixture":
            return self.mount_dir / self.fixture_dir_name
        return self.mount_dir / self.run_dir_name / workload["id"] / f"trial-{trial}"

    def run_trial(self, workload: dict[str, Any], trial: int) -> dict[str, Any]:
        self.mount()
        try:
            work_dir = self.trial_work_dir(workload, trial)
            work_dir.mkdir(parents=True, exist_ok=True)

            if workload.get("prepare_job"):
                prepare_path = self.raw_dir / workload["id"] / f"trial-{trial}-prepare.json"
                self.run_fio(workload["prepare_job"], work_dir, prepare_path)

            if workload.get("warmup"):
                warmup_path = self.raw_dir / workload["id"] / f"trial-{trial}-warmup.json"
                self.run_fio(workload["job_file"], work_dir, warmup_path)

            start_rx, start_tx = self.network_bytes()
            result_path = self.raw_dir / workload["id"] / f"trial-{trial}.json"
            wall_seconds = self.run_fio(workload["job_file"], work_dir, result_path)
            end_rx, end_tx = self.network_bytes()

            metrics = parse_fio_result(result_path, workload["direction"], wall_seconds)
            metrics["network_rx_mib"] = (end_rx - start_rx) / 1024**2
            metrics["network_tx_mib"] = (end_tx - start_tx) / 1024**2
            metrics["network_to_io_ratio"] = network_to_io_ratio(
                workload["direction"],
                metrics["network_rx_mib"],
                metrics["network_tx_mib"],
                metrics["io_gib"],
            )

            minimum_ratio = workload.get("expected_network_ratio")
            if minimum_ratio is not None and metrics.get("network_to_io_ratio", 0) < minimum_ratio:
                raise RuntimeError(
                    f"Network transfer invariant failed for {workload['id']}: network/io ratio "
                    f"{metrics.get('network_to_io_ratio', 0):.3f} < {minimum_ratio:.3f}"
                )

            remove_trial_data(work_dir, workload["target"], self.args.keep_data)

            return {
                "trial": trial,
                "metrics": metrics,
            }
        finally:
            self.unmount()

    def cleanup_remote_run(self) -> None:
        if self.args.keep_data:
            return
        try:
            self.mount()
            run_path = self.mount_dir / self.run_dir_name
            if run_path.exists():
                shutil.rmtree(run_path)
            self.unmount()
        except Exception as error:
            log(f"Remote cleanup warning: {error}")
            self.unmount()

    def environment(self) -> dict[str, Any]:
        cpu_model = "unknown"
        cpuinfo = Path("/proc/cpuinfo")
        if cpuinfo.exists():
            for line in cpuinfo.read_text(errors="replace").splitlines():
                if line.startswith(("model name", "Model")) and ":" in line:
                    cpu_model = line.split(":", 1)[1].strip()
                    break
        memory_kib = 0
        for line in Path("/proc/meminfo").read_text().splitlines():
            if line.startswith("MemTotal:"):
                memory_kib = int(line.split()[1])
                break

        fio_version = run_command(["fio", "--version"]).stdout.strip()
        blobfuse_version = run_command([str(self.binary), "--version"]).stdout.strip()
        instance_metadata = azure_instance_metadata()
        if os.environ.get("AZURE_VM_SIZE"):
            instance_metadata["vm_size"] = os.environ["AZURE_VM_SIZE"]
        if os.environ.get("AZURE_REGION"):
            instance_metadata["azure_region"] = os.environ["AZURE_REGION"]
        sanitized_config = "\n".join(
            "  account-key: <redacted>" if line.lstrip().startswith("account-key:") else line
            for line in self.config.read_text(encoding="utf-8").splitlines()
        )
        environment = {
            "architecture": self.args.architecture,
            "account_type": self.args.account_type,
            "cache_mode": self.args.cache_mode,
            "runner": platform.node(),
            "kernel": platform.release(),
            "cpu_model": cpu_model,
            "cpu_count": os.cpu_count(),
            "memory_gib": round(memory_kib / 1024**2, 2),
            "network_interface": self.network_interface,
            "fio_version": fio_version,
            "blobfuse_version": blobfuse_version,
            "config_sha256": hashlib.sha256(sanitized_config.encode()).hexdigest(),
            **instance_metadata,
        }
        environment["compute_profile"] = compute_profile(environment)
        return environment

    def run(self) -> dict[str, Any]:
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.raw_dir.mkdir(parents=True, exist_ok=True)
        self.validate()
        environment = self.environment()
        if self.args.validate_only:
            return {"environment": environment, "validated": True}
        if self.args.prepare_only:
            try:
                self.ensure_fixtures()
            finally:
                self.unmount()
            return {"environment": environment, "prepared": True}

        trials = self.args.trials or int(self.suite["trials"])
        benchmarks = []
        try:
            self.ensure_fixtures()
            for workload in self.suite["workloads"]:
                log(f"Running {workload['name']} ({trials} trials)")
                samples = []
                for trial in range(1, trials + 1):
                    log(f"  trial {trial}/{trials}")
                    samples.append(self.run_trial(workload, trial))
                metrics = summarize_samples(samples)
                primary = workload["primary_metric"]
                if primary not in metrics:
                    raise RuntimeError(f"Primary metric {primary} missing for {workload['id']}")
                benchmarks.append(
                    {
                        "id": workload["id"],
                        "name": workload["name"],
                        "direction": workload["direction"],
                        "primary_metric": primary,
                        "higher_is_better": workload["higher_is_better"],
                        "parameters": workload["parameters"],
                        "metrics": metrics,
                        "samples": samples,
                    }
                )
        finally:
            self.unmount()
            self.cleanup_remote_run()

        return {
            "schema_version": 1,
            "generated_at": utc_now(),
            "run": {
                "id": self.args.run_id,
                "label": self.args.label,
                "suite": self.args.suite,
                "commit": self.args.commit,
                "ref": self.args.ref,
                "architecture": self.args.architecture,
                "account_type": self.args.account_type,
                "cache_mode": self.args.cache_mode,
                "compute_profile": environment["compute_profile"],
            },
            "environment": environment,
            "benchmarks": benchmarks,
        }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run reproducible Blobfuse2 FIO benchmarks")
    parser.add_argument("--binary", required=True, help="Blobfuse2 binary to benchmark")
    parser.add_argument("--config", required=True, help="Generated Blobfuse2 mount configuration")
    parser.add_argument("--suite", choices=("public", "regression", "quick"), required=True)
    parser.add_argument("--mount-dir", required=True)
    parser.add_argument("--cache-dir", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--label", default="main")
    parser.add_argument("--commit", required=True)
    parser.add_argument("--ref", required=True)
    parser.add_argument("--architecture", required=True)
    parser.add_argument("--account-type", required=True)
    parser.add_argument("--cache-mode", choices=("block_cache", "file_cache"), required=True)
    parser.add_argument("--trials", type=int)
    parser.add_argument("--keep-data", action="store_true")
    parser.add_argument("--validate-only", action="store_true")
    parser.add_argument("--prepare-only", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    runner = BenchmarkRunner(args)
    try:
        result = runner.run()
        if args.validate_only or args.prepare_only:
            log("Benchmark configuration validated")
            return 0
        output_path = Path(args.output_dir) / "summary.json"
        temporary_path = output_path.with_suffix(".json.tmp")
        temporary_path.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
        temporary_path.replace(output_path)
        log(f"Benchmark summary written to {output_path}")
        return 0
    except Exception as error:
        Path(args.output_dir).mkdir(parents=True, exist_ok=True)
        failure = {
            "schema_version": 1,
            "generated_at": utc_now(),
            "run_id": args.run_id,
            "suite": args.suite,
            "label": args.label,
            "error": str(error),
        }
        (Path(args.output_dir) / "failure.json").write_text(
            json.dumps(failure, indent=2) + "\n", encoding="utf-8"
        )
        print(f"Benchmark failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
