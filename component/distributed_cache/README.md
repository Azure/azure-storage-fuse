# Distributed Cache Setup for Blobfuse2

## Overview

Blobfuse2 supports a distributed cache mode, enabling multiple nodes to share a cache and provide high availability and scalability for large-scale workloads. This is achieved by running Blobfuse2 in a cluster, where each node contributes local storage to a distributed cache pool.

---

## Prerequisites

- **Blobfuse2** built and installed on all cluster nodes.
- Each node must have a dedicated local cache directory (preferably on fast SSD/NVMe).
- All nodes must have access to the same Azure Storage account/container.

---

## Distributed Cache Configuration

A sample configuration for distributed cache (YAML):

- [config.yaml](../../sampleDistributedCacheConfig.yaml)

**Key Parameters:**
- `cache-id`: Unique identifier for your cache cluster.
- `cache-dirs`: List of local directories to use for cache storage (one or more per node).

---

## Running Blobfuse2 in Cluster Mode

### 1. Prepare Each Node

- Place the config file (as above) on each node, defining `cache-dirs` as needed.
- Ensure the cache directory exists and is writable by the Blobfuse2 process.

### 2. Start Blobfuse2

On each node, run:

```bash
blobfuse2 mount <mount_path> --config-file=<path_to_config.yaml>
```

- `<mount_path>`: Local mount point for the Azure container.
- `<path_to_config.yaml>`: Path to your distributed cache config.

### 3. Cluster Health & Validation

- To check cluster state, you can inspect the debug filesystem:
  ```bash
  cat <mount_path>/fs=debug/clustermap
  ```
  This will show the current cluster map, node states, and health.

---

## Example: 3-Node Cluster

Suppose you have three VMs: `vm1`, `vm2`, `vm3`.

- On each VM, set `cache-dirs` to a unique local path (e.g., `/mnt/cache`).
- Use the same `cache-id` and Azure storage credentials on all nodes.
- Start Blobfuse2 on each node as described above.

The cluster becomes writable once at least `min-nodes` nodes (default: 3) are online. The distributed cache will automatically handle data replication and failover.

---


## Setup on Azure Kubernetes Service (AKS)

### 1) Create an AKS Cluster

- Create an AKS cluster using Azure CLI or Portal (ensure nodes have sufficient local SSD/NVMe for cache directories if using hostPath or ephemeral storage).

Example with Azure CLI (adjust as needed):
```bash
az group create -n <rg-name> -l <region>
az aks create -g <rg-name> -n <cluster-name> --node-count 3 --generate-ssh-keys
az aks get-credentials -g <rg-name> -n <cluster-name>
```

### 2) Install the open-source Azure Blob CSI Driver via Helm

- Ensure Helm is installed: https://helm.sh/docs/intro/install/
- Add the chart repo and install the driver with blobfuse proxy support enabled.

```bash
helm repo add blob-csi-driver https://raw.githubusercontent.com/kubernetes-sigs/blob-csi-driver/master/charts
helm install blob-csi-driver blob-csi-driver/blob-csi-driver \
  --set node.enableBlobfuseProxy=true \
  --namespace kube-system \
  --version 1.26.7
```

Note: Use a tested chart version in your environment. Newer "latest" versions may change behavior.

### 3) Verify CSI Driver Pods

```bash
kubectl --namespace=kube-system get pods --selector="app.kubernetes.io/name=blob-csi-driver" --watch
```

- Keep watching until all `blob-csi-driver` node pods are in `Running` state.

> Next steps (not shown here): create a `StorageClass` and `PersistentVolumeClaim` referencing the Azure Storage account/container, and deploy a DaemonSet/Deployment that mounts the volume and runs Blobfuse2 with the distributed cache config per node.

### 4) Create StorageClass and PVC

Create a StorageClass (adjust parameters as needed):

```yaml
# storageclass-blobfuse-existing-container.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: blob-fuse
provisioner: blob.csi.azure.com
parameters:
  skuName: Premium_LRS
  # You can also add storage account details here itself if you don't want to provide in PV
reclaimPolicy: Retain  # If set as "Delete" container would be removed after PVC deletion
volumeBindingMode: Immediate
```

Apply the StorageClass:

```bash
kubectl create -f storageclass-blobfuse-existing-container.yaml
```

Create a PVC (example from upstream):

```bash
kubectl create -f https://raw.githubusercontent.com/kubernetes-sigs/blob-csi-driver/master/deploy/example/pvc-blob-csi.yaml
```

Or define your own PVC bound to the StorageClass above:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-blob
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: blob-fuse
  resources:
    requests:
      storage: 100Gi
```

### 5) Sample Deployment (Demo) using Distributed Cache

The example below demonstrates a simple Deployment that:
- Mounts a PVC at `/mnt/blob` (backed by the Azure Blob CSI driver)

Create a Deployment that mounts the PVC and runs Blobfuse2:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: blobfuse2-dcache-demo
spec:
  replicas: 3
  selector:
    matchLabels:
      app: blobfuse2-dcache-demo
  template:
    metadata:
      labels:
        app: blobfuse2-dcache-demo
    spec:
      containers:
        - name: blobfuse2
          image: mcr.microsoft.com/mirror/docker/library/nginx:1.23
          volumeMounts:
            - name: blob-pvc
              mountPath: /mnt/blob
          command:
            - "/bin/bash"
            - "-c"
            - set -euo pipefail; while true; do echo $(date) >> /mnt/blob/outfile; sleep 1; done
      volumes:
        - name: blob-pvc
          persistentVolumeClaim:
            claimName: pvc-blob  # ensure this matches your PVC name
```

Notes:
- Ensure the PVC name used in the Deployment (`pvc-blob`) matches the PVC created earlier.

---


### 4.1) Create a PersistentVolume (optional, static PV)

If you prefer a static PV (instead of only dynamic provisioning via StorageClass), create a PV referencing the Azure Blob CSI driver and Blobfuse options:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-blob
spec:
  capacity:
    storage: 100Gi
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Delete
  storageClassName: blob-fuse
  mountOptions:
    - --log-level=LOG_DEBUG
    - --dcache
    - --cache-id=AKS
    - --cache-dirs=/mnt/cacheDir # cache directory path inside node
    - -o direct_io
  csi:
    driver: blob.csi.azure.com
    readOnly: false
    volumeHandle: <volumeHandleName>
    volumeAttributes:
      resourceGroup: <resourceGroupName>
      storageAccount: <storageAccountName>
      containerName: <containerName>
      # refer to https://github.com/Azure/azure-storage-fuse#environment-variables
      AzureStorageAuthType: msi # key, sas, msi, spn
      AzureStorageIdentityClientID: <client_id>
      protocol: fuse2
```

Apply the PV:

```bash
kubectl apply -f pv-blob.yaml
```

Notes:
- Ensure `storageClassName` aligns with your StorageClass if you intend to bind via PVC selector.
- `mountOptions` includes Blobfuse flags; adjust for your environment.
- For managed identity, confirm the identity has access to the storage account.

---

## Full Cluster Setup (Quick Checklist)

1) Create AKS cluster and get kubeconfig.
2) Install Blob CSI driver via Helm with `node.enableBlobfuseProxy=true`.
3) Create StorageClass (and optionally a static PV).
4) Create PVC (bound to SC or PV).
5) Deploy your workload (Deployment/DaemonSet) mounting the PVC.
6) Verify pods are Running and data I/O on the mounted path.

---


## Tips & Best Practices

- Use fast, dedicated disks for cache directories.
- Monitor logs and cluster health regularly.
- For production, ensure all nodes have synchronized clocks and reliable networking.
- Adjust `replicas` and `min-nodes` for your desired redundancy and availability.

---

## References

- [Sample Distributed Cache Config](../../sampleDistributedCacheConfig.yaml)
- [Cluster Test script](../../test/distributed_cache/test-cluster.sh)
- [Main Blobfuse2 README](../../README.md)
- [Blob CSI Driver Documentation](https://github.com/kubernetes-sigs/blob-csi-driver)

---

