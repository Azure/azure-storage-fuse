#!/bin/bash
# Fail fast on any error, unset variable, or pipe failure so a broken
# download/extract cannot leave the agent in a half-installed state.
set -euo pipefail

work_dir=$(echo "$1" | sed 's:/*$::')

# Microsoft build of Go (FIPS-capable; ships systemcrypto GOEXPERIMENT).
# Pinned to a specific patch version for reproducible builds. Override by
# exporting GO_VERSION before run (either a specific version like "1.26.4"
# or a major.minor stream like "1.26" which the aka.ms redirector resolves
# to the latest patch in that line).
version="${GO_VERSION:-1.26.4}"
arch=$(dpkg --print-architecture 2>/dev/null || true)
if [ -z "$arch" ]; then
  case "$(uname -m)" in
    aarch64|arm64) arch="arm64" ;;
    *) arch="amd64" ;;
  esac
fi

if [ "$arch" != "arm64" ]
then
  arch="amd64"
fi

echo "Installing Microsoft Go (FIPS) on : $arch  Stream : $version"
tarball="go${version}.linux-${arch}.tar.gz"

# TLS-verified download from the Microsoft-controlled aka.ms redirector.
# --tries / --timeout avoid hanging the pipeline on transient network issues.
wget --tries=3 --timeout=60 -O "$work_dir/$tarball" \
  "https://aka.ms/golang/release/latest/${tarball}"

# Verify the tarball against the official SHA256 sidecar published next to
# the release. This guards against a compromised redirector, a corrupted
# upstream artifact, or a man-in-the-middle that has TLS but not the
# checksum. The sidecar lists the resolved release filename (e.g.
# "go1.26.4-20260508.1.linux-amd64.tar.gz"), so we extract just the hash
# and compare it against the locally computed hash of the downloaded file.
wget --tries=3 --timeout=60 -O "$work_dir/$tarball.sha256" \
  "https://aka.ms/golang/release/latest/${tarball}.sha256"

expected_sha=$(awk '{print $1}' "$work_dir/$tarball.sha256")
actual_sha=$(sha256sum "$work_dir/$tarball" | awk '{print $1}')
if [ "$expected_sha" != "$actual_sha" ]; then
  echo "ERROR: SHA256 mismatch for $tarball" >&2
  echo "  expected: $expected_sha" >&2
  echo "  actual:   $actual_sha" >&2
  rm -f "$work_dir/$tarball" "$work_dir/$tarball.sha256"
  exit 1
fi
echo "SHA256 verified: $actual_sha"
rm "$work_dir/$tarball.sha256"

# Stage the new toolchain in /usr/local/go.new before touching the existing
# install. Keeping the staging dir on the same filesystem as /usr/local/go
# ensures the final `mv` is atomic, so a failed tar / power loss / partial
# extract cannot leave the agent without a working Go.
NEW_GOROOT="/usr/local/go.new"
OLD_GOROOT="/usr/local/go.old"
sudo rm -rf "$NEW_GOROOT" "$OLD_GOROOT"
sudo mkdir -p "$NEW_GOROOT"

# Clean up the staging dir if anything below fails before we swap.
trap 'sudo rm -rf "$NEW_GOROOT"' ERR

# Extract into the staging dir. --strip-components=1 drops the tarball's
# top-level "go/" so the contents land directly under NEW_GOROOT.
sudo tar -C "$NEW_GOROOT" --strip-components=1 -xzf "$work_dir/$tarball"

# Validate the staged toolchain BEFORE swapping. If any of these checks
# fail, the existing /usr/local/go is left intact.
"$NEW_GOROOT/bin/go" version

# Verify this is the Microsoft build of Go, not upstream. The MS fork
# ships a MICROSOFT_REVISION file at GOROOT that upstream Go does not.
# This is the definitive FIPS-lineage check: without MS Go, the
# systemcrypto GOEXPERIMENT is unavailable and binaries built here will
# not carry the microsoft_systemcrypto marker required for FIPS.
if [ ! -f "$NEW_GOROOT/MICROSOFT_REVISION" ]; then
  echo "ERROR: $NEW_GOROOT/MICROSOFT_REVISION not found." >&2
  echo "       The installed toolchain is not the Microsoft build of Go." >&2
  echo "       FIPS-compliant binaries cannot be produced. Aborting." >&2
  exit 1
fi
echo "Microsoft Go revision: $(cat "$NEW_GOROOT/MICROSOFT_REVISION")"

# Atomic swap: move the existing install aside, then move the new one in.
# Both renames are within /usr/local (same filesystem) so each is atomic
# on Linux. After the swap, prune the old install. The trap above is
# cleared since the staging dir has been moved into place.
if [ -d /usr/local/go ]; then
  sudo mv /usr/local/go "$OLD_GOROOT"
fi
sudo mv "$NEW_GOROOT" /usr/local/go
sudo rm -rf "$OLD_GOROOT"
trap - ERR

sudo ln -sf /usr/local/go/bin/go /usr/bin/go
sudo ln -sf /usr/local/go/bin/gofmt /usr/bin/gofmt

# Remove the tarball
rm "$work_dir/$tarball"
