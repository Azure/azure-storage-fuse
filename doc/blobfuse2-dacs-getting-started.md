# Getting started with BlobFuse2 and DACS on AKS (Preview)

Mount an existing Azure Blob container into an AKS workload with a shared,
distributed read cache. This walkthrough downloads
[Microsoft Phi-1.5](https://huggingface.co/microsoft/phi-1_5), uploads it to Blob
Storage, and verifies that a reader on a second node can reuse cached data.

[Distributed Accelerator for Cloud Storage (DACS)](https://github.com/Azure/dacs)
is an open-source distributed caching system. BlobFuse2's `distributed_cache`
component lets applications use it through a mounted volume without a
DACS-specific API. Cache misses fetch data from Blob Storage and populate DACS
asynchronously so other workloads can reuse it.

You will:

1. Check eligibility and prerequisites.
2. Choose two reader nodes and save your configuration.
3. Verify the Blob CSI driver and install a two-server DACS cache.
4. Grant the mount identity access and upload the model (about 2.64 GiB) to
   your Blob container.
5. Create a read-only volume and two reader pods on different nodes.
6. Read the model from each node and confirm DACS was used.
7. Troubleshoot if the result is not a pass.
8. Clean up.

> [!WARNING]
> **Evaluation scope.** DACS is pre-release software, and this topology is for
> evaluation only, not production sizing. DACS treats the entire Kubernetes
> cluster as a single trust domain and provides no tenant isolation, so do not
> use it in a multi-tenant cluster. See the
> [DACS security model](https://github.com/Azure/dacs/blob/main/docs/security-model.md).
> Use a cache dedicated to this test.

## 1. Check eligibility before creating resources

### Compatibility

| Component | Requirement | Verified in |
| --- | --- | --- |
| AKS version | 1.34 or later (managed Blob CSI add-on path) | `kubectl version` |
| Reader node OS | Ubuntu 20.04, 22.04, or 24.04 | Step 2 node listing |
| Blob CSI driver | 1.27.11 or later, with distributed-cache support | Step 3 |
| Node-host BlobFuse2 | 2.5.6, which introduces distributed cache in preview | Step 3 |
| Volume access | Read-only mounts only | Step 5 |

The main path uses the **AKS-managed Blob CSI add-on** on an existing cluster.
The add-on uses the BlobFuse2 installed in the node image; it does not install
or pin the binary. Without the managed add-on, use the
[open-source driver](#a-open-source-blob-csi-driver) instead.

### Prerequisites

- **Cluster:** at least two Ubuntu worker nodes for readers, plus capacity for
  the DACS controller and two 2 GiB ephemeral CacheServers (CacheServers may
  share reader nodes if resources permit). For sizing, NVMe/RAID preparation,
  and operations, see the
  [DACS deployment guide](https://github.com/Azure/dacs/blob/main/user-guide.md).
- **Workstation:** Bash, Helm 3, `kubectl`, Azure CLI (`az`), and the
  [Hugging Face CLI](https://huggingface.co/docs/huggingface_hub/guides/cli)
  (`hf`); 4 GB of free disk space; access to Hugging Face and your storage
  account. Run the commands from any directory; no repository checkout is needed.
- **Cluster network:** nodes can pull the driver, DACS, and BusyBox images
  (the DACS charts and images at `dacstest.azurecr.io` are publicly readable),
  reach Blob Storage, and reach the DACS CacheServers.
- **Kubernetes permissions:** install cluster-scoped resources (including DACS
  CRDs); create namespaces, PVs, PVCs, and pods (and Secrets, for the
  account-key fallback); list CacheServer pods; and run commands in their
  `cacheserver` containers. The metrics check needs `curl` in those containers,
  not Prometheus.
- **Storage:** an existing storage account and Blob container, and an upload
  identity with **Storage Blob Data Contributor** or equivalent.
- **Mount identity:** an AKS cluster with a managed kubelet identity, and
  permission to create role assignments on the storage account (for example,
  **Owner**, **User Access Administrator**, or **Role Based Access Control
  Administrator**). Step 4 grants that identity **Storage Blob Data Reader** on
  the container only; it is separate from your upload identity. If you can't
  use a managed identity, use the
  [account-key fallback](#b-account-key-authentication-fallback), which needs
  shared-key authentication enabled on the storage account.

## 2. Choose nodes and set shared configuration

### Choose two reader nodes

Inspect your context and nodes. Pick two nodes in the same Ubuntu node pool and
note their `kubernetes.io/hostname` values.

```bash
kubectl config current-context
kubectl cluster-info
kubectl version
kubectl get nodes -o wide -L kubernetes.azure.com/agentpool,kubernetes.io/hostname
```

### Set the configuration

Replace the seven placeholders. `COLD_NODE` and `WARM_NODE` must be different
nodes in `NODE_POOL`; `AKS_RESOURCE_GROUP` and `AKS_CLUSTER` identify your
cluster and are used by the managed-identity step. The commands save the
non-secret settings to a local working directory so you can reuse them.

```bash
export STORAGE_ACCOUNT='<storage-account-name>'
export STORAGE_CONTAINER='<container-name>'
export NODE_POOL='<supported-ubuntu-node-pool>'
export COLD_NODE='<first-node-hostname-label>'
export WARM_NODE='<second-node-hostname-label>'
export AKS_RESOURCE_GROUP='<aks-cluster-resource-group>'
export AKS_CLUSTER='<aks-cluster-name>'
export DACS_NAMESPACE='dacs-system'
export DACS_CACHE='cache-sample'
# Assumes the standard cluster.local DNS domain; change it if yours differs.
export DACS_ENDPOINT="$DACS_CACHE-discovery.$DACS_NAMESPACE.svc.cluster.local:9065"
export DEMO_DIR
DEMO_DIR=$(mktemp -d "$HOME/blob-dacs-demo.XXXXXX")
export MODEL_REVISION='77aa61eeac94fbf33d492b9f2744c98b42d5b5eb'
export MODEL_SHA256='b4f562cc4b12f1154a0b658b9d74ab9d024a686420fdf154ebc00a2219976df7'
export MODEL_DIR="$DEMO_DIR/phi-1_5-$MODEL_REVISION"
export BLOB_PREFIX="models/phi-1_5/$MODEL_REVISION/gsg-$(date -u +%Y%m%dT%H%M%SZ)-$RANDOM"
export MODEL_PATH="/mnt/blob/$BLOB_PREFIX/model.safetensors"
# The PV in step 5 passes this to BlobFuse2; the driver's default 10.0.0.10 is
# fixed, not discovered. Set it manually if your DNS Service is not kube-dns.
export CLUSTER_DNS
CLUSTER_DNS=$(kubectl get service kube-dns -n kube-system \
  -o jsonpath='{.spec.clusterIP}')

test -n "$CLUSTER_DNS" || echo 'ERROR: CLUSTER_DNS is empty; set it manually.'
test "$COLD_NODE" != "$WARM_NODE" || echo 'ERROR: COLD_NODE and WARM_NODE must differ.'

declare -p STORAGE_ACCOUNT STORAGE_CONTAINER NODE_POOL COLD_NODE WARM_NODE \
  AKS_RESOURCE_GROUP AKS_CLUSTER DACS_NAMESPACE DACS_CACHE DACS_ENDPOINT \
  DEMO_DIR MODEL_REVISION MODEL_SHA256 MODEL_DIR BLOB_PREFIX MODEL_PATH \
  CLUSTER_DNS > "$DEMO_DIR/settings.env"
printf 'Restore settings in a new terminal: source %q\n' "$DEMO_DIR/settings.env"
```

### Run preflight checks

```bash
kubectl get nodes -l "kubernetes.azure.com/agentpool=$NODE_POOL" \
  -o wide -L kubernetes.io/hostname
kubectl auth can-i create persistentvolumes
kubectl auth can-i create customresourcedefinitions.apiextensions.k8s.io
kubectl auth can-i create pods/exec -n "$DACS_NAMESPACE"

az login
az account show --query '{subscription:name,subscriptionId:id}' -o table
az storage container show --account-name "$STORAGE_ACCOUNT" \
  --name "$STORAGE_CONTAINER" --auth-mode login -o table
```

**Checkpoint:** both selected nodes are Ready and in the supported pool, the
permission checks return `yes`, and the account/container lookup succeeds.
Resolve any failures before continuing; these checks cover only part of the
prerequisites. In a new terminal, source the printed settings file instead of
rerunning the configuration block, which would generate a new blob prefix.

## 3. Verify the driver and install DACS

### Verify the AKS-managed Blob CSI add-on

Confirm the add-on is enabled on your cluster; see the
[AKS driver documentation](https://learn.microsoft.com/azure/aks/azure-blob-csi)
to enable it. If you use the
[open-source driver](#a-open-source-blob-csi-driver) instead, install it first,
then return here.

```bash
kubectl rollout status daemonset/csi-blob-node -n kube-system --timeout=10m
kubectl get daemonset csi-blob-node -n kube-system \
  -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{": "}{.image}{"\n"}{end}'
```

On **each reader node's host** (not a workstation or container, which don't
verify the binary the proxy uses), run the following using your approved
node-access method in the
[CSI debugging guide](https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/csi-debug.md):

```bash
blobfuse2 -v
```

**Checkpoint:** the node DaemonSet is ready on both reader nodes, its `blob`
image is 1.27.11 or later, and each host reports `blobfuse2 version 2.5.6`.
Stop if any version is lower. If the node image lacks the required binary, use
a supported image or wait for availability.

### Install two ephemeral DACS CacheServers

The model weights are about 2.64 GiB, so the upstream quickstart's single 2 GiB
CacheServer is too small. This guide uses **two CacheServers at the default
2 GiB each** (4 GiB total). Keep the cache dedicated to this test: the model
and the quickstart's 1 GiB smoke-test data leave limited headroom, and
eviction can affect warm-read results.

Complete the [DACS AKS quickstart](https://github.com/Azure/dacs/blob/main/docs/quickstart.md)
unchanged through **Confirm success**, using its default namespace
`dacs-system` and cache name `cache-sample`. Once both smoke-test Jobs report
`All iterations succeeded: true`, clean up the resources created by the smoke
test:

```bash
kubectl delete job dacs-smoke-upload dacs-smoke-download \
  -n "$DACS_NAMESPACE" --ignore-not-found
```

Delete **only the smoke-test Jobs**. Don't run the rest of the quickstart's
cleanup section: this guide still uses the DACS Helm release, the
`cache-sample` cache, and the `dacs-system` namespace. Deleting the Jobs
doesn't evict their 1 GiB of test data from the cache.

Then scale the cache from one to two CacheServers:

```bash
kubectl patch cache "$DACS_CACHE" -n "$DACS_NAMESPACE" \
  --type=merge \
  -p '{"spec":{"scaling":{"static":{"numServers":2}}}}'

kubectl get cache "$DACS_CACHE" -n "$DACS_NAMESPACE" \
  -o jsonpath='{"servers="}{.spec.scaling.static.numServers}{" size-per-server-GiB="}{.spec.serverSizeInGB}{"\n"}'
kubectl wait statefulset/"$DACS_CACHE" -n "$DACS_NAMESPACE" \
  --for=jsonpath='{.status.readyReplicas}'=2 --timeout=10m
kubectl get pods -n "$DACS_NAMESPACE" \
  -l "dacs.azure.com/cache-name=$DACS_CACHE" -o wide
kubectl get service "$DACS_CACHE-discovery" -n "$DACS_NAMESPACE"
```

The patch updates the standalone `Cache` resource. Keep
the server count unchanged through the cold and warm reads in step 6.

**Checkpoint:** the configuration reports `servers=2 size-per-server-GiB=2`,
both CacheServers are Ready, the discovery service exists, the smoke test
passed, and its Jobs have been deleted.

## 4. Configure storage access and upload the sample

This walkthrough uses fixed Kubernetes names: namespace `blob-dacs-demo`,
PV `pv-blob-dacs-demo`, and PVC `pvc-blob-dacs-demo`.

### Grant the mount identity access

The recommended mount authentication is a managed identity, so no storage
account key is needed. This block looks up your cluster's kubelet identity
(already assigned to the reader nodes), grants it read-only data access to the
container unless it already has it, and saves the details for later steps and
cleanup. To use an account key instead, follow the
[fallback](#b-account-key-authentication-fallback) in place of this section.

```bash
(
set -eu
MOUNT_IDENTITY_CLIENT_ID=$(az aks show -g "$AKS_RESOURCE_GROUP" -n "$AKS_CLUSTER" \
  --query identityProfile.kubeletidentity.clientId -o tsv)
MOUNT_IDENTITY_OBJECT_ID=$(az aks show -g "$AKS_RESOURCE_GROUP" -n "$AKS_CLUSTER" \
  --query identityProfile.kubeletidentity.objectId -o tsv)
if [ -z "$MOUNT_IDENTITY_CLIENT_ID" ] || [ -z "$MOUNT_IDENTITY_OBJECT_ID" ]; then
  echo 'ERROR: no kubelet managed identity found; use the account-key fallback.' >&2
  exit 1
fi
CONTAINER_SCOPE="$(az storage account show --name "$STORAGE_ACCOUNT" \
  --query id -o tsv)/blobServices/default/containers/$STORAGE_CONTAINER"

# Record whether this guide created the assignment so cleanup removes only that.
ROLE_CREATED=false
# Capture the lookup on its own so a failure stops setup instead of being
# mistaken for "assignment already exists".
if ! EXISTING_ROLES=$(az role assignment list --assignee "$MOUNT_IDENTITY_OBJECT_ID" \
  --role 'Storage Blob Data Reader' --scope "$CONTAINER_SCOPE" \
  --query 'length(@)' -o tsv); then
  echo 'ERROR: role assignment lookup failed.' >&2
  exit 1
fi
if [ "$EXISTING_ROLES" = 0 ]; then
  az role assignment create \
    --assignee-object-id "$MOUNT_IDENTITY_OBJECT_ID" \
    --assignee-principal-type ServicePrincipal \
    --role 'Storage Blob Data Reader' \
    --scope "$CONTAINER_SCOPE" -o table
  ROLE_CREATED=true
else
  echo 'Role assignment already exists; leaving it in place.'
fi

declare -p MOUNT_IDENTITY_CLIENT_ID MOUNT_IDENTITY_OBJECT_ID CONTAINER_SCOPE \
  ROLE_CREATED >> "$DEMO_DIR/settings.env"
)
if [ "$?" -eq 0 ]; then
  source "$DEMO_DIR/settings.env"
  kubectl create namespace blob-dacs-demo
else
  echo 'ERROR: mount identity setup failed; do not continue.' >&2
fi
```

**Checkpoint:** `Storage Blob Data Reader` is assigned on the container scope
(created now or already present) and the namespace exists. Role assignments can
take several minutes to propagate; the upload below usually covers that time.

### Download and upload Phi-1.5

Review the model's [model card and MIT license](https://huggingface.co/microsoft/phi-1_5)
before use. The pinned revision and checksum guarantee both reads use identical
bytes.

```bash
hf download microsoft/phi-1_5 \
  --revision "$MODEL_REVISION" --local-dir "$MODEL_DIR"
printf '%s  %s\n' "$MODEL_SHA256" "$MODEL_DIR/model.safetensors" \
  | sha256sum --check

az storage blob upload-batch \
  --account-name "$STORAGE_ACCOUNT" \
  --auth-mode login \
  --destination "$STORAGE_CONTAINER" \
  --destination-path "$BLOB_PREFIX" \
  --source "$MODEL_DIR" \
  --pattern '[!.]*' \
  --overwrite false

printf 'Model path for both readers: %s\n' "$MODEL_PATH"
```

**Checkpoint:** the local checksum reports `OK` and the upload succeeds.
The upload pattern excludes hidden download metadata, such as `.cache`.

## 5. Create the read-only volume and both readers

### Create the PersistentVolume and claim

The block expands your saved configuration into the manifest, including the
discovery endpoint, cluster DNS IP, and mount identity from steps 2 and 4.
There are no additional placeholders to replace. If you use the
[account-key fallback](#b-account-key-authentication-fallback), make the
manifest change it describes before applying.

```bash
kubectl apply -f - <<YAML
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-blob-dacs-demo
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadOnlyMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""
  mountOptions:
    - -o allow_other
    - -o ro
    - --read-only=true
    - --distributed-cache-discovery-endpoint=$DACS_ENDPOINT
    - --distributed-cache-dns-server=$CLUSTER_DNS
  csi:
    driver: blob.csi.azure.com
    readOnly: true
    volumeHandle: "$STORAGE_ACCOUNT#$STORAGE_CONTAINER#blob-dacs-demo"
    volumeAttributes:
      protocol: fuse2
      storageAccount: "$STORAGE_ACCOUNT"
      containerName: "$STORAGE_CONTAINER"
      AzureStorageAuthType: MSI
      AzureStorageIdentityClientID: "$MOUNT_IDENTITY_CLIENT_ID"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-blob-dacs-demo
  namespace: blob-dacs-demo
spec:
  accessModes:
    - ReadOnlyMany
  storageClassName: ""
  volumeName: pv-blob-dacs-demo
  resources:
    requests:
      storage: 10Gi
YAML

kubectl wait pvc/pvc-blob-dacs-demo -n blob-dacs-demo \
  --for=jsonpath='{.status.phase}'=Bound --timeout=2m
```

The empty storage class and explicit `volumeName` select static binding. The
PV's capacity is for Kubernetes binding only; it does not size DACS or impose
a Blob quota. The discovery endpoint enables the distributed-cache pipeline, so
do not add file-cache or streaming options.

### Create readers on different nodes

First confirm the model blob exists. This uses your upload identity and reads
only metadata, so it does not warm DACS:

```bash
az storage blob show \
  --account-name "$STORAGE_ACCOUNT" \
  --container-name "$STORAGE_CONTAINER" \
  --name "$BLOB_PREFIX/model.safetensors" \
  --auth-mode login --query '{name:name,size:properties.contentLength}' -o table
```

If the lookup fails, don't create the readers. Check that the settings are
loaded and that the step 4 upload, your login, and your storage access are
correct.

The helper below creates both pods from the same manifest:

```bash
create_reader() {
  local name="$1" node="$2"
  kubectl apply -f - <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: $name
  namespace: blob-dacs-demo
spec:
  nodeSelector:
    kubernetes.io/os: linux
    kubernetes.azure.com/agentpool: "$NODE_POOL"
    kubernetes.io/hostname: "$node"
  containers:
    - name: reader
      image: busybox:1.36
      command: ["sh", "-c", "sleep 86400"]
      volumeMounts:
        - name: dataset
          mountPath: /mnt/blob
          readOnly: true
  volumes:
    - name: dataset
      persistentVolumeClaim:
        claimName: pvc-blob-dacs-demo
        readOnly: true
YAML
}

create_reader blob-dacs-reader "$COLD_NODE"
create_reader blob-dacs-reader-warm "$WARM_NODE"
kubectl wait pod/blob-dacs-reader pod/blob-dacs-reader-warm \
  -n blob-dacs-demo --for=condition=Ready --timeout=5m
kubectl get pods -n blob-dacs-demo -o wide
kubectl exec -n blob-dacs-demo blob-dacs-reader -- ls /mnt/blob
```

**Checkpoint:** the PVC is Bound and both pods are Ready on **different nodes**.
Listing directories is fine; **do not read the weights yet**.

## 6. Read the model and confirm DACS was used

The reader on the first node reads the model through the mount, which
populates the DACS cache (cold read). The reader on the second node then reads
the same file (warm read) and should be served from DACS. A successful mount
doesn't show whether DACS was used, so you'll check DACS request counters
before and after each read.

Run everything in the same shell with the step 2 settings loaded, because the
steps below keep values in shell variables. (In a new shell, source the
settings file and paste the helper again.) First define a small helper that
sums one DACS request counter across both CacheServers:

```bash
dacs_count() (   # usage: dacs_count <Download|Upload> <Success|InvalidTransition>
  set -euo pipefail
  pods=$(kubectl get pods -n "$DACS_NAMESPACE" \
    -l "dacs.azure.com/cache-name=$DACS_CACHE" -o name)
  [[ $(wc -w <<< "$pods") == 2 ]] || { echo "ERROR: expected two CacheServer pods." >&2; exit 1; }
  for pod in $pods; do
    kubectl exec -n "$DACS_NAMESPACE" "$pod" -c cacheserver -- \
      curl --fail --silent --show-error --max-time 10 http://localhost:9096/metrics || exit 1
  done | awk -v type="$1" -v status="$2" '
    /^cache_server_request_counter\{/ &&
      index($0, "request_type=\"" type "\"") && index($0, "status=\"" status "\"") { n += $2 }
    END { printf "%.0f\n", n }'
)
```

The counters are cumulative since server startup and cache-wide, so each step
records a baseline first and reports the increase. Keep the cache free of other
readers, or the increase can't be attributed to your test.

### Cold read: populate the cache

Record the current upload count, then read the model from the first node. The
read computes the file's checksum, which pulls every byte through the mount:

```bash
UPLOADS_BEFORE=$(dacs_count Upload Success)
if [ "$?" -ne 0 ]; then
  echo 'ERROR: could not read the DACS upload baseline; fix metrics access before retrying.' >&2
  exit 1
fi

kubectl exec -n blob-dacs-demo blob-dacs-reader -- \
  sh -c 'set -eu; sha256sum "$1"' sh "$MODEL_PATH" \
  | tee /dev/stderr | grep -q "^$MODEL_SHA256" \
  && echo 'Checksum OK' || echo 'Checksum MISMATCH'
```

Cache uploads are asynchronous. Check that DACS accepted them, waiting up to
5 minutes:

```bash
uploads=0
for i in $(seq 30); do
  now=$(dacs_count Upload Success) || break
  uploads=$((now - UPLOADS_BEFORE))
  (( uploads > 0 )) && break
  sleep 10
done
(( uploads > 0 )) && echo "PASS: DACS accepted $uploads upload requests." \
  || echo "INCONCLUSIVE: no uploads observed; see step 7."
```

**Checkpoint:** the checksum is OK and the result is `PASS`, which means the
cold read populated the cache.

### Warm read: reuse data from the second node

Record the current download count, then read the same blob from the second node:

```bash
DOWNLOADS_BEFORE=$(dacs_count Download Success)
if [ "$?" -ne 0 ]; then
  echo 'ERROR: could not read the DACS download baseline; fix metrics access before retrying.' >&2
  exit 1
fi

kubectl exec -n blob-dacs-demo blob-dacs-reader-warm -- \
  sh -c 'set -eu; sha256sum "$1"' sh "$MODEL_PATH" \
  | tee /dev/stderr | grep -q "^$MODEL_SHA256" \
  && echo 'Checksum OK' || echo 'Checksum MISMATCH'
```

Confirm that DACS served the read:

```bash
served=0
now=$(dacs_count Download Success) && served=$((now - DOWNLOADS_BEFORE))
(( served > 0 )) && echo "PASS: DACS served $served download requests." \
  || echo "INCONCLUSIVE: no downloads served by DACS; see step 7."
```

**Checkpoint:** the checksum is OK and the result is `PASS`, which means a
reader on a different node was served from the shared cache. Don't expect
100% of requests to be hits: eviction and incomplete population can cause
misses.

### What the counts mean

| Counter (`cache_server_request_counter`) | Meaning |
| --- | --- |
| `Upload`, `Success` | Cache population from a miss |
| `Download`, `Success` | Request served by DACS |
| `Download`, `InvalidTransition` | Not found in DACS (a miss) |

These mappings follow [DACS's scenario-test checks](https://github.com/Azure/dacs/blob/main/test/pkg/scenario/cache_metrics.go).
Counts are requests, not files or bytes, and a pass doesn't show that the whole
file is resident.

## 7. Troubleshoot

| Symptom | Next action |
| --- | --- |
| Pod stays Pending | Inspect pod events, PVC binding, hostname/pool labels, taints, and available node resources. |
| Mount fails | Check host BlobFuse2 version, `fuse2`, read-only settings, and storage network access. |
| Mount fails with authorization errors (managed identity) | Confirm the role assignment on the container scope and that `MOUNT_IDENTITY_CLIENT_ID` belongs to an identity assigned to the reader nodes. Allow time for role propagation. The [managed identity example](https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/deploy/example/blobfuse-mi/README.md) uses **Storage Blob Data Contributor**; try it if Reader isn't sufficient. |
| Mount fails (account-key fallback) | Check the Secret namespace and name, and the shared-key policy. |
| Discovery fails | Verify the discovery service and its endpoints, cluster DNS IP/domain, port 9065, and node-to-cache connectivity. |
| Helper cannot read metrics | Check that both CacheServers are Ready, `pods/exec` permissions, container `curl`, metrics port 9096, and the reported error. |
| `Checksum MISMATCH` | Confirm the step 4 upload succeeded and that `MODEL_PATH` points at the uploaded prefix. |
| `INCONCLUSIVE`, or misses remain high | [Recover from an inconclusive result](#recover-from-an-inconclusive-result). |
| `INCONCLUSIVE` and a CacheServer restarted | A restart resets its counters. Check restart counts with `kubectl get pods -n "$DACS_NAMESPACE"`, wait for stable pods, then [repeat the test](#repeat-the-test-from-the-start). |

### Check pod events, DACS health, and CSI logs

```bash
kubectl describe pod blob-dacs-reader -n blob-dacs-demo
kubectl describe pod blob-dacs-reader-warm -n blob-dacs-demo
kubectl get pvc -n blob-dacs-demo
kubectl get pods -n "$DACS_NAMESPACE" \
  -l "dacs.azure.com/cache-name=$DACS_CACHE" -o wide
kubectl logs -n "$DACS_NAMESPACE" \
  -l "dacs.azure.com/cache-name=$DACS_CACHE" \
  -c cacheserver --since=15m --tail=200
kubectl get pods -n kube-system -l app=csi-blob-node -o wide
```

Match the CSI pod's node to the reader's node, then inspect its `blob` container:

```bash
kubectl logs -n kube-system <csi-blob-node-pod-on-reader-node> \
  -c blob --since=15m
```

If the driver cannot verify distributed-cache support, it removes those mount
options and selects local block cache. A warning beginning
`BlobFuse2 distributed-cache support could not be verified` means that mount
is **not using DACS**. Correct the host binary or capability issue and remount.

### Check host BlobFuse2 and proxy diagnostics

BlobFuse2 2.5.6 can mount successfully without discovering any DACS servers and
fall back to Azure Storage, and CSI container logs are not host BlobFuse2 logs.
On the affected reader node, use your approved node-access method and inspect:

```bash
blobfuse2 -v
journalctl -u blobfuse-proxy --since "15 minutes ago" --no-pager
```

Also check `/var/log/blobfuse2.log` if that is the configured BlobFuse2 log
path, and `/var/log/syslog` for syslog-based logging. Log routing depends on
your installation; see the
[CSI debugging guide](https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/csi-debug.md).

### Recover from an inconclusive result

Don't repeatedly read the same warm mount: node-local caching can hide DACS
activity. Use this bounded procedure instead:

1. Inspect errors and discovery diagnostics using the checks above.
2. If a cold read finished before uploads appeared, keep its reader running,
   wait up to five minutes, and rerun the cold read's upload check. It reuses
   `UPLOADS_BEFORE`, so run it in the same shell.
3. If there are no upload errors and capacity is sufficient, retry the warm
   read once from a fresh mount on a supported node that has no active mount
   of this PV. Deleting a pod alone doesn't guarantee its node's CSI mount is
   gone, so stop the old warm reader and confirm the unmount in the CSI logs
   before recreating it with `create_reader` (re-paste its definition if you
   changed shells).
4. If hits are still absent or misses remain high, stop and investigate cache
   capacity, discovery, and upload failures rather than declaring success.

### Repeat the test from the start

A cold read only works once per blob: afterwards DACS already holds the data,
so a repeated cold read can't show new uploads. To run the cold and warm test
again, for example after a CacheServer restart:

1. Run the first block of [step 8](#8-clean-up-evaluation-resources) to delete
   the Kubernetes resources, whose names are fixed.
2. Rerun the step 2 configuration block, which generates a new working
   directory and blob prefix.
3. Repeat steps 4 through 6. Step 4 detects the existing role assignment and
   leaves it in place; to remove it later, use the first run's `settings.env`,
   which recorded that this guide created it.

## 8. Clean up evaluation resources

Delete only resources created for this walkthrough. Stop the reader pods before
deleting the claim:

```bash
kubectl delete pod blob-dacs-reader blob-dacs-reader-warm -n blob-dacs-demo
kubectl delete pvc pvc-blob-dacs-demo -n blob-dacs-demo
kubectl delete pv pv-blob-dacs-demo
# Only exists if you used the account-key fallback.
kubectl delete secret azure-secret -n blob-dacs-demo --ignore-not-found
kubectl delete namespace blob-dacs-demo
```

Then clean up what remains, as applicable:

- **Mount identity role assignment** (managed identity path). Remove it only
  if this guide created it, which step 4 recorded in `ROLE_CREATED`:

  ```bash
  if [ "${ROLE_CREATED:-false}" = true ]; then
    az role assignment delete \
      --assignee "$MOUNT_IDENTITY_OBJECT_ID" \
      --role 'Storage Blob Data Reader' \
      --scope "$CONTAINER_SCOPE"
  fi
  ```

- **Blob data and local files.** The PV uses `Retain`, so the commands above
  do **not** delete your container or blobs. Review the uploaded prefix, then
  delete only that prefix with your normal storage-management tools. Remove
  the local model download and settings when you no longer need them. Don't
  delete a shared container, other model versions, or unrelated local files.

  ```bash
  printf 'Retained Blob prefix: %s/%s/%s\n' \
    "$STORAGE_ACCOUNT" "$STORAGE_CONTAINER" "$BLOB_PREFIX"
  printf 'Retained local working directory: %s\n' "$DEMO_DIR"
  ```

- **DACS**, if you installed it only for this evaluation. Delete the Cache
  resource, then follow the
  [upstream quickstart cleanup](https://github.com/Azure/dacs/blob/main/docs/quickstart.md#clean-up)
  for the Helm release and its namespace. The smoke-test Jobs were already
  deleted in step 3.

  ```bash
  kubectl delete cache "$DACS_CACHE" -n "$DACS_NAMESPACE"
  ```

- **Blob CSI driver.** Leave the managed add-on enabled. For the open-source
  driver, see [its cleanup](#a-open-source-blob-csi-driver).

## Appendix: alternatives

### A. Open-source Blob CSI driver

Use a cluster where the managed Blob CSI add-on is **not enabled**, and never
install this Helm release alongside the managed add-on. Review the
[open-source driver's identity prerequisites](https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/install-driver-on-aks.md)
and
[Helm installation guidance](https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/charts/README.md)
first. If an open-source driver already exists, verify its version and
configuration instead of installing a second release.

Install driver **1.27.11**, explicitly enabling installation of BlobFuse2
**2.5.6** on the nodes:

```bash
helm repo add blob-csi-driver https://kubernetes-sigs.github.io/blob-csi-driver
helm repo update blob-csi-driver
helm install blob-csi-driver blob-csi-driver/blob-csi-driver \
  --namespace kube-system \
  --version 1.27.11 \
  --set node.enableBlobfuseProxy=true \
  --set node.blobfuseProxy.installBlobfuse2=true \
  --set-string node.blobfuseProxy.blobfuse2Version=2.5.6 \
  --wait --timeout 10m
```

These values configure the `install-blobfuse-proxy` init container to install
the pinned binary, independent of the version bundled in the node image.
Return to [step 3](#verify-the-aks-managed-blob-csi-add-on) to verify the
driver, then continue with the same DACS, storage, volume, and reader steps.

After step 8, uninstall this Helm release **only if you installed it solely for
this evaluation and no other workloads use it**:

```bash
helm uninstall blob-csi-driver -n kube-system
```

### B. Account-key authentication (fallback)

Use this only if you can't use a managed identity for the mount. It requires
shared-key authentication enabled on the storage account, and the key grants
full account access even though the mount is read-only.

Instead of granting the mount identity access in step 4, create the namespace
and a Secret. Enter your storage account key at the prompt; do not paste it
into this document or the saved settings file, and do not enable shell
tracing. The key is passed to the local `kubectl` process; follow your
organization's credential-handling requirements.

```bash
read -r -s -p 'Storage account key: ' STORAGE_KEY
printf '\n'
test -n "$STORAGE_KEY" || echo 'ERROR: no key entered.'
AZURE_STORAGE_KEY="$STORAGE_KEY" az storage container show \
  --account-name "$STORAGE_ACCOUNT" \
  --name "$STORAGE_CONTAINER" --auth-mode key -o table
kubectl create namespace blob-dacs-demo
kubectl create secret generic azure-secret \
  --namespace blob-dacs-demo \
  --from-literal=azurestorageaccountname="$STORAGE_ACCOUNT" \
  --from-literal=azurestorageaccountkey="$STORAGE_KEY"
unset STORAGE_KEY
```

**Checkpoint:** the account-key container lookup succeeds before any model
transfer. If it fails, check the key, shared-key policy, and storage network
access, and do not continue to the download.

Then, in the step 5 PersistentVolume, replace the two `AzureStorage*`
attributes with a reference to the Secret, keeping `protocol: fuse2`, all
read-only settings, and the distributed-cache mount options:

```yaml
    volumeAttributes:
      protocol: fuse2
      storageAccount: "$STORAGE_ACCOUNT"
      containerName: "$STORAGE_CONTAINER"
    nodeStageSecretRef:
      name: azure-secret
      namespace: blob-dacs-demo
```

Step 8 deletes the Secret; there is no role assignment to clean up.

## References

- [DACS source repository](https://github.com/Azure/dacs)
- [DACS AKS quickstart](https://github.com/Azure/dacs/blob/main/docs/quickstart.md)
- [DACS deployment user guide](https://github.com/Azure/dacs/blob/main/user-guide.md)
- [BlobFuse2 2.5.6 release notes](https://github.com/Azure/azure-storage-fuse/releases/tag/blobfuse2-2.5.6)
- [Blob CSI driver parameters](https://github.com/kubernetes-sigs/blob-csi-driver/blob/master/docs/driver-parameters.md)
- [Microsoft Phi-1.5 model](https://huggingface.co/microsoft/phi-1_5)
