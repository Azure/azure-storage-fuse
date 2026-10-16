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

# Tachyon 2.5.6-preview.2 release

This branch prepares an Ubuntu and Azure Linux preview. All existing distro
tests, including Azure Linux 4.0 tests, remain enabled; a failure on any tested
distro still blocks the release.

1. Merge the release preparation PR into `feature/tachyon`.
2. Create the GitHub tag `blobfuse2-2.5.6-preview.2` at the intended commit on
   `feature/tachyon`, and prepare a matching draft prerelease. The pipeline uses
   `action: edit`; it does not create a new GitHub release.
3. Manually run [blobfuse2-release.yaml](../blobfuse2-release.yaml), selecting
   `feature/tachyon` as the source branch. Use these parameters:

   | Parameter | Value |
   |-----------|-------|
   | `tag` | `blobfuse2-2.5.6-preview.2` |
   | `unit_test` | `true` |
   | `post_release` | `true` |
   | `prerelease` | `true` |
   | `draft` | `true` for initial review |
   | `publish_artifacts` | `true` only when ready to publish to package feeds |
   | `update_version` | Unused; the version is committed in `common/types.go` |

4. Do not push changes to `feature/tachyon` during the run: build and test jobs
   independently check out its current tip, not the queued commit SHA.
5. Review the draft's assets before publishing it. GitHub uploads include only
   the existing Ubuntu DEBs and the signed Azure Linux RPMs for x86_64 and
   aarch64. Other distro packages remain available as pipeline test artifacts,
   but are not selected for the GitHub release. Use a new, empty draft for this
   tag; the edit task is not intended to remove unrelated assets already there.

[packages.csv](packages.csv) retains Ubuntu 18.04, 20.04, 22.04, 24.04 and 26.04,
and Azure Linux 3.0, with their existing architecture mappings. Ubuntu preview
packages are named `blobfuse2-preview` and published to the listed Ubuntu
production feeds. The pipeline switches Azure Linux 3.0 entries from `prod` to
`preview` feeds when the package version contains `preview`. No Mariner 2.0 or
Azure Linux 4.0 feed mapping is added.

GitHub draft status does not delay package-feed publishing. For a review-only
run, leave `publish_artifacts` set to `false`. The pipeline still builds, signs
and tests all existing distro targets and retains its existing container
publishing steps.
