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

## Publishing Blobfuse2 packages

The [release pipeline](../blobfuse2-release.yaml) uses [packages.csv](./packages.csv)
to select PMC destinations. Azure Linux 3.0 uses the existing `mariner`-signed
fuse3 RPMs for x86_64 and aarch64; Azure Linux 4.0 reuses those same signed RPMs.
Azure Linux 4 production provides `libfuse3.so.3` (fuse3 3.16.2), not the
`libfuse3.so.4` ABI used by the Ubuntu 26 `fuse3-4` build. The Mariner-signed
2.5.6 RPMs were published to AZL4 production on 2026-11-05.
The `cm2` suffix previously used during upload was a local filename marker, not
the target distribution or an RPM rebuild.

Set these runtime parameters when `publish_artifacts: true`:

- `publish_scope: all` publishes all manifest destinations, including 3.0 and 4.0.
- `publish_scope: azurelinux4` uploads and publishes only the two 4.0 destinations.
  Build, signing, and distro-test jobs still run, including the existing Azure
  Linux 4.0 x86_64 test job. GitHub release edits and container publishing are
  disabled in this mode, even if `post_release` is selected.

The confirmed Azure Linux 4 production destinations are
configured directly in the manifest; no repository-ID runtime inputs are needed:

- x86_64: `azurelinux-4-prod-microsoft-x86_64-yum`
- aarch64: `azurelinux-4-prod-microsoft-aarch64-yum`

The publisher validates all selected artifacts and IDs before uploading, only
uploads selected package types, stops on PMC failures, and publishes each
selected repository once. APT architectures share repositories; Azure Linux
architectures have separate repositories and both must be published.
Azure Linux 3.0 preview packages retain their automatic `prod` to `preview`
routing. Preview releases explicitly skip AZL4 production while continuing to
publish to the other selected distributions. An `azurelinux4`-only preview run
logs that all targets were skipped and performs no PMC operations.
Beta AZL4 publishing remains a separate workflow.

Production and beta use different public paths:

- Production: `azurelinux/4/prod/microsoft/{x86_64,aarch64}/`
- Beta: `azurelinux/4.0/beta/microsoft/{x86_64,aarch64}/`

The release stage installs PMC in `$(Agent.TempDirectory)/blobfuse-pmc` using a
dedicated Python user base and passes its absolute executable path as `PMC_BIN`
to subsequent tasks. This avoids dependence on user-local scripts being on PATH.
The publisher defaults to `pmc` on PATH when `PMC_BIN` is not supplied.

The existing release and nightly matrices already test AZL4 x86_64 with fuse3.
An AZL4 ARM64 agent is not configured; ARM64 release validation must be performed
separately. Verify signatures with the approved Azure Linux key and perform
install/mount/read/write/unmount tests on both architectures.

This integration does not migrate the full release pipeline to the official
1ES template. That is a separate prerequisite where organization policy requires
1ES adoption; no breakglass or enforcement bypass is introduced here.

### Publishing a selected release

The pipeline builds the selected source branch. Its `tag` parameter labels the
GitHub release and does not select source code. Use the intended release source,
not a newer development branch, when rebuilding a previous version.

To publish already signed artifacts from an authenticated PMC/WIF environment:

```bash
bash tools/publish-packages.sh setup/packages.csv /path/to/blobfuse2-signed azurelinux4
```

Use the original `mariner/` signed RPMs, not the differently signed GitHub RHEL
assets. This command uploads and publishes selected packages; it is not a dry
run. No 2.5.6 package IDs or checksums are pinned in the mainline publisher:
future releases use the artifacts produced by that release.

After replication, verify the production repositories on both architectures:

```bash
sudo dnf5 --refresh install blobfuse2
blobfuse2 --version
```

Publisher regression tests (no PMC access or Azure credentials needed):

```bash
bash -n tools/publish-packages.sh
python3 -B -m unittest discover -s tools -p 'test_publish_packages.py' -v
```
