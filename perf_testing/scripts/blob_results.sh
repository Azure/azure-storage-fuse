#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "Usage: $0 <upload|download> <blobfuse2-binary> <local-file> <remote-path>" >&2
  exit 2
}

[[ $# -eq 4 ]] || usage

operation="$1"
binary="$2"
local_file="$3"
remote_path="$4"

[[ "$operation" == "upload" || "$operation" == "download" ]] || usage
[[ -x "$binary" ]] || { echo "Blobfuse2 binary is not executable: $binary" >&2; exit 1; }
[[ "$remote_path" != /* && "$remote_path" != *$'\n'* ]] || {
  echo "Remote result path must be a relative single-line path" >&2
  exit 1
}
IFS='/' read -ra remote_segments <<< "$remote_path"
for segment in "${remote_segments[@]}"; do
  [[ -n "$segment" && "$segment" != "." && "$segment" != ".." ]] || {
    echo "Remote result path contains an invalid segment" >&2
    exit 1
  }
done

: "${PERF_RESULTS_ACCOUNT:?PERF_RESULTS_ACCOUNT is required}"
: "${PERF_RESULTS_KEY:?PERF_RESULTS_KEY is required}"

if [[ "$operation" == "upload" ]]; then
  [[ -f "$local_file" ]] || { echo "Local result bundle does not exist: $local_file" >&2; exit 1; }
else
  mkdir -p "$(dirname "$local_file")"
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../.." && pwd)"
template="$repo_root/testdata/config/azure_results_storage.yaml"
[[ -f "$template" ]] || { echo "Results storage config template is missing" >&2; exit 1; }

transfer_root="$(mktemp -d)"
mount_dir="$transfer_root/mount"
cache_dir="$transfer_root/cache"
work_dir="$transfer_root/work"
config_file="$transfer_root/results-storage.yaml"
log_file="$transfer_root/blobfuse2-results-storage.log"
pid_name="${mount_dir//\//_}.pid"
pid_file="$work_dir/$pid_name"
mounted=false

cleanup() {
  status=$?
  trap - EXIT
  set +e
  if [[ "$mounted" == true ]] || mountpoint -q "$mount_dir"; then
    "$binary" unmount "$mount_dir" >/dev/null 2>&1 || \
      fusermount3 -u -z "$mount_dir" >/dev/null 2>&1 || true
  fi
  for _ in {1..300}; do
    [[ ! -e "$pid_file" ]] && break
    sleep 0.2
  done
  if mountpoint -q "$mount_dir" || [[ -e "$pid_file" ]]; then
    echo "Private results Blobfuse mount did not shut down cleanly" >&2
    [[ "$status" -ne 0 ]] || status=1
  fi
  rm -rf "$transfer_root"
  exit "$status"
}
trap cleanup EXIT

umask 077
mkdir -p "$mount_dir" "$cache_dir" "$work_dir"

export AZURE_STORAGE_ACCOUNT="$PERF_RESULTS_ACCOUNT"
export AZURE_STORAGE_ACCESS_KEY="$PERF_RESULTS_KEY"
"$binary" gen-test-config \
  --config-file="$template" \
  --container-name=results \
  --temp-path="$cache_dir" \
  --output-file="$config_file"
chmod 0600 "$config_file"

"$binary" mount "$mount_dir" \
  --config-file="$config_file" \
  --log-file-path="$log_file" \
  --default-working-dir="$work_dir" \
  --disable-version-check=true

for _ in {1..30}; do
  if mountpoint -q "$mount_dir"; then
    mounted=true
    break
  fi
  sleep 1
done
[[ "$mounted" == true ]] || { echo "Private results container did not mount" >&2; exit 1; }

remote_file="$mount_dir/$remote_path"
checksum_file="$remote_file.sha256"
if [[ "$operation" == "upload" ]]; then
  mkdir -p "$(dirname "$remote_file")"
  cp -- "$local_file" "$remote_file"
  sync "$remote_file"
  sha256sum "$local_file" | awk '{print $1}' > "$checksum_file"
  sync "$checksum_file"
else
  [[ -f "$remote_file" ]] || { echo "Requested private result bundle was not found: $remote_path" >&2; exit 1; }
  [[ -f "$checksum_file" ]] || { echo "Private result checksum was not found: $remote_path.sha256" >&2; exit 1; }
  cp -- "$remote_file" "$local_file"
fi

local_size="$(stat -c %s "$local_file")"
remote_size="$(stat -c %s "$remote_file")"
[[ "$local_size" == "$remote_size" ]] || {
  echo "Private result bundle size verification failed" >&2
  exit 1
}

expected_sha="$(tr -d '[:space:]' < "$checksum_file")"
actual_sha="$(sha256sum "$local_file" | awk '{print $1}')"
[[ "$expected_sha" =~ ^[0-9a-f]{64}$ && "$actual_sha" == "$expected_sha" ]] || {
  echo "Private result bundle checksum verification failed" >&2
  exit 1
}