#!/bin/bash
# Compares the libfuse callback benchmarks (component/libfuse/libfuse_bench_test.go) of the current checkout with a
# base revision, on this machine, and fails if a benchmark got slower by more than a threshold.
#
# Usage: test/scripts/libfuse_bench_compare.sh <base-ref>
#
# Environment:
#   ROUNDS              rounds to run, each runs both revisions (default 5)
#   COUNT               runs of every benchmark per revision and round (default 2), so ROUNDS*COUNT samples
#   THRESHOLD           regression in percent that fails the comparison (default 10)
#   FAIL_ON_REGRESSION  "false" to only report a regression (default true)
#   OUT_DIR             where to put the results (default: a temporary directory)
#
# Both revisions are measured with the benchmark code of the current checkout, which is copied into the base, so that
# only the code under test differs. Runs alternate between the revisions so that drift of the machine affects both,
# and benchstat only reports a difference that is statistically significant.

set -euo pipefail

base_ref="${1:?usage: $0 <base-ref>}"
rounds="${ROUNDS:-5}"
count="${COUNT:-2}"
threshold="${THRESHOLD:-10}"
fail_on_regression="${FAIL_ON_REGRESSION:-true}"
out_dir="${OUT_DIR:-$(mktemp -d)}"
bench_files=(component/libfuse/libfuse_bench_wrapper.go component/libfuse/libfuse_bench_test.go)

repo="$(git rev-parse --show-toplevel)"
base_tree="$(mktemp -d)/base"
mkdir -p "$out_dir"
# Both revisions are built from their own directory, so a relative OUT_DIR has to be made absolute
out_dir="$(cd "$out_dir" && pwd)"
export CGO_ENABLED=1 GOTOOLCHAIN=local

cleanup() {
    git -C "$repo" worktree remove --force "$base_tree" >/dev/null 2>&1 || true
}
trap cleanup EXIT

if ! command -v benchstat >/dev/null; then
    echo "benchstat not found, install it with: go install golang.org/x/perf/cmd/benchstat@latest" >&2
    exit 1
fi

echo "Building candidate ($(git -C "$repo" rev-parse --short HEAD)) and base ($base_ref)"
git -C "$repo" worktree add --quiet --detach "$base_tree" "$base_ref"
for f in "${bench_files[@]}"; do
    cp "$repo/$f" "$base_tree/$f"
done
(cd "$repo" && go test -c -tags unittest,fuse3 -o "$out_dir/candidate.test" ./component/libfuse/)
(cd "$base_tree" && go test -c -tags unittest,fuse3 -o "$out_dir/base.test" ./component/libfuse/)

rm -f "$out_dir/base.txt" "$out_dir/candidate.txt"
for round in $(seq 1 "$rounds"); do
    order="base candidate"
    if (( round % 2 == 0 )); then
        order="candidate base"
    fi
    for rev in $order; do
        echo "Round $round/$rounds: $rev"
        "$out_dir/$rev.test" -test.run '^$' -test.bench '^BenchmarkCallback$' -test.count "$count" >> "$out_dir/$rev.txt"
    done
done

benchstat base="$out_dir/base.txt" candidate="$out_dir/candidate.txt" | tee "$out_dir/benchstat.txt"
benchstat -format csv base="$out_dir/base.txt" candidate="$out_dir/candidate.txt" > "$out_dir/benchstat.csv"

# benchstat reports a change in percent only if it is significant, "~" otherwise
regressions="$(python3 - "$out_dir/benchstat.csv" "$threshold" <<'EOF'
import csv, sys
path, threshold = sys.argv[1], float(sys.argv[2])
in_table = False
for row in csv.reader(open(path)):
    if len(row) > 1 and row[1] == "sec/op":
        in_table = True
        continue
    if not in_table:
        continue
    if not row or not row[0]:
        break
    name, change = row[0], row[5]
    if name != "geomean" and change.endswith("%") and float(change.rstrip("%")) > threshold:
        print(f"{name}: {change} ({row[6]})")
EOF
)"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    {
        echo "### libfuse callback benchmarks: base \`$base_ref\` vs this change"
        echo
        echo "Fails on a significant slowdown of more than ${threshold}%."
        echo
        echo '```'
        cat "$out_dir/benchstat.txt"
        echo '```'
        if [[ -n "$regressions" ]]; then
            echo
            echo "**Regressions over ${threshold}%:**"
            echo
            echo '```'
            echo "$regressions"
            echo '```'
        fi
    } >> "$GITHUB_STEP_SUMMARY"
fi

if [[ -z "$regressions" ]]; then
    echo "No benchmark got slower by more than ${threshold}%"
    exit 0
fi

echo "Benchmarks that got slower by more than ${threshold}%:"
echo "$regressions"
if [[ "$fail_on_regression" == "true" ]]; then
    exit 1
fi
echo "FAIL_ON_REGRESSION=$fail_on_regression, not failing"
