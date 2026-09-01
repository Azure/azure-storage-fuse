# Blobfuse2 performance testing

This directory contains the performance system used for two distinct purposes:

1. Publish a small, stable set of block-cache throughput numbers for Blobfuse2 users.
2. Detect performance regressions across filesystem and cache code paths for maintainers.

The two result sets intentionally have different workloads, retention, and presentation.

## Scheduled runs

The [Blobfuse2 Performance workflow](../.github/workflows/benchmark.yml) serializes all jobs that share the benchmark storage accounts.

| Schedule | Suites | Destination |
| --- | --- | --- |
| Monday-Saturday, 04:00 UTC | Developer regression | `/developer/` dashboard |
| Sunday, 04:00 UTC | Developer regression and public sustained throughput | Public and developer dashboards |

The workflow acquires one X86 runner and one ARM64 runner instead of provisioning a VM for every account/cache profile. Each host is validated, prepared, and built once. The X86 host runs eight profiles sequentially; the ARM64 host runs the two supported block-cache profiles. The architecture jobs are serialized with `max-parallel: 1` to avoid concurrent load on shared benchmark accounts. Profile failures are collected so later profiles still produce diagnostics, but the architecture job fails after all selected profiles finish.

Results are published only for a complete successful `main` run. The two architecture jobs upload raw FIO JSON as `perf-X86` and `perf-ARM64`, then one publisher job updates both histories in a single commit to the `benchmarks` branch. A failed profile or architecture job cannot leave a partial public run.

On initial rollout, a developer-only weekday run adds `/developer/` but preserves the legacy public site. The first successful Sunday public run performs the curated public cutover and removes old generated performance directories while preserving `release/` and issue-metric data. To cut over immediately after merge, manually run the `all` suite on `main` with `publish` enabled.

## Migrate D96 runner pools

The workflows currently use these benchmark profiles while the Gen2 X86 image is prepared:

| Architecture | Azure VM SKU | vCPUs | Memory | Local temporary storage | Expected network ceiling |
| --- | --- | ---: | ---: | --- | ---: |
| X86 | `Standard_D96ds_v5` | 96 | 384 GiB | 1 x 3,600 GiB temporary disk | 35,000 Mbps |
| ARM64 | `Standard_D96pds_v6` | 96 | 384 GiB | 6 x 880 GiB NVMe | 60,000 Mbps |

The planned X86 target remains `Standard_D192ds_v6`: 192 vCPUs, 768 GiB memory, six 1,760 GiB temporary NVMe devices, and up to 82,000 Mbps networking. Switch the workflow SKU and CPU guards only after the new generalized Gen2 image provisions successfully in `1ES.Pool=blobfuse2-benchmark`.

Azure Cobalt ARM `Dpdsv6` and `Dpldsv6` currently stop at 96 vCPUs. There is no `Standard_D192pds_v6`, so “D192 for both architectures” is not a supported one-to-one migration. Keep ARM on D96 for architecture coverage. Running two ARM D96 VMs would be a distributed benchmark with different semantics and must use separate workload IDs and history.

Migrate X86 without resizing the only active runner in place:

1. Check regional `Standard_D192ds_v6` availability and quota for 192 vCPUs, six local NVMe devices, and MANA/accelerated networking.
2. Create a new runner from the same OS image family and keep kernel, FUSE, FIO 3.36, Microsoft Go, Blobfuse build flags, sysctls, and network settings aligned with the old runner.
3. Build RAID0 from the six temporary NVMe devices, format it, mount it under `/mnt` (preferably at `/mnt/localssd`), and make the benchmark user its owner. This storage is ephemeral and must be recreated after host replacement. The workflow accepts a cache directory beneath a dedicated non-root filesystem and requires at least 200 GiB free.
4. Register the new VM in `1ES.Pool=blobfuse2-benchmark` without removing the D96 runner yet. Ensure benchmark scheduling selects only the D192 runner; do not leave mixed D96/D192 hosts behind the same label.
5. Verify `nproc` returns 192 and IMDS returns exactly `Standard_D192ds_v6`. The workflow enforces both. The ARM pool remains `1ES.Pool=blobfuse2-benchmark-arm` and is enforced as `Standard_D96pds_v6` with 96 CPUs.
6. Run public and regression workflows with `publish=false`, inspect the private ZIPs, local-disk capacity, network ratios, and MAD, then run the D192 concurrency sweep described below.
7. Publish only after no-publish runs pass. Keep the D96 runner available for rollback until at least three matching D192 runs establish the rolling baseline.

Reuse the same Azure accounts, containers, regions, and benchmark configuration during the migration so the VM is the only intentional variable. Existing correctly sized `v4` read fixtures can be reused by D192. Clear local caches before every run as usual.

The dashboards retain D96 and D192 as separate compute profiles. Regression checks also require the same VM SKU, Azure region, configuration hash, and trial count; initial D192 or five-trial results show `insufficient-baseline` rather than comparing against a different profile or three-trial history.

## Run on a standalone benchmark VM

Use a dedicated VM, storage account, and container. Keep the VM and storage account in the same Azure region, stop other workloads, and do not run two benchmarks against the same account/container concurrently.

Confirm the host before running:

```bash
nproc
lscpu | grep -E 'Architecture|CPU\(s\)|Model name|NUMA'
curl -fsS -H Metadata:true \
	'http://169.254.169.254/metadata/instance/compute?api-version=2021-02-01' | \
	jq '{vmSize, location}'
df -h
test -c /dev/fuse
sudo -v
sudo -n true
```

The temporary X86 `Standard_D96ds_v5` profile must report 96 online CPUs. Scheduled X86 and branch-comparison jobs fail before setup when the SKU or `nproc` does not match. Change both guards to `Standard_D192ds_v6` and 192 CPUs as part of the eventual pool migration, never before it. The ARM profile is checked separately as `Standard_D96pds_v6` with 96 online CPUs. The runner also records Azure VM SKU and region from Instance Metadata Service; set `AZURE_VM_SIZE` and `AZURE_REGION` only when IMDS is unavailable for a manual run.

The runner drops the kernel page cache before every mount using non-interactive `sudo`, so `sudo -n true` must succeed for the entire run. File-cache shutdown clears its dedicated local cache; the runner waits for the Blobfuse daemon PID file to disappear before remounting or touching that directory, preventing cleanup races between trials. File-cache regression tests should use a local SSD path with at least 200 GiB free. Public tests use block cache only and need little local scratch space, but their persistent Azure fixtures consume 640 GiB in the benchmark container.

For Actions file-cache jobs, the workflow discovers `/mnt/localssd`, `/mnt/azure_nvme_temp`, or `/mnt/resource` when the path is backed by a non-root filesystem with at least 200 GiB free. A subdirectory such as `/mnt/localssd` is valid when its backing mount is `/mnt`; the directory itself does not need to be a mountpoint. The workflow still rejects an OS-disk fallback. D192 local-disk count and device names are otherwise not assumed by the X86 workflow; provision and mount the desired local SSD/RAID in the runner image before benchmarking.

The scheduled ARM64 profiles use block cache only and do not use the VM's local NVMe array. The workflow leaves any image-provisioned ARM temporary-disk mount, such as `/mnt/azure_nvme_temp`, unchanged.

The block-cache benchmark template does not set `max-fuse-threads` or any `block_cache` options. Blobfuse therefore resolves its runtime defaults for block size, memory pool, prefetch, worker parallelism, prefetch-on-open, and FUSE background requests on each measured host. The template still fixes component selection, authentication mode, error logging, `ignore-open-flags=true`, and a 7,200-second attribute-cache timeout. The secret-redacted generated-config hash is a history boundary, so default-config results are not compared with the earlier explicitly tuned profile.

The public aggregate case is fixed at four FIO jobs and does not scale with VM CPU count. It is a representative comparison point, not a statement that four streams are required to saturate the network. Any future concurrency change must use new workload IDs and a new fixture version rather than mixing different application-concurrency contracts in one history series.

A cached `sudo -v` ticket may expire during fixture creation. Configure the dedicated benchmark user for the required non-interactive cache-drop operation according to your VM security policy; do not weaken sudo policy on a shared machine.

Install the pinned tools and build Blobfuse2:

```bash
sudo apt-get update
sudo apt-get install -y fuse3 libfuse3-dev gcc jq python3 bc libaio-dev \
	build-essential curl git iproute2
./tools/install_fio.sh
fio --version # must be fio-3.36
./build.sh
./blobfuse2 --version
ulimit -n 65536
```

Set credentials without putting the key directly in shell history:

```bash
read -r -p 'Storage account: ' AZURE_STORAGE_ACCOUNT
read -r -s -p 'Storage key: ' AZURE_STORAGE_ACCESS_KEY; echo
read -r -p 'Dedicated benchmark container: ' BENCH_CONTAINER
export AZURE_STORAGE_ACCOUNT AZURE_STORAGE_ACCESS_KEY BENCH_CONTAINER

ACCOUNT_TYPE=premium # or standard
RUN_ID="manual-$(date -u +%Y%m%dT%H%M%SZ)"
COMMIT="$(git rev-parse HEAD)"
REF="$(git symbolic-ref --short -q HEAD || git rev-parse --short HEAD)"
SCRATCH_ROOT=/mnt/localssd/blobfuse-benchmark # required dedicated blobfuse-* path
MOUNT_DIR="/mnt/blobfuse-benchmark-${RUN_ID}"
CACHE_DIR="${SCRATCH_ROOT}/cache"
RESULT_ROOT="${SCRATCH_ROOT}/results/${RUN_ID}"
CONFIG="${SCRATCH_ROOT}/block-cache.yaml"
sudo mkdir -p "$SCRATCH_ROOT"
sudo chown "$(id -u):$(id -g)" "$SCRATCH_ROOT"
mkdir -p "$CACHE_DIR" "$RESULT_ROOT"
```

The runner recursively clears `--cache-dir` between mounts. It rejects filesystem roots, shallow paths, paths without a dedicated `blobfuse-` component, and paths overlapping the mount, output, repository, current working directory, binary, or config. Never reuse a directory containing unrelated data.

Generate the normal-mount block-cache configuration. Mount-wide direct I/O must remain absent:

```bash
./blobfuse2 gen-test-config \
	--config-file=azure_block_benchmark.yaml \
	--container-name="$BENCH_CONTAINER" \
	--output-file="$CONFIG"
! grep -q 'direct-io:' "$CONFIG"
```

Before transferring data, append `--validate-only` to a runner command to check the binary, config, FIO jobs, network interface, and direct-I/O contract without mounting or performing Azure I/O. Use `--prepare-only` instead to create or validate persistent read fixtures without producing `summary.json`. A normal run automatically prepares missing fixtures, so both stages are optional.

If you invoke the public read FIO files directly instead of using `run_benchmark.py`, prepare their immutable files once in the same directory that will be passed to the read jobs:

```bash
FIXTURE_DIR="${MOUNT_DIR}/.blobfuse-benchmark-fixtures/v4"
mkdir -p "$FIXTURE_DIR"

fio --directory="$FIXTURE_DIR" \
	perf_testing/config/benchmark/setup/public_single_read.fio
fio --directory="$FIXTURE_DIR" \
	perf_testing/config/benchmark/setup/public_multi_read.fio

test "$(stat -c %s "$FIXTURE_DIR/public-single-read.data")" -eq $((320 * 1024 * 1024 * 1024))
test "$(find "$FIXTURE_DIR" -maxdepth 1 -type f -name 'public-multi-read.*' | wc -l)" -eq 4
test -z "$(find "$FIXTURE_DIR" -maxdepth 1 -type f -name 'public-multi-read.*' \
	! -size $((80 * 1024 * 1024 * 1024))c -print -quit)"
```

After fixture creation completes, unmount Blobfuse, clear kernel and local cache state, remount, and run the reads against that same directory:

```bash
./blobfuse2 unmount "$MOUNT_DIR"
sudo -n sh -c 'sync; echo 3 > /proc/sys/vm/drop_caches'
find "${CACHE_DIR:?}" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
./blobfuse2 mount "$MOUNT_DIR" --config-file="$CONFIG"

fio --directory="$FIXTURE_DIR" \
	perf_testing/config/benchmark/public/seq_read_single.fio
fio --directory="$FIXTURE_DIR" \
	perf_testing/config/benchmark/public/seq_read_multi.fio
```

The expected fixture names are `public-single-read.data` at 320 GiB and `public-multi-read.0` through `public-multi-read.3` at 80 GiB each. Do not remove `allow_file_create=0` from the read jobs and do not recreate fixtures inside the measured read command. The benchmark runner performs this preparation, validation, unmount, cache clearing, and remount sequence automatically.

Run a one-trial public smoke test first. Fixture creation is automatic; the first public run uploads 640 GiB of persistent read fixtures:

```bash
python3 perf_testing/scripts/run_benchmark.py \
	--binary ./blobfuse2 --config "$CONFIG" --suite public \
	--mount-dir "$MOUNT_DIR" --cache-dir "$CACHE_DIR" \
	--output-dir "$RESULT_ROOT/smoke-public" \
	--run-id "$RUN_ID" --label manual --commit "$COMMIT" --ref "$REF" \
	--architecture X86 --account-type "$ACCOUNT_TYPE" --cache-mode block_cache \
	--trials 1
```

The runner deletes each transient write trial immediately after collecting its metrics. Budget approximately 960 GiB peak container usage for 640 GiB of persistent fixtures plus one 320 GiB write trial, excluding developer fixtures, deletion lag, or old fixture versions. `--keep-data` disables this cleanup and requires substantially more capacity.

If the smoke test succeeds, run the publishable five-trial public suite by omitting `--trials`:

```bash
python3 perf_testing/scripts/run_benchmark.py \
	--binary ./blobfuse2 --config "$CONFIG" --suite public \
	--mount-dir "$MOUNT_DIR" --cache-dir "$CACHE_DIR" \
	--output-dir "$RESULT_ROOT/publish/public" \
	--run-id "$RUN_ID" --label manual --commit "$COMMIT" --ref "$REF" \
	--architecture X86 --account-type "$ACCOUNT_TYPE" --cache-mode block_cache
```

Run the developer regression suite with block cache in the same way:

```bash
python3 perf_testing/scripts/run_benchmark.py \
	--binary ./blobfuse2 --config "$CONFIG" --suite regression \
	--mount-dir "$MOUNT_DIR" --cache-dir "$CACHE_DIR" \
	--output-dir "$RESULT_ROOT/publish/regression-block" \
	--run-id "$RUN_ID" --label manual --commit "$COMMIT" --ref "$REF" \
	--architecture X86 --account-type "$ACCOUNT_TYPE" --cache-mode block_cache
```

To exercise developer file-cache coverage, create a separate dedicated cache directory and config. Never point `FILE_CACHE_DIR` at a directory containing other data:

```bash
FILE_CACHE_DIR="${SCRATCH_ROOT}/file-cache"
FILE_CONFIG="${SCRATCH_ROOT}/file-cache.yaml"
mkdir -p "$FILE_CACHE_DIR"
./blobfuse2 gen-test-config \
	--config-file=azure_file_benchmark.yaml \
	--container-name="$BENCH_CONTAINER" \
	--temp-path="$FILE_CACHE_DIR" \
	--output-file="$FILE_CONFIG"
! grep -q 'direct-io:' "$FILE_CONFIG"

python3 perf_testing/scripts/run_benchmark.py \
	--binary ./blobfuse2 --config "$FILE_CONFIG" --suite regression \
	--mount-dir "$MOUNT_DIR" --cache-dir "$FILE_CACHE_DIR" \
	--output-dir "$RESULT_ROOT/publish/regression-file" \
	--run-id "$RUN_ID" --label manual --commit "$COMMIT" --ref "$REF" \
	--architecture X86 --account-type "$ACCOUNT_TYPE" --cache-mode file_cache
```

Each output directory contains `blobfuse2.log`; successful runs also contain `summary.json` and a `raw/` tree with per-trial FIO JSON. A failed invocation writes `failure.json`, and Actions adds `diagnostics/host-state.txt`, `diagnostics/kernel.log`, and a secret-redacted mount config to the same profile directory. The complete architecture result tree is stored as a private ZIP in the separate results account. The original generated config, including its account key, is deleted and is never archived. Print the primary medians and MAD values with:

Private ZIP uploads and downloads mount the `results` container through Blobfuse file cache. Each transfer uses an isolated cache below a mode-0700 temporary directory, enables `sync-to-flush`, waits for Blobfuse to stop, and then removes the temporary cache and credential-bearing config.

```bash
jq -r '.benchmarks[] | . as $b | [
	$b.name,
	$b.primary_metric,
	($b.metrics[$b.primary_metric].median | tostring),
	($b.metrics[$b.primary_metric].mad | tostring)
] | @tsv' "$RESULT_ROOT/publish/public/summary.json" | column -t -s $'\t'
```

Render the same public and developer pages locally from only the full results. Use a new temporary site directory, not a checkout of the `benchmarks` branch:

```bash
SITE_DIR=/tmp/blobfuse-benchmark-site
rm -rf "$SITE_DIR"
python3 perf_testing/scripts/publish_benchmarks.py \
	--results-dir "$RESULT_ROOT/publish" \
	--site-dir "$SITE_DIR"
python3 -m http.server 8000 --directory "$SITE_DIR"
```

Open `http://localhost:8000/` for the public results and `http://localhost:8000/developer/` for the developer results. Use SSH or VS Code port forwarding instead of exposing port 8000 through the VM network security group.

## Transition the GitHub Pages site

Before merging, verify these repository settings:

- The X86 self-hosted runner label currently resolves only to `Standard_D96ds_v5` hosts exposing 96 online CPUs; after the Gen2 image migration, update this guard and the pool together to `Standard_D192ds_v6` with 192 CPUs. The ARM64 label resolves only to `Standard_D96pds_v6` hosts exposing 96 online CPUs.
- `STANDARD_ACCOUNT`, `STANDARD_KEY`, `PREMIUM_ACCOUNT`, `PREMIUM_KEY`, `STANDARD_HNS_ACCOUNT`, `STANDARD_HNS_KEY`, `PREMIUM_HNS_ACCOUNT`, `PREMIUM_HNS_KEY`, `BENCH_CONTAINER`, `PERF_RESULTS_ACCOUNT`, and `PERF_RESULTS_KEY` secrets are present.
- `PERF_RESULTS_ACCOUNT` and `PERF_RESULTS_KEY` identify a separate private account with a container named `results`. Disable anonymous access to this account and container.
- Every account has a dedicated container named by `BENCH_CONTAINER`, in the same region as its runner.
- The `performance-benchmark-compare` environment exists with required reviewers. Only trusted same-repository refs should be approved because candidate binaries receive benchmark credentials.
- GitHub Pages still serves the `benchmarks` branch from its root.

Create a rollback branch before the first publication:

```bash
git fetch origin benchmarks
BACKUP_BRANCH="benchmarks-backup-$(date -u +%Y%m%dT%H%M%SZ)"
git branch "$BACKUP_BRANCH" origin/benchmarks
git push origin "$BACKUP_BRANCH"
echo "Rollback branch: $BACKUP_BRANCH"
```

Test Actions without changing Pages first:

1. Run **Blobfuse2 Performance** manually on `main` with `suite=public` and `publish=false`.
2. Confirm both architecture jobs succeed and retrieve their complete private ZIPs from the `results` container. Together they must contain four public summaries: standard and premium block cache for both architectures.
3. Verify each public `summary.json` has four benchmarks, five samples per benchmark, positive throughput, approximately 320 GiB per trial, and a network/I/O ratio of at least 0.75.
4. Run `suite=regression` with `publish=false`. The X86 ZIP must contain eight regression summaries and the ARM64 ZIP two, covering X86 block/file cache, HNS, and ARM64 block cache before unattended publication is enabled.

Scheduled ZIPs use this searchable layout:

```text
results/benchmark-runs/run-<run-id>-attempt-<attempt>/<arch>/blobfuse2-perf-<arch>-suite-<all|public|regression|scheduled>-status-<success|failure>-run-<run-id>-attempt-<attempt>-sha-<12-char-sha>.zip
```

For example, retrieve a successful X86 bundle after setting `PERF_RESULTS_ACCOUNT` and `PERF_RESULTS_KEY` in the shell:

```bash
RUN_ID=<github-run-id>
ATTEMPT=<attempt>
SHORT_SHA=<12-character-workflow-commit>
ARCH=X86
STATUS=success
SUITE=public
BUNDLE="blobfuse2-perf-${ARCH}-suite-${SUITE}-status-${STATUS}-run-${RUN_ID}-attempt-${ATTEMPT}-sha-${SHORT_SHA}.zip"
REMOTE="benchmark-runs/run-${RUN_ID}-attempt-${ATTEMPT}/${ARCH}/${BUNDLE}"

mkdir -p benchmark-artifacts/perf-${ARCH}
./perf_testing/scripts/blob_results.sh download ./blobfuse2 "/tmp/${BUNDLE}" "$REMOTE"
unzip -q "/tmp/${BUNDLE}" -d "benchmark-artifacts/perf-${ARCH}"
```

Useful checks after extracting the ZIPs under `benchmark-artifacts/`:

```bash
find benchmark-artifacts -name failure.json -print
mapfile -d '' public_summaries < <(
	find benchmark-artifacts -path '*/public/summary.json' -print0
)
test "${#public_summaries[@]}" -eq 4
for summary in "${public_summaries[@]}"; do
	jq -e '
		.run.suite == "public" and
		(.benchmarks | length == 4) and
		all(.benchmarks[];
			(.samples | length == 5) and
			(.metrics[.primary_metric].median > 0) and
			(.metrics.io_gib.median > 319.9) and
			(.metrics.network_to_io_ratio.median >= 0.75)
		)
	' "$summary"
done
```

Simulate the Pages migration locally against a copy of the real branch before pushing it:

```bash
DRY_RUN_SITE=/tmp/blobfuse-benchmark-migration
rm -rf "$DRY_RUN_SITE"
mkdir -p "$DRY_RUN_SITE"
git archive origin/benchmarks | tar -x -C "$DRY_RUN_SITE"

python3 perf_testing/scripts/publish_benchmarks.py \
	--results-dir benchmark-artifacts \
	--site-dir "$DRY_RUN_SITE"

test -f "$DRY_RUN_SITE/index.html"
test -f "$DRY_RUN_SITE/developer/index.html"
test -f "$DRY_RUN_SITE/data/public/history.json"
test -f "$DRY_RUN_SITE/data/developer/history.json"
test -d "$DRY_RUN_SITE/release"
test -e "$DRY_RUN_SITE/issueMetric.txt"
test ! -e "$DRY_RUN_SITE/X86"
test ! -e "$DRY_RUN_SITE/ARM64"
test ! -e "$DRY_RUN_SITE/legacy_runs"

python3 -m http.server 8000 --directory "$DRY_RUN_SITE"
```

The first developer-only publication intentionally installs `/developer/` while preserving the legacy root page. The curated public cutover occurs only when at least one new public result exists. For a controlled cutover, run **Blobfuse2 Performance** manually on `main` with `suite=all` and `publish=true` after both no-publish runs pass.

During that first publication, the generated performance paths `X86/`, `ARM64/`, `ARM/`, and `legacy_runs/`, plus the old `_config.yaml`, are removed. `release/`, `issueMetrics/`, and `issueMetric.txt` are preserved. Existing external links to the removed generated paths will return 404; update repository documentation or announce the new root and `/developer/` URLs before cutover if those links are used externally.

After cutover, verify:

```bash
curl -fsS https://azure.github.io/azure-storage-fuse/ >/dev/null
curl -fsS https://azure.github.io/azure-storage-fuse/developer/ >/dev/null
curl -fsS https://azure.github.io/azure-storage-fuse/data/public/history.json | jq '.runs | length'
curl -fsS https://azure.github.io/azure-storage-fuse/data/developer/history.json | jq '.runs | length'
git fetch origin benchmarks
git ls-tree --name-only origin/benchmarks:release/latest
```

The new workload IDs deliberately start new history series; old generated chart data is not converted. Rolling regression checks report `insufficient-baseline` until at least three matching runs exist. Do not lower the gate just to make the transition green. Establish baselines with repeated runs on unchanged code and unchanged infrastructure, then investigate any high MAD before trusting automatic regression failures.

Fixture schema `v4` starts a new persistent fixture namespace for the four-file public workload. Keep older fixture versions until successful public and developer runs verify `v4` on every account. Then remove only obsolete `.blobfuse-benchmark-fixtures/v1/`, `v2/`, and `v3/` paths from each dedicated benchmark container to reclaim space. Never run broad container cleanup against a container containing unrelated data.

If the site migration must be rolled back, use a clean checkout to revert the generated publication commit on the `benchmarks` branch:

```bash
git fetch origin benchmarks
git switch -C benchmarks origin/benchmarks
git log --oneline -5
git revert <publication-commit>
git push origin benchmarks
```

Keep the backup branch until the new scheduled runs, release-version lookup, and public/developer URLs have been stable for several cycles. Avoid force-pushing the backup over `benchmarks`; a revert preserves the migration and rollback audit trail.

## Public benchmark

The public page is generated from the `public` suite and accepts only these dimensions:

- Cache: block cache
- Accounts: standard and premium, without HNS
- Architectures: X86 and ARM64
- Workloads: single-file and multi-file sequential read and durable write

| Operation | FIO jobs | Files | Size per file | Total data | FIO block size |
| --- | ---: | ---: | ---: | ---: | ---: |
| Sequential read | 1 | 1 | 320 GiB | 320 GiB | 10 MiB |
| Sequential read | 4 | 4 | 80 GiB | 320 GiB | 10 MiB |
| Durable sequential write | 1 | 1 | 320 GiB | 320 GiB | 10 MiB |
| Durable sequential write | 4 | 4 | 80 GiB | 320 GiB | 10 MiB |

Every measured FIO job uses `direct=1`, so FIO requests `O_DIRECT` when it opens each benchmark file. The mount configuration deliberately does **not** enable mount-wide `libfuse.direct-io`; normal mount behavior is retained so the kernel can schedule concurrent FUSE requests. In FIO's default unit mode, `bs=10M` means a 10 MiB **application I/O unit**. The kernel and FUSE protocol may split that unit into smaller requests before Blobfuse receives it. The dashboard consistently reports MiB/s and GiB.

All four cases use FIO's synchronous engine. The single-file cases therefore have one FIO job with one application I/O in flight. The aggregate cases have four independent FIO jobs, each with one application I/O in flight against its own file. “Single stream” describes the application workload; it does not disable Blobfuse prefetching, Azure SDK concurrency, or other internal parallelism.

The public read fixtures are remounted with local and kernel cache state cleared before every measured trial. Application direct I/O bypasses buffered application access, while Blobfuse block cache remains the component under test. The network-transfer invariant ensures a supposedly cold read did not become a local-cache result.

Reads use immutable pre-created objects and reject a trial when received network bytes are unexpectedly low. Writes use `end_fsync=1` and reject a trial when transmitted network bytes are unexpectedly low. File-cache profiles require `file_cache.sync-to-flush: true`, so that final fsync uploads the current file; block cache likewise completes pending block uploads and commits the block list on sync.

`end_to_end_throughput_mib_s` is total completed FIO bytes divided by the parent harness timer. The timer starts immediately before launching FIO and stops only after the FIO process exits, so it includes process startup, file open, I/O, final fsync, file close, and process exit. It deliberately excludes Blobfuse mount time, immutable fixture creation, and inter-trial cleanup. `throughput_mib_s` remains an identical compatibility key and the regression primary metric, while `fio_throughput_mib_s` preserves FIO's internal transfer-only rate for diagnostics; FIO's rate is not used for throughput regression decisions. Each result is the median of five isolated trials, with MAD shown as the run-to-run variability signal.

Interpret the single-file result as sustained throughput for one application stream over one file. Interpret the four-file result as aggregate throughput when four independent application streams access four independent files concurrently. The ratio between them shows scaling for this fixed benchmark setup; it neither specifies the concurrency required to saturate a network nor guarantees performance for every VM, network path, cache configuration, or workload.

The public page records the exact Blobfuse2 version, FIO version, kernel, CPU model, CPU count, memory size, Azure VM SKU and region, architecture, account tier, trial count, and a secret-redacted mount-configuration hash with every run. VM SKU, region, configuration, and trial count are history boundaries: D192 is not compared with D96, and five-trial runs are not compared with three-trial runs. Storage-account region, network configuration, and account limits should also be recorded in repository or runner documentation because they can cap throughput independently of Blobfuse2.

Do not add diagnostic or narrow microbenchmarks to the public suite. Public series should remain small, understandable, and comparable across releases.

## Developer regression benchmark

The `regression` suite covers:

- Cold and warm sequential reads
- 4 KiB and 64 KiB random reads
- A time-bounded 64-stream shared-set contention workload
- Durable sequential and random writes
- Mixed 70/30 random read/write
- Parallel create, stat, and delete operations
- Block cache on X86 and ARM64
- File cache on the canonical X86 runner
- Standard, premium, standard HNS, and premium HNS accounts on X86

Metadata workloads use operation rate rather than data throughput. Create and delete each perform 2,000 unique operations per trial (250 files per worker at eight-way concurrency). Warm stat performs 100,000 measured operations by repeating stat 100 times over 1,000 files. FIO creates the stat and delete files during unmeasured setup, which still consumes workflow wall time. Every trial must complete its declared operation count and report a positive rate before it can be summarized.

The metadata primary metric is `operations_per_second`, displayed as ops/s. `operation_duration_seconds` records measured operation time separately from FIO process/setup wall time. FIO's `filecreate`, `filestat`, and `filedelete` engines do not emit operation latency samples, so the metadata jobs deliberately omit percentile settings and are not p99-gated. The `metadata-create-2k`, `metadata-stat-100k`, and `metadata-delete-2k` IDs start new history series rather than mixing these samples with the former 1,000-operation workloads.

Each benchmark publishes its median, median absolute deviation, minimum, maximum, p99 latency, sync latency when present, network traffic, and workload parameters. Raw per-trial FIO JSON, Blobfuse logs, and redacted configs remain in private ZIPs in the results account and are not copied to GitHub Pages.

Before publication, each new result is compared with the last five matching `main` runs. This includes the weekly public sustained-throughput suite. At least three historical runs are required. The default gate is 10% for throughput, IOPS, or operation rate and 20% for p99 latency when that metric exists, widened to three times observed MAD when necessary. A confirmed regression is published, stored with private publisher diagnostics, and then fails the workflow so repository notifications identify it.

## Compare a branch before a PR

Run the **Compare Branch Performance** workflow from the `main` workflow definition and provide a trusted same-repository branch, tag, or commit as `target_ref`.

The workflow:

1. Checks out the stable harness from `main`.
2. Builds baseline and candidate directly with `go build`.
3. Prepares the `quick` and `public` fixture sets once with the baseline binary.
4. Runs both suites for both binaries on the same X86 host, storage account, and selected cache mode.
5. Alternates baseline-first and candidate-first ordering between workflow runs.
6. Writes one suite-labeled comparison table containing six quick workloads and four public workloads to the job summary.
7. Stores one private ZIP containing the combined `comparison.html`, compact JSON, all four suite summaries, raw FIO output, Blobfuse logs, metadata, and a redacted mount config.

Comparison ZIPs use this layout:

```text
results/benchmark-comparisons/run-<run-id>-attempt-<attempt>/blobfuse2-compare-X86-<account-type>-<cache-mode>-status-<pass|regression|failure>-run-<run-id>-attempt-<attempt>-sha-<12-char-candidate-sha>.zip
```

The comparison defaults to a 10% throughput, IOPS, or operation-rate threshold and a 20% p99-latency threshold when available. A metric's effective threshold expands to three times the observed median absolute deviation to avoid classifying noisy samples as regressions. Enable `fail_on_regression` when the workflow should act as a gate.

The public workload comparison uses the selected comparison cache mode; it does not publish branch results to GitHub Pages. With the default five trials, its four 320 GiB cases move approximately 6.4 TiB per revision, or 12.8 TiB across baseline and candidate, in addition to the quick suite. Use one trial for an initial workflow smoke test. File-cache comparison requires at least 1 TiB free on a dedicated non-root local filesystem because initial public fixture preparation can cache up to 640 GiB.

Candidate binaries receive benchmark account credentials. Create the `performance-benchmark-compare` GitHub environment, configure required reviewers on it, and approve only trusted revisions. Scheduled `main` benchmarks do not use this environment, so they remain unattended.

## Adding or changing a workload

Workload definitions live in `config/benchmark/suites.json`; FIO jobs live below `config/benchmark/`.

Follow these rules:

1. Give every workload a stable ID, explicit primary metric, comparison direction, cache state, block size, concurrency, and working-set description.
2. Read workloads must use an immutable fixture and `allow_file_create=0`.
3. Cold workloads must remount between trials. Warm workloads must declare `warmup: true`.
4. Durable writes must use `end_fsync=1`; keep sync latency separate from I/O latency. File-cache profiles must set `file_cache.sync-to-flush: true` so end fsync includes the upload.
5. Use `group_reporting=1` for cloned FIO jobs and unique files or explicit offsets when workers must not overlap.
6. Public data-path jobs must use FIO `direct=1`; benchmark mount profiles must not enable mount-wide `libfuse.direct-io`.
7. Keep five trials in scheduled suites and publish median plus MAD rather than a single result or best run.
8. Bump `fixture_version` when fixture names, sizes, or content assumptions change.
9. Start a new workload ID when semantics change. Reusing an ID for a different workload creates a misleading historical series.

Validate local changes without Azure I/O:

```bash
find perf_testing/config/benchmark -name '*.fio' -exec fio --parse-only {} \;
jq empty perf_testing/config/benchmark/suites.json
python3 -m py_compile perf_testing/scripts/*.py
```

## Result lifecycle

- Public compact history: 730 days
- Developer compact history: 400 days
- Raw scheduled and comparison ZIPs: private `results` container; configure Azure lifecycle management to delete blobs below `benchmark-runs/` and `benchmark-comparisons/` after 30 days
- Persistent fixtures: versioned under `.blobfuse-benchmark-fixtures/` in the reserved benchmark container
- Per-run writes and metadata: removed after each suite

Environment metadata includes the kernel, CPU model, memory, FIO version, Blobfuse2 version, and a secret-redacted configuration hash. Treat an environment change as a possible series boundary when investigating a trend.
