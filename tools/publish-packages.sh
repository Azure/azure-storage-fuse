#!/bin/bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
    echo "Usage: $0 <packages.csv> <signed-artifact-directory> <all|azurelinux4>" >&2
    exit 1
fi

manifest=$1
artifact_dir=$2
scope=$3
if [[ $scope != all && $scope != azurelinux4 ]]; then
    echo "Unsupported publish scope: $scope" >&2
    exit 1
fi

declare -A patterns=(
    [fuse2AmdDeb]='blobfuse2*fuse2.x86_64.deb'
    [fuse3AmdDeb]='blobfuse2*fuse3.x86_64.deb'
    [fuse3ArmDeb]='blobfuse2*fuse3.arm64.deb'
    [fuse34AmdDeb]='blobfuse2*fuse3-4.x86_64.deb'
    [fuse34ArmDeb]='blobfuse2*fuse3-4.arm64.deb'
    [fuse3AmdRpm]='blobfuse2*fuse3.x86_64.rpm'
    [fuse3ArmRpm]='blobfuse2*fuse3.aarch64.rpm'
    [marinerFuse3AmdRpm]='mariner/blobfuse2*fuse3.x86_64.rpm'
    [marinerFuse3AarchRpm]='mariner/blobfuse2*fuse3.aarch64.rpm'
)
declare -A package_files package_ids published legacy_repos
distros=()
types=()
repos=()
releases=()
shopt -s nullglob

# Validate every selected destination and artifact before making any PMC changes.
while IFS=, read -r distro package_type repo release || [[ -n $distro ]]; do
    [[ -z $distro || $distro == \#* ]] && continue
    if [[ $distro != AzureLinux-4.0-* ]]; then
        legacy_repos["$repo"]=1
        [[ $distro == Mariner-* ]] && legacy_repos["${repo/prod/preview}"]=1
        [[ $scope == azurelinux4 ]] && continue
    fi
    if [[ ! $repo =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]+$ ]]; then
        echo "Missing or invalid PMC repository ID for $distro: $repo" >&2
        exit 1
    fi
    if [[ -z ${patterns[$package_type]:-} ]]; then
        echo "Unsupported package type for $distro: $package_type" >&2
        exit 1
    fi
    pattern=${patterns[$package_type]}
    files=("$artifact_dir"/$pattern)
    if [[ ${#files[@]} -ne 1 || ! -f ${files[0]} ]]; then
        echo "Expected exactly one signed artifact for $distro: $artifact_dir/$pattern" >&2
        exit 1
    fi
    package_files["$package_type"]=${files[0]}
    package_name=${files[0]##*/}
    if [[ $distro == AzureLinux-4.0-* && $repo == *-prod-* && $package_name == *preview* ]]; then
        echo "Refusing to publish a preview package to Azure Linux 4.0 production: $package_name -> $repo" >&2
        exit 1
    fi
    if [[ $distro == Mariner-* && $package_name == *preview* ]]; then
        repo=${repo/prod/preview}
    fi
    distros+=("$distro")
    types+=("$package_type")
    repos+=("$repo")
    releases+=("$release")
done < "$manifest"

if [[ ${#distros[@]} -eq 0 ]]; then
    echo "No repositories selected for scope: $scope" >&2
    exit 1
fi
for ((i = 0; i < ${#distros[@]}; i++)); do
    if [[ ${distros[$i]} == AzureLinux-4.0-* && -n ${legacy_repos[${repos[$i]}]:-} ]]; then
        echo "Azure Linux 4.0 must not target an existing distribution repository: ${repos[$i]}" >&2
        exit 1
    fi
    echo "Publish target: ${distros[$i]} -> ${repos[$i]} (${package_files[${types[$i]}]})"
done

pmc_args=(--auth-type wif --base-url https://pmc-ingest.trafficmanager.net/api/v4)
for ((i = 0; i < ${#distros[@]}; i++)); do
    package_type=${types[$i]}
    if [[ -z ${package_ids[$package_type]:-} ]]; then
        package_ids["$package_type"]=$(pmc "${pmc_args[@]}" --id-only package upload "${package_files[$package_type]}")
        if [[ -z ${package_ids[$package_type]} ]]; then
            echo "PMC returned no package ID for $package_type" >&2
            exit 1
        fi
    fi
    args=(repo package update --add-packages "${package_ids[$package_type]}" "${repos[$i]}")
    [[ -n ${releases[$i]} ]] && args+=("${releases[$i]}")
    pmc "${pmc_args[@]}" "${args[@]}"
done

# Architectures may share a repository (APT) or have separate repositories (Azure Linux).
for repo in "${repos[@]}"; do
    if [[ -z ${published[$repo]:-} ]]; then
        pmc "${pmc_args[@]}" repo publish "$repo"
        published["$repo"]=1
    fi
done
