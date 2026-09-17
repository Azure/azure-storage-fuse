#!/usr/bin/env python3

import argparse
import csv
from pathlib import Path


MIB = 1024 * 1024


def rate_mib(bytes_per_second: float) -> str:
    return f"{bytes_per_second / MIB:.2f}"


def split_case_name(name):
    configuration, separator, test_case = name.partition(": ")
    if not separator:
        return "-", name
    return configuration, test_case


def print_log_report(title, rows, max_ingress, max_egress):
    print(f"\n{title}")
    print(f"Recorded test cases   : {len(rows)}")
    print(f"Peak network ingress: {rate_mib(max_ingress)} MiB/s")
    print(f"Peak network egress : {rate_mib(max_egress)} MiB/s\n")

    if not rows:
        print("No metrics were recorded.")
        return

    for index, row in enumerate(rows, start=1):
        print(f"[{index:03d}] Configuration: {row['configuration']}")
        print(f"      Test case    : {row['test_case']}")
        print(
            "      I/O          : {operation} | Data: {data:.2f} MiB".format(
                operation=row["operation"],
                data=row["bytes"] / MIB,
            )
        )
        print(
            "      Performance  : {elapsed:.3f} s | {throughput} MiB/s".format(
                elapsed=row["elapsed"],
                throughput=rate_mib(row["throughput"]),
            )
        )
        print(
            "      Peak network : ingress {ingress} MiB/s | egress {egress} MiB/s".format(
                ingress=rate_mib(row["max_rx"]),
                egress=rate_mib(row["max_tx"]),
            )
        )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("metrics_file", type=Path)
    parser.add_argument("summary_file", type=Path)
    parser.add_argument("--title", default="Block Cache I/O Metrics")
    args = parser.parse_args()

    rows = []
    if args.metrics_file.exists():
        with args.metrics_file.open(newline="", encoding="utf-8") as stream:
            reader = csv.reader(stream, delimiter="\t")
            for row in reader:
                if len(row) != 6:
                    continue
                name, operation, byte_count, elapsed_ns, max_rx, max_tx = row
                configuration, test_case = split_case_name(name)
                elapsed_seconds = max(int(elapsed_ns) / 1_000_000_000, 1e-9)
                rows.append(
                    {
                        "name": name,
                        "configuration": configuration,
                        "test_case": test_case,
                        "operation": operation,
                        "bytes": int(byte_count),
                        "elapsed": elapsed_seconds,
                        "throughput": int(byte_count) / elapsed_seconds,
                        "max_rx": int(max_rx),
                        "max_tx": int(max_tx),
                    }
                )

    max_ingress = max((row["max_rx"] for row in rows), default=0)
    max_egress = max((row["max_tx"] for row in rows), default=0)

    lines = [
        f"# {args.title}",
        "",
        f"**Peak network ingress:** {rate_mib(max_ingress)} MiB/s  ",
        f"**Peak network egress:** {rate_mib(max_egress)} MiB/s",
        "",
        "Network peaks are sampled from the agent's default-route interface and may include unrelated host traffic. ",
        "For `read+write` cases, throughput is logical payload processed per second (payload is counted once).",
        "",
        "| Configuration | Test case | I/O | Data (MiB) | Duration (s) | Throughput (MiB/s) | Peak ingress (MiB/s) | Peak egress (MiB/s) |",
        "|---|---|---:|---:|---:|---:|---:|---:|",
    ]

    for row in rows:
        lines.append(
            "| {configuration} | {test_case} | {operation} | {data:.2f} | {elapsed:.3f} | {throughput} | {ingress} | {egress} |".format(
                configuration=row["configuration"].replace("|", "\\|"),
                test_case=row["test_case"].replace("|", "\\|"),
                operation=row["operation"],
                data=row["bytes"] / MIB,
                elapsed=row["elapsed"],
                throughput=rate_mib(row["throughput"]),
                ingress=rate_mib(row["max_rx"]),
                egress=rate_mib(row["max_tx"]),
            )
        )

    if not rows:
        lines.append("| - | No metrics were recorded | - | - | - | - | - | - |")

    args.summary_file.parent.mkdir(parents=True, exist_ok=True)
    args.summary_file.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print_log_report(args.title, rows, max_ingress, max_egress)


if __name__ == "__main__":
    main()