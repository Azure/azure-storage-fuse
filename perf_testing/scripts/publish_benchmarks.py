#!/usr/bin/env python3

import argparse
import json
import shutil
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any


PUBLIC_METRICS = {
    "throughput_mib_s",
    "latency_p99_ms",
    "sync_latency_p99_ms",
    "io_gib",
    "elapsed_seconds",
    "network_to_io_ratio",
}

LEGACY_PERFORMANCE_PATHS = (
    "X86",
    "ARM64",
    "ARM",
    "legacy_runs",
)


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def parse_timestamp(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def load_json(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as json_file:
        return json.load(json_file)


def result_key(result: dict[str, Any]) -> str:
    run = result["run"]
    fields = (
        run["id"],
        run["label"],
        run["suite"],
        run["commit"],
        run["architecture"],
        run["account_type"],
        run["cache_mode"],
        run.get("compute_profile", "unknown"),
    )
    return ":".join(fields)


def compact_result(summary: dict[str, Any], *, public: bool) -> dict[str, Any]:
    run = summary["run"]
    if summary.get("schema_version") != 1:
        raise ValueError(f"Unsupported summary schema for run {run.get('id', 'unknown')}")

    benchmarks = []
    for benchmark in summary["benchmarks"]:
        metrics = benchmark["metrics"]
        if public:
            metrics = {name: value for name, value in metrics.items() if name in PUBLIC_METRICS}
        benchmarks.append(
            {
                "id": benchmark["id"],
                "name": benchmark["name"],
                "direction": benchmark["direction"],
                "primary_metric": benchmark["primary_metric"],
                "higher_is_better": benchmark["higher_is_better"],
                "parameters": benchmark.get("parameters", {}),
                "metrics": metrics,
            }
        )

    environment = summary.get("environment", {})
    compact_environment = {
        key: environment[key]
        for key in (
            "architecture",
            "account_type",
            "cache_mode",
            "kernel",
            "cpu_model",
            "cpu_count",
            "memory_gib",
            "vm_size",
            "azure_region",
            "compute_profile",
            "fio_version",
            "blobfuse_version",
            "config_sha256",
        )
        if key in environment
    }
    return {
        "key": result_key(summary),
        "generated_at": summary["generated_at"],
        "run": run,
        "environment": compact_environment,
        "benchmarks": benchmarks,
    }


def load_history(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {"schema_version": 1, "updated_at": None, "runs": []}
    history = load_json(path)
    if history.get("schema_version") != 1 or not isinstance(history.get("runs"), list):
        raise ValueError(f"Unsupported benchmark history schema in {path}")
    return history


def merge_history(
    path: Path,
    additions: list[dict[str, Any]],
    *,
    retention_days: int,
) -> dict[str, Any]:
    history = load_history(path)
    by_key = {run["key"]: run for run in history["runs"]}
    for addition in additions:
        by_key[addition["key"]] = addition

    cutoff = datetime.now(timezone.utc) - timedelta(days=retention_days)
    retained = [
        run for run in by_key.values() if parse_timestamp(run["generated_at"]) >= cutoff
    ]
    retained.sort(key=lambda run: (run["generated_at"], run["key"]))
    return {
        "schema_version": 1,
        "updated_at": utc_now(),
        "retention_days": retention_days,
        "runs": retained,
    }


def atomic_write_json(path: Path, data: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(data, separators=(",", ":")) + "\n", encoding="utf-8")
    temporary.replace(path)


def publish(args: argparse.Namespace) -> tuple[int, int]:
    summary_paths = sorted(args.results_dir.rglob("summary.json"))
    if not summary_paths:
        raise ValueError(f"No benchmark summaries found under {args.results_dir}")

    summaries = [load_json(path) for path in summary_paths]
    developer_additions = [compact_result(summary, public=False) for summary in summaries]
    public_additions = [
        compact_result(summary, public=True)
        for summary in summaries
        if summary["run"]["suite"] == "public"
        and summary["run"]["cache_mode"] == "block_cache"
        and summary["run"]["account_type"] in {"standard", "premium"}
    ]
    invalid_public = [
        summary
        for summary in summaries
        if summary["run"]["suite"] == "public"
        and (
            summary["run"]["cache_mode"] != "block_cache"
            or summary["run"]["account_type"] not in {"standard", "premium"}
        )
    ]
    if invalid_public:
        dimensions = [summary["run"] for summary in invalid_public]
        raise ValueError(f"Public result set contains unsupported dimensions: {dimensions}")

    public_path = args.site_dir / "data" / "public" / "history.json"
    developer_path = args.site_dir / "data" / "developer" / "history.json"
    public_history = merge_history(
        public_path,
        public_additions,
        retention_days=args.public_retention_days,
    )
    developer_history = merge_history(
        developer_path,
        developer_additions,
        retention_days=args.developer_retention_days,
    )
    atomic_write_json(developer_path, developer_history)

    public_dashboard = args.dashboard_dir / "public" / "index.html"
    developer_dashboard = args.dashboard_dir / "developer" / "index.html"
    branch_readme = args.dashboard_dir / "README.md"
    if not public_dashboard.is_file() or not developer_dashboard.is_file() or not branch_readme.is_file():
        raise FileNotFoundError("Benchmark dashboard sources are missing")
    args.site_dir.mkdir(parents=True, exist_ok=True)
    developer_target = args.site_dir / "developer"
    developer_target.mkdir(parents=True, exist_ok=True)
    shutil.copy2(developer_dashboard, developer_target / "index.html")
    if public_history["runs"]:
        atomic_write_json(public_path, public_history)
        for relative_path in LEGACY_PERFORMANCE_PATHS:
            legacy_path = args.site_dir / relative_path
            if legacy_path.is_dir():
                shutil.rmtree(legacy_path)
            elif legacy_path.exists():
                legacy_path.unlink()
        legacy_jekyll_config = args.site_dir / "_config.yaml"
        if legacy_jekyll_config.exists():
            legacy_jekyll_config.unlink()
        shutil.copy2(public_dashboard, args.site_dir / "index.html")
        shutil.copy2(branch_readme, args.site_dir / "README.md")
        (args.site_dir / ".nojekyll").touch()
    return len(public_additions), len(developer_additions)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Publish compact Blobfuse2 benchmark histories")
    parser.add_argument("--results-dir", type=Path, required=True)
    parser.add_argument("--site-dir", type=Path, required=True)
    parser.add_argument(
        "--dashboard-dir",
        type=Path,
        default=Path(__file__).resolve().parents[1] / "dashboard",
    )
    parser.add_argument("--public-retention-days", type=int, default=730)
    parser.add_argument("--developer-retention-days", type=int, default=400)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        public_count, developer_count = publish(args)
        print(
            f"Published {public_count} public summaries and "
            f"{developer_count} developer summaries"
        )
        return 0
    except Exception as error:
        print(f"Publishing failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
