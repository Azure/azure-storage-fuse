#!/usr/bin/env bash
#
# run_tests.sh
# ---------------------------------------------------------------------------
# Runs the OpenClaw skill test suites.
#
# Options:
#   --sanity    Run the sanity test suite (task_sanity).
#               Default (when omitted) is the automated-only suite.
#   --verbose   Enable verbose logging.
#
# Usage:
#   ./run_tests.sh                     # automated-only suite, quiet
#   ./run_tests.sh --sanity            # sanity suite
#   ./run_tests.sh --verbose           # automated-only suite, verbose
#   ./run_tests.sh --sanity --verbose  # sanity suite, verbose
# ---------------------------------------------------------------------------

# Fail fast and loudly:
#   -e : exit on any command error
#   -u : treat unset variables as errors
#   -o pipefail : a pipeline fails if any component fails
set -euo pipefail

MODEL="github-copilot/gpt-5.4-mini"
OUTPUT_DIR="output"

# Defaults: automated-only suite, non-verbose.
SUITE="automated-only"
VERBOSE=false

# ---------------------------------------------------------------------------
# Parse command-line arguments
# ---------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --sanity)
            SUITE="task_sanity"
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        *)
            echo "Unknown option: $1" >&2
            echo "Usage: $0 [--sanity] [--verbose]" >&2
            exit 1
            ;;
    esac
done

# ---------------------------------------------------------------------------
# Ensure the skill repo is present (clone if missing)
# ---------------------------------------------------------------------------
SKILL_DIR="/home/sourav/skill"
SKILL_REPO="https://github.com/pinchbench/skill"

if [[ ! -d "${SKILL_DIR}/.git" ]]; then
    echo "Skill repo not found at ${SKILL_DIR}; cloning from ${SKILL_REPO}..."
    git clone "${SKILL_REPO}" "${SKILL_DIR}"
else
    echo "Skill repo already present at ${SKILL_DIR}."
fi

# ---------------------------------------------------------------------------
# Build the argument list and run
# ---------------------------------------------------------------------------
run_args=(
    --model "${MODEL}"
    --suite "${SUITE}"
    --no-upload
    --output-dir "${OUTPUT_DIR}"
)

# Only add --verbose when requested.
if [[ "${VERBOSE}" == true ]]; then
    run_args+=(--verbose)
fi

echo "Running '${SUITE}' suite (verbose=${VERBOSE})..."
echo "+ /home/sourav/skill/scripts/run.sh ${run_args[*]}"
/home/sourav/skill/scripts/run.sh "${run_args[@]}"
