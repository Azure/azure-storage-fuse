#!/usr/bin/env python3

import argparse
import html
import json
import math
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


METRIC_LABELS = {
    "throughput_mib_s": ("End-to-end throughput", "MiB/s"),
    "end_to_end_throughput_mib_s": ("End-to-end throughput", "MiB/s"),
    "iops": ("IOPS", "IOPS"),
    "operations_per_second": ("Operation rate", "ops/s"),
    "latency_p99_ms": ("p99 latency", "ms"),
    "sync_latency_p99_ms": ("p99 sync latency", "ms"),
}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def load_summary(path: Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as summary_file:
        summary = json.load(summary_file)
    if summary.get("schema_version") != 1:
        raise ValueError(f"Unsupported benchmark result schema in {path}")
    return summary


def safe_ratio(value: float, reference: float) -> float:
    if reference == 0:
        return 100.0 if value else 0.0
    return ((value / reference) - 1) * 100


def noise_percent(metric: dict[str, float]) -> float:
    median = abs(float(metric["median"]))
    if median == 0:
        return 0.0
    return abs(float(metric.get("mad", 0))) / median * 100


def compare_metric(
    baseline: dict[str, float],
    candidate: dict[str, float],
    *,
    higher_is_better: bool,
    threshold: float,
) -> dict[str, Any]:
    baseline_value = float(baseline["median"])
    candidate_value = float(candidate["median"])
    delta = safe_ratio(candidate_value, baseline_value)
    observed_noise = max(noise_percent(baseline), noise_percent(candidate))
    effective_threshold = max(threshold, observed_noise * 3)

    signed_change = delta if higher_is_better else -delta
    if signed_change <= -effective_threshold:
        status = "regression"
    elif signed_change >= effective_threshold:
        status = "improvement"
    elif observed_noise > threshold:
        status = "noisy"
    else:
        status = "stable"

    return {
        "baseline": baseline_value,
        "candidate": candidate_value,
        "delta_percent": delta,
        "noise_percent": observed_noise,
        "effective_threshold_percent": effective_threshold,
        "higher_is_better": higher_is_better,
        "status": status,
    }


def compare_summaries(
    baseline: dict[str, Any],
    candidate: dict[str, Any],
    primary_threshold: float,
    latency_threshold: float,
) -> dict[str, Any]:
    baseline_suite = str(baseline["run"]["suite"])
    candidate_suite = str(candidate["run"]["suite"])
    if baseline_suite != candidate_suite:
        raise ValueError(
            f"Cannot compare baseline suite {baseline_suite!r} with "
            f"candidate suite {candidate_suite!r}"
        )
    baseline_benchmarks = {item["id"]: item for item in baseline["benchmarks"]}
    candidate_benchmarks = {item["id"]: item for item in candidate["benchmarks"]}
    shared_ids = sorted(set(baseline_benchmarks) & set(candidate_benchmarks))
    if not shared_ids:
        raise ValueError("Baseline and candidate have no benchmarks in common")

    results = []
    suite = baseline_suite
    for benchmark_id in shared_ids:
        baseline_benchmark = baseline_benchmarks[benchmark_id]
        candidate_benchmark = candidate_benchmarks[benchmark_id]
        primary_metric = baseline_benchmark["primary_metric"]
        metrics = {}
        metrics[primary_metric] = compare_metric(
            baseline_benchmark["metrics"][primary_metric],
            candidate_benchmark["metrics"][primary_metric],
            higher_is_better=bool(baseline_benchmark["higher_is_better"]),
            threshold=primary_threshold,
        )

        for latency_metric in ("latency_p99_ms", "sync_latency_p99_ms"):
            if (
                latency_metric in baseline_benchmark["metrics"]
                and latency_metric in candidate_benchmark["metrics"]
            ):
                metrics[latency_metric] = compare_metric(
                    baseline_benchmark["metrics"][latency_metric],
                    candidate_benchmark["metrics"][latency_metric],
                    higher_is_better=False,
                    threshold=latency_threshold,
                )

        statuses = {metric["status"] for metric in metrics.values()}
        if "regression" in statuses:
            status = "regression"
        elif "noisy" in statuses:
            status = "noisy"
        elif "improvement" in statuses:
            status = "improvement"
        else:
            status = "stable"
        results.append(
            {
                "id": benchmark_id,
                "suite": suite,
                "name": baseline_benchmark["name"],
                "primary_metric": primary_metric,
                "status": status,
                "parameters": baseline_benchmark.get("parameters", {}),
                "metrics": metrics,
            }
        )

    overall = "regression" if any(item["status"] == "regression" for item in results) else "pass"
    return {
        "schema_version": 1,
        "generated_at": utc_now(),
        "overall_status": overall,
        "suites": [suite],
        "baseline": baseline["run"],
        "candidate": candidate["run"],
        "environment": {
            "baseline": baseline.get("environment", {}),
            "candidate": candidate.get("environment", {}),
        },
        "thresholds": {
            "primary_percent": primary_threshold,
            "latency_percent": latency_threshold,
            "noise_multiplier": 3,
        },
        "benchmarks": results,
    }


def revision_key(run: dict[str, Any]) -> tuple[str, ...]:
    return tuple(
        str(run.get(field, "unknown"))
        for field in (
            "id",
            "label",
            "ref",
            "commit",
            "architecture",
            "account_type",
            "cache_mode",
            "compute_profile",
            "trials",
        )
    )


def combine_comparisons(comparisons: list[dict[str, Any]]) -> dict[str, Any]:
    if not comparisons:
        raise ValueError("No benchmark comparisons were provided")

    first = comparisons[0]
    baseline_key = revision_key(first["baseline"])
    candidate_key = revision_key(first["candidate"])
    suites = []
    benchmarks = []
    seen_suites = set()
    seen_benchmarks = set()
    for comparison in comparisons:
        if revision_key(comparison["baseline"]) != baseline_key:
            raise ValueError("Baseline suite summaries describe different revisions or dimensions")
        if revision_key(comparison["candidate"]) != candidate_key:
            raise ValueError("Candidate suite summaries describe different revisions or dimensions")
        if comparison["thresholds"] != first["thresholds"]:
            raise ValueError("Suite comparisons use different thresholds")

        suite = str(comparison["baseline"]["suite"])
        if suite in seen_suites:
            raise ValueError(f"Duplicate suite comparison: {suite}")
        seen_suites.add(suite)
        suites.append(suite)
        for benchmark in comparison["benchmarks"]:
            benchmark_key = (suite, benchmark["id"])
            if benchmark_key in seen_benchmarks:
                raise ValueError(f"Duplicate benchmark comparison: {suite}/{benchmark['id']}")
            seen_benchmarks.add(benchmark_key)
            benchmarks.append({**benchmark, "suite": suite})

    baseline = {**first["baseline"], "suite": "+".join(suites)}
    candidate = {**first["candidate"], "suite": "+".join(suites)}
    return {
        "schema_version": 1,
        "generated_at": utc_now(),
        "overall_status": (
            "regression"
            if any(item["status"] == "regression" for item in benchmarks)
            else "pass"
        ),
        "suites": suites,
        "baseline": baseline,
        "candidate": candidate,
        "environment": first["environment"],
        "thresholds": first["thresholds"],
        "benchmarks": benchmarks,
    }


def format_value(value: float, metric_name: str) -> str:
    _, unit = METRIC_LABELS.get(metric_name, (metric_name, ""))
    if abs(value) >= 1000:
        rendered = f"{value:,.0f}"
    elif abs(value) >= 100:
        rendered = f"{value:,.1f}"
    else:
        rendered = f"{value:,.2f}"
    return f"{rendered} {unit}".strip()


def comparison_rows(comparison: dict[str, Any]) -> str:
    rows = []
    for benchmark in comparison["benchmarks"]:
        primary_name = benchmark["primary_metric"]
        primary = benchmark["metrics"][primary_name]
        delta = primary["delta_percent"]
        width = min(abs(delta), 50) * 2
        direction_class = "positive" if delta >= 0 else "negative"
        parameter_text = " / ".join(str(value) for value in benchmark["parameters"].values())
        secondary_signals = "".join(
            f"<small>{html.escape(METRIC_LABELS.get(name, (name, ''))[0])}: "
            f"{metric['delta_percent']:+.1f}% · {metric['status']}</small>"
            for name, metric in benchmark["metrics"].items()
            if name != primary_name
        )
        rows.append(
            f"""
            <tr>
              <td><span class="suite">{html.escape(str(benchmark.get('suite', 'unknown')))}</span></td>
              <td><strong>{html.escape(benchmark['name'])}</strong><small>{html.escape(parameter_text)}</small></td>
              <td>{html.escape(format_value(primary['baseline'], primary_name))}</td>
              <td>{html.escape(format_value(primary['candidate'], primary_name))}</td>
              <td>
                <span class="delta {direction_class}">{delta:+.1f}%</span>
                <span class="delta-bar"><span class="{direction_class}" style="width:{width:.1f}%"></span></span>
                {secondary_signals}
              </td>
              <td><span class="status {benchmark['status']}">{benchmark['status']}</span></td>
            </tr>
            """
        )
    return "".join(rows)


def render_html(comparison: dict[str, Any]) -> str:
    baseline = comparison["baseline"]
    candidate = comparison["candidate"]
    regression_count = sum(item["status"] == "regression" for item in comparison["benchmarks"])
    return f"""<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Blobfuse2 performance comparison</title>
  <style>
    @import url('https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@500&family=IBM+Plex+Sans:wght@400;500;600&display=swap');
    :root {{ --ink:#17202a; --muted:#65717e; --line:#d8dee5; --paper:#f7f9fb; --panel:#fff; --blue:#0067b8; --green:#177245; --red:#b42318; --amber:#9a6700; }}
    * {{ box-sizing:border-box; }}
    body {{ margin:0; color:var(--ink); background:linear-gradient(180deg,#eef5f9 0,#f7f9fb 260px); font-family:'IBM Plex Sans',sans-serif; letter-spacing:0; }}
    main {{ width:min(1180px,calc(100% - 32px)); margin:0 auto; padding:42px 0 64px; }}
    header {{ display:grid; grid-template-columns:1fr auto; gap:24px; align-items:end; border-bottom:1px solid var(--line); padding-bottom:28px; }}
    h1 {{ margin:0 0 8px; font-size:clamp(28px,4vw,46px); line-height:1.05; font-weight:600; }}
    p {{ margin:0; color:var(--muted); }}
    .verdict {{ border-left:5px solid var(--green); padding:12px 18px; background:var(--panel); border-radius:4px; min-width:190px; }}
    .verdict.regression {{ border-color:var(--red); }}
    .verdict strong {{ display:block; font-size:24px; }}
    .refs {{ display:grid; grid-template-columns:1fr 1fr; gap:16px; margin:24px 0; }}
    .ref {{ background:var(--panel); border:1px solid var(--line); border-radius:6px; padding:18px; min-width:0; }}
    .ref span {{ display:block; color:var(--muted); font-size:13px; text-transform:uppercase; }}
    .ref code {{ display:block; overflow:hidden; text-overflow:ellipsis; margin-top:6px; font-family:'IBM Plex Mono',monospace; }}
    .table-wrap {{ overflow-x:auto; background:var(--panel); border:1px solid var(--line); border-radius:6px; }}
    table {{ width:100%; border-collapse:collapse; min-width:820px; }}
    th,td {{ padding:15px 16px; text-align:left; border-bottom:1px solid var(--line); }}
    th {{ color:var(--muted); font-size:12px; text-transform:uppercase; font-weight:600; background:#f4f7f9; }}
    td small {{ display:block; color:var(--muted); margin-top:4px; }}
    tr:last-child td {{ border-bottom:0; }}
    .delta {{ font-family:'IBM Plex Mono',monospace; font-weight:500; }}
    .delta.positive {{ color:var(--green); }} .delta.negative {{ color:var(--red); }}
    .delta-bar {{ display:block; height:4px; width:100px; margin-top:7px; background:#e8edf1; }}
    .delta-bar span {{ display:block; height:100%; }} .delta-bar .positive {{ background:var(--green); }} .delta-bar .negative {{ background:var(--red); }}
    .status {{ display:inline-block; padding:4px 8px; border-radius:3px; font-size:12px; font-weight:600; text-transform:uppercase; }}
    .suite {{ display:inline-block; color:var(--blue); font-family:'IBM Plex Mono',monospace; font-size:12px; text-transform:uppercase; }}
    .status.stable {{ color:var(--blue); background:#e6f2fa; }} .status.improvement {{ color:var(--green); background:#e7f4ec; }}
    .status.regression {{ color:var(--red); background:#feeceb; }} .status.noisy {{ color:var(--amber); background:#fff4ce; }}
    footer {{ margin-top:18px; color:var(--muted); font-size:13px; }}
    @media (max-width:700px) {{ header,.refs {{ grid-template-columns:1fr; }} .verdict {{ min-width:0; }} main {{ width:min(100% - 20px,1180px); padding-top:26px; }} }}
  </style>
</head>
<body>
<main>
  <header>
    <div><h1>Performance comparison</h1><p>Blobfuse2 candidate versus its main-branch baseline on the same benchmark host.</p></div>
    <div class="verdict {'regression' if comparison['overall_status'] == 'regression' else ''}"><span>Result</span><strong>{html.escape(comparison['overall_status'].upper())}</strong><p>{regression_count} regressions</p></div>
  </header>
  <section class="refs">
    <div class="ref"><span>Baseline</span><code>{html.escape(baseline['ref'])} · {html.escape(baseline['commit'][:12])}</code></div>
    <div class="ref"><span>Candidate</span><code>{html.escape(candidate['ref'])} · {html.escape(candidate['commit'][:12])}</code></div>
  </section>
  <div class="table-wrap"><table>
        <thead><tr><th>Suite</th><th>Workload</th><th>Baseline</th><th>Candidate</th><th>Change</th><th>Assessment</th></tr></thead>
    <tbody>{comparison_rows(comparison)}</tbody>
  </table></div>
  <footer>Thresholds expand to three times observed median absolute deviation when trial noise exceeds the configured limit. Review raw FIO artifacts for any noisy result.</footer>
</main>
</body>
</html>
"""


def render_markdown(comparison: dict[str, Any]) -> str:
    chart_labels = [
        f"{benchmark.get('suite', 'unknown')}: {benchmark['name']}".replace('"', "'")[:28]
        for benchmark in comparison["benchmarks"]
    ]
    candidate_percentages = [
        (
            round(
                benchmark["metrics"][benchmark["primary_metric"]]["candidate"]
                / benchmark["metrics"][benchmark["primary_metric"]]["baseline"]
                * 100,
                1,
            )
            if benchmark["metrics"][benchmark["primary_metric"]]["baseline"]
            else (
                100.0
                if not benchmark["metrics"][benchmark["primary_metric"]]["candidate"]
                else 200.0
            )
        )
        for benchmark in comparison["benchmarks"]
    ]
    chart_maximum = max(130, int(math.ceil(max(candidate_percentages, default=100) / 10) * 10))
    lines = [
        "## Blobfuse2 performance comparison",
        "",
        f"**Result:** {comparison['overall_status'].upper()}",
        "",
        "```mermaid",
        "xychart-beta",
        '    title "Primary metrics relative to baseline"',
        f"    x-axis [{', '.join(json.dumps(label) for label in chart_labels)}]",
        f'    y-axis "Percent" 0 --> {chart_maximum}',
        f"    line [{', '.join('100' for _ in chart_labels)}]",
        f"    bar [{', '.join(str(value) for value in candidate_percentages)}]",
        "```",
        "",
        "| Suite | Workload | Baseline | Candidate | Change | Status |",
        "| --- | --- | ---: | ---: | ---: | --- |",
    ]
    for benchmark in comparison["benchmarks"]:
        metric_name = benchmark["primary_metric"]
        metric = benchmark["metrics"][metric_name]
        secondary = "; ".join(
            f"{METRIC_LABELS.get(name, (name, ''))[0]} {value['delta_percent']:+.1f}% ({value['status']})"
            for name, value in benchmark["metrics"].items()
            if name != metric_name
        )
        status = benchmark["status"] if not secondary else f"{benchmark['status']}; {secondary}"
        lines.append(
            f"| {benchmark.get('suite', 'unknown')} | {benchmark['name']} | "
            f"{format_value(metric['baseline'], metric_name)} | "
            f"{format_value(metric['candidate'], metric_name)} | {metric['delta_percent']:+.1f}% | "
            f"{status} |"
        )
    lines.extend(
        [
            "",
            f"Baseline: `{comparison['baseline']['ref']}` at `{comparison['baseline']['commit'][:12]}`  ",
            f"Candidate: `{comparison['candidate']['ref']}` at `{comparison['candidate']['commit'][:12]}`",
            "",
        ]
    )
    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Compare Blobfuse2 benchmark summary pairs")
    parser.add_argument("--baseline", type=Path, action="append", required=True)
    parser.add_argument("--candidate", type=Path, action="append", required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--primary-threshold", type=float, default=10.0)
    parser.add_argument("--latency-threshold", type=float, default=20.0)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if len(args.baseline) != len(args.candidate):
            raise ValueError("Each --baseline summary requires one --candidate summary")
        comparisons = [
            compare_summaries(
                load_summary(baseline),
                load_summary(candidate),
                args.primary_threshold,
                args.latency_threshold,
            )
            for baseline, candidate in zip(args.baseline, args.candidate)
        ]
        comparison = combine_comparisons(comparisons)
        args.output_dir.mkdir(parents=True, exist_ok=True)
        (args.output_dir / "comparison.json").write_text(
            json.dumps(comparison, indent=2) + "\n", encoding="utf-8"
        )
        (args.output_dir / "comparison.html").write_text(render_html(comparison), encoding="utf-8")
        (args.output_dir / "summary.md").write_text(render_markdown(comparison), encoding="utf-8")
        print(f"Comparison result: {comparison['overall_status']}")
        return 0
    except Exception as error:
        print(f"Comparison failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
