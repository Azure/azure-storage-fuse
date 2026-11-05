systemd service file for blobfuse2

# Step to install this to systemd
1. If you are existing blobfuse user follow the MIGRATION.md file on how to convert blobfuse config and cli parameters to blobfuse2 compliant config
2. Prepare the config file. Name of the container should be specified by `azstorage:container` in config file or AZURE_STORAGE_ACCOUNT_CONTAINER environment variable. 
3. Download blobfuse2.service and put it into /etc/systemd/system
4. Edit the file, changing environment values in Service block.
5. Run command to reload service config files: `systemctl daemon-reload`
6. Start service: `systemctl start blobfuse2.service`
7. Make the service starting with system: `systemctl enable blobfuse2.service`
8. Please Note that the example has the User AzureUser, please create a user called AzureUser or replace this with an existing user.

## One-off 2.5.6 publication to Azure Linux 4

This release recovery branch is now AZL4-only. Do not merge it into main.
The [release pipeline](../blobfuse2-release.yaml) downloads the original signed
2.5.6 RPMs, without rebuilding, re-signing, changing GitHub releases, publishing
containers, or touching the earlier SUSE, Rocky, or Azure Linux 3 feeds.

1. Use the existing Azure DevOps release pipeline definition and choose branch
   `release/2.5.6-publish-remaining`. Do not create a new pipeline definition:
   artifact downloads use the current definition's ID.
   The pipeline extends the official 1ES template required by the organization,
   using `blobfuse2-pool` / `ubuntu22-1espt` for publication and
   `windows2022-1espt` for SDL source analysis, matching the existing CI setup.
   If prompted, authorize the pipeline to read
   `1ESPipelineTemplates/1ESPipelineTemplates` and use the agent pool.
   Keep the required 1ES checks enabled; breakglass is not needed for this change.
2. Set `signed_artifacts_run_id` to the `buildId` of the original 2.5.6 release
   run that produced `blobfuse2-signed`, not the later recovery run. The original
   run may have failed during publishing; its signing artifacts are still used.
3. First run with `publish_artifacts: false`. This now downloads and checks the
   artifacts rather than skipping the pipeline. Inspect the two AZL4 production
   targets and SHA256 output.
4. Both RPMs must come from `blobfuse2-signed/mariner/`, not the root directory.
   The x86_64 checksum is pinned to the signature-checked, mount-tested RPM:
   `979c26454a2b58e84e2cc813cfa4d9083b67038402390581433d7ef116803a1b`.
   Before publishing, verify the ARM64 RPM's signatures with the approved Azure
   Linux signing key, install and mount-test it on AZL4 ARM64, and obtain its
   checksum with `sha256sum`. Supply that value as `aarch64_sha256`.
   The pipeline checks hashes; it does not itself run ARM64 mounts or verify RPM
   signatures. Supplying the checksum is not a substitute for those checks.
5. Rerun with the same original build ID, the validated `aarch64_sha256`, and
   `publish_artifacts: true`. Authorize the existing `BlobFuse/Blobfuse` package
   feed and `WIF_MI_ESRP_V5` service connection if prompted.
   PMC is installed into a job-local Python user base under
   `$(Agent.TempDirectory)/azl4-pmc`. Both installation and publication use that
   same Python user base, and publication invokes the absolute `bin/pmc` path
   rather than depending on PATH changes persisting between tasks. Installation
   ignores globally installed packages so the executable is always created in
   the job-local directory; both tasks check `pmc --version` before proceeding.
6. Only these PMC repositories will be updated and published:
   - `azurelinux-4.0-prod-microsoft-x86_64-yum`
   - `azurelinux-4.0-prod-microsoft-aarch64-yum`
   Coordinate with the owners: repository publication may expose other pending
   changes, and publication across the two architectures is not atomic.
   If a command fails, inspect PMC state before retrying.
7. After replication, verify both production repositories on clean AZL4 machines
   using `dnf5 --refresh list --available blobfuse2`, install
   `blobfuse2-2.5.6`, and repeat signature and mount/read/write/unmount checks.

Production URLs use `azurelinux/4/prod/microsoft/`, while beta uses
`azurelinux/4.0/beta/microsoft/`. No beta repository is targeted by this pipeline.
