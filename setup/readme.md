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
The public 4.0 beta metadata checked on 2026-10-30 provides `libfuse3.so.3`
(fuse3 3.16.2) on both architectures, matching the existing fuse3 build ABI.
Production compatibility and signing-key acceptance still require validation.
The `cm2` suffix previously used during upload was a local filename marker, not
the target distribution or an RPM rebuild.

Set these runtime parameters when `publish_artifacts: true`:

- `publish_scope: all` publishes all manifest destinations, including 3.0 and 4.0.
- `publish_scope: azurelinux4` uploads and publishes only the two 4.0 destinations.
  Build, signing, and distro-test jobs still run, including the existing Azure
  Linux 4.0 x86_64 test job. GitHub release edits and container publishing are
  disabled in this mode, even if `post_release` is selected.

The Azure Linux 4.0 production destinations supplied by the release team are
configured directly in the manifest; no repository-ID runtime inputs are needed:

- x86_64: `azurelinux-4.0-prod-microsoft-x86_64-yum`
- aarch64: `azurelinux-4.0-prod-microsoft-aarch64-yum`

The publisher validates all selected artifacts and IDs before uploading, only
uploads selected package types, stops on PMC failures, and publishes each
selected repository once. APT architectures share repositories; Azure Linux
architectures have separate repositories and both must be published.
Azure Linux 3.0 preview packages retain their automatic `prod` to `preview`
routing. Azure Linux 4.0 preview packages are blocked from production before
any uploads, for both publish scopes. Beta publishing remains a separate workflow.
Since `all` includes 4.0 production, a preview run with this manifest fails
validation rather than partially publishing to the other distributions.

On 2026-10-30, the public `4.0/beta/microsoft` metadata confirmed
`blobfuse2-preview` version `2.5.5~preview.1-1` on both architectures:
[x86_64](https://packages.microsoft.com/azurelinux/4.0/beta/microsoft/x86_64/)
and [aarch64](https://packages.microsoft.com/azurelinux/4.0/beta/microsoft/aarch64/).
This confirms the beta publication, not access to the internal production repos.

Production and beta use different public paths:

- Production: `azurelinux/4/prod/microsoft/{x86_64,aarch64}/`
- Beta: `azurelinux/4.0/beta/microsoft/{x86_64,aarch64}/`

On 2026-10-30, production `repodata/repomd.xml` and primary package metadata
were accessible for both architectures, but contained no Blobfuse packages.
The [production index](https://packages.microsoft.com/azurelinux/4/prod/)
did not list its subdirectories; this does not mean the repositories are empty.
The `azurelinux/4.0/prod/` path returned 404: production uses `4`, not `4.0`.
Public URL paths do not establish internal PMC IDs. Keep the release team's
`azurelinux-4.0-prod-microsoft-*-yum` IDs unless PMC or the team indicates otherwise.

### Publish 2.5.6 only to Azure Linux 4.0

1. Verify the manifest's two **production** repository IDs, publishing permission for the
   pipeline's `WIF_MI_ESRP_V5` identity, and signing-key acceptance with the Azure
   Linux team. The production repositories are publicly accessible under
   `azurelinux/4/prod/microsoft/`, not `azurelinux/4.0/prod/microsoft/`.
   Do not substitute beta IDs without agreement.
2. Create a release branch from `blobfuse2-2.5.6` and apply only this pipeline,
   manifest, publisher, and documentation change. Do not change the version.
   The pipeline builds the selected branch; the `tag` parameter only labels the
   GitHub release and **does not select build source**. Running from current
   main would build 2.5.7 development code.
3. First queue that branch with `publish_artifacts: false`,
   `publish_scope: azurelinux4`, `post_release: false`, `update_version: false`,
   and `tag: blobfuse2-2.5.6`. Inspect the `blobfuse2-signed/mariner` artifacts,
   RPM signatures, package version 2.5.6, and the Azure Linux 4.0 x86_64 tests.
   Before publishing aarch64, also install and test its signed RPM on Azure
   Linux 4.0 aarch64: the current pipeline has no 4.0 aarch64 test agent.
   Validate package dependency resolution, `blobfuse2 --version`,
   `bfusemon --help`, and Blob/ADLS mount/read/write/unmount.
4. Queue the same unchanged branch with `publish_artifacts: true`,
   `publish_scope: azurelinux4`, and
   `post_release: false`. All build/test gates run again. Check the
   `Publish target` log entries: only the two 4.0
   repositories should appear. No 3.0, Ubuntu, Debian, RHEL, or other repository
   is updated or published in this mode.
5. After PMC publishing/replication completes, verify repository metadata and
   `tdnf list --available blobfuse2` on clean 4.0 machines of both architectures.
   Install version 2.5.6 from the intended repository and repeat the smoke tests.
   Coordinate public availability with the Azure Linux release team.

To reuse the exact signed artifacts from the original 2.5.6 run instead of
rebuilding, an authorized release operator can download that run's complete
`blobfuse2-signed` artifact, perform step 3's 4.0 validation, and invoke the same
publisher from an authenticated PMC/WIF release environment:

```bash
bash tools/publish-packages.sh setup/packages.csv /path/to/blobfuse2-signed azurelinux4
```

Use the original `mariner/` signed RPMs, not the differently signed GitHub RHEL
assets. This command makes live changes; it is not a dry run.

Publisher regression tests (no PMC access or Azure credentials needed):

```bash
bash -n tools/publish-packages.sh
python3 -B -m unittest discover -s tools -p 'test_publish_packages.py' -v
```
