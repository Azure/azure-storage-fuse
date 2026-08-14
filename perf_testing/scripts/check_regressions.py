#!/usr/bin/env python3

import argparse
import importlib.util
import json
import statistics
import sys
from pathlib import Path
from typing import Any


SCRIPT_DIR = Path(__file__).resolve().parent
COMPARATOR_PATH = SCRIPT_DIR / "compare_benchmarks.py"


def load_comparator(path: Path) -> Any:
    spec = importlib.util.spec_from_file_location("benchmark_comparator", path)
    if spec is None or spec.loader is None:
        raise ImportError(f"Unable to load benchmark comparator from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


comparator = load_comparator(COMPARATOR_PATH)


def load_json(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as json_file:
        return json.load(json_file)


def environment_key(result: dict[str, Any]) -> tuple[str, str, str, str]:
    run = result.get("run", {})
    environment = result.get("environment", {})
    profile = run.get("compute_profile") or environment.get("compute_profile")
    if not profile:
        profile = (
            f"{environment.get('cpu_count', 'unknown')} CPUs / "
            f"{environment.get('memory_gib', 'unknown')} GiB"
        )
    return (
        str(profile),
        str(environment.get("azure_region", "unknown")),
        str(environment.get("config_sha256", "unknown")),
        str(run.get("trials", "unknown")),
    )


def matching_history(history: dict[str, Any], candidate: dict[str, Any]) -> list[dict[str, Any]]:
    run = candidate["run"]
    candidate_environment = environment_key(candidate)
    matches = [
        item
        for item in history.get("runs", [])
        if item["run"]["suite"] == run["suite"]
        and item["run"]["architecture"] == run["architecture"]
        and item["run"]["account_type"] == run["account_type"]
        and item["run"]["cache_mode"] == run["cache_mode"]
        and environment_key(item) == candidate_environment
    ]
    return sorted(matches, key=lambda item: item["generated_at"])


def historical_metric(
    runs: list[dict[str, Any]],
    benchmark_id: str,
    metric_name: str,
    window: int,
) -> tuple[dict[str, float] | None, int]:
    values = []
    for run in reversed(runs):
        benchmark = next((item for item in run["benchmarks"] if item["id"] == benchmark_id), None)
        metric = benchmark.get("metrics", {}).get(metric_name) if benchmark else None
        if metric and isinstance(metric.get("median"), (int, float)):
            values.append(float(metric["median"]))
        if len(values) == window:
            break
    if not values:
        return None, 0
    median = statistics.median(values)
    mad = statistics.median(abs(value - median) for value in values)
    return {
        "median": median,
        "mad": mad,
        "min": min(values),
        "max": max(values),
    }, len(values)


def evaluate_candidate(
    history: dict[str, Any],
    candidate: dict[str, Any],
    *,
    window: int,
    minimum_baselines: int,
    primary_threshold: float,
    latency_threshold: float,
) -> dict[str, Any]:
    prior_runs = matching_history(history, candidate)
    benchmark_checks = []
    for benchmark in candidate["benchmarks"]:
        metric_checks = {}
        metric_names = [benchmark["primary_metric"]]
        metric_names.extend(
            name
            for name in ("latency_p99_ms", "sync_latency_p99_ms")
            if name in benchmark["metrics"]
        )
        primary_baselines = 0
        for metric_name in metric_names:
            baseline, count = historical_metric(prior_runs, benchmark["id"], metric_name, window)
            if metric_name == benchmark["primary_metric"]:
                primary_baselines = count
            if baseline is None or count < minimum_baselines:
                continue
            metric_checks[metric_name] = comparator.compare_metric(
                baseline,
                benchmark["metrics"][metric_name],
                higher_is_better=(
                    bool(benchmark["higher_is_better"])
                    if metric_name == benchmark["primary_metric"]
                    else False
                ),
                threshold=(primary_threshold if metric_name == benchmark["primary_metric"] else latency_threshold),
            )

        statuses = {metric["status"] for metric in metric_checks.values()}
        if (
            primary_baselines < minimum_baselines
            or benchmark["primary_metric"] not in metric_checks
        ):
            status = "insufficient-baseline"
        elif "regression" in statuses:
            status = "regression"
        elif "noisy" in statuses:
            status = "noisy"
        elif "improvement" in statuses:
            status = "improvement"
        else:
            status = "stable"
        benchmark_checks.append(
            {
                "id": benchmark["id"],
                "name": benchmark["name"],
                "primary_metric": benchmark["primary_metric"],
                "baseline_count": primary_baselines,
                "status": status,
                "metrics": metric_checks,
            }
        )

    run_status = "regression" if any(item["status"] == "regression" for item in benchmark_checks) else "pass"
    return {
        "run": candidate["run"],
        "status": run_status,
        "benchmarks": benchmark_checks,
    }


def evaluate_results(
    history: dict[str, Any],
    candidates: list[dict[str, Any]],
    *,
    window: int,
    minimum_baselines: int,
    primary_threshold: float,
    latency_threshold: float,
) -> dict[str, Any]:
    checks = [
        evaluate_candidate(
            history,
            candidate,
            window=window,
            minimum_baselines=minimum_baselines,
            primary_threshold=primary_threshold,
            latency_threshold=latency_threshold,
        )
        for candidate in candidates
    ]
    return {
        "schema_version": 1,
        "overall_status": "regression" if any(item["status"] == "regression" for item in checks) else "pass",
        "window": window,
        "minimum_baselines": minimum_baselines,
        "checks": checks,
    }


def render_markdown(report: dict[str, Any]) -> str:
    lines = [
        "## Scheduled performance regression check",
        "",
        f"**Result:** {report['overall_status'].upper()}",
        "",
        "| Dimensions | Workload | Primary change | Assessment | Baseline runs |",
        "| --- | --- | ---: | --- | ---: |",
    ]
    for check in report["checks"]:
        run = check["run"]
        dimensions = (
            f"{run['architecture']} / {run['account_type']} / {run['cache_mode']} / "
            f"{run.get('compute_profile', 'unknown compute')}"
        )
        for benchmark in check["benchmarks"]:
            primary = benchmark["metrics"].get(benchmark["primary_metric"])
            change = "n/a" if primary is None else f"{primary['delta_percent']:+.1f}%"
            lines.append(
                f"| {dimensions} | {benchmark['name']} | {change} | "
                f"{benchmark['status']} | {benchmark['baseline_count']} |"
            )
    if not report["checks"]:
        lines.append("| n/a | No developer summaries found | n/a | n/a | 0 |")
    lines.append("")
    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check new Blobfuse2 results against rolling history")
    parser.add_argument("--history", type=Path, required=True)
    parser.add_argument("--results-dir", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--window", type=int, default=5)
    parser.add_argument("--minimum-baselines", type=int, default=3)
    parser.add_argument("--primary-threshold", type=float, default=10.0)
    parser.add_argument("--latency-threshold", type=float, default=20.0)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        history = load_json(args.history) if args.history.exists() else {"schema_version": 1, "runs": []}
        candidate_paths = sorted(args.results_dir.rglob("summary.json"))
        candidates = [load_json(path) for path in candidate_paths]
        report = evaluate_results(
            history,
            candidates,
            window=args.window,
            minimum_baselines=args.minimum_baselines,
            primary_threshold=args.primary_threshold,
            latency_threshold=args.latency_threshold,
        )
        args.output_dir.mkdir(parents=True, exist_ok=True)
        (args.output_dir / "regression-check.json").write_text(
            json.dumps(report, indent=2) + "\n", encoding="utf-8"
        )
        (args.output_dir / "summary.md").write_text(render_markdown(report), encoding="utf-8")
        print(f"Scheduled regression check: {report['overall_status']}")
        return 0
    except Exception as error:
        print(f"Scheduled regression check failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
