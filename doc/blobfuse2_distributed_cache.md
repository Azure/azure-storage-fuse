# Distributed Cache Setup for Blobfuse2 (Preview)

## Overview

Blobfuse2 supports newly added distributed cache mode, enabling multiple nodes to share their local cache and provide high availability and scalability for large-scale workloads. This is achieved by running Blobfuse2 in cluster, where each node contributes their local storage towards the distributed cache pool.

---

## Prerequisites

- **Blobfuse2** built and installed on all nodes in the cluster.
- Each node must have a dedicated local cache directory (preferably on fast SSD/NVMe).
- All nodes must have access to the same Azure Storage account and container.
- All nodes should be present in the same vnet.

---

## Install Blobfuse2 on all nodes in cluster

On each node (if running Ubuntu 22.04), run the following to install Blobfuse2-preview binary and its dependencies:

```bash
wget https://packages.microsoft.com/config/ubuntu/22.04/packages-microsoft-prod.deb
dpkg -i packages-microsoft-prod.deb
apt-get update
apt-get install -y fuse3 blobfuse2-preview
```

For other distros, please refer [this](https://github.com/Azure/azure-storage-fuse/wiki/Blobfuse2-Installation) documentation for installing Blobfuse2.

---

## Distributed Cache Configuration

Sample configuration for distributed cache:

- [config.yaml](../setup/sampleDistributedCacheConfig.yaml)

**Required Parameters:**
- `cache-id`: Unique identifier for your cache cluster.
- `cache-dirs`: List of local directories to use for cache storage (one or more per node).

---

## Running Blobfuse2 in Cluster Mode

### 1. Prepare Each Node

- Place the config file (as above) on each node, defining `cache-dirs` as needed.
- Make sure that the `cache-id` in the config file in each node is same.
- Ensure the cache directory exists and is writable by the Blobfuse2 process.

### 2. Start Blobfuse2

On each node, run:

```bash
blobfuse2 mount <mount_path> --config-file=<path_to_config.yaml>
```

- `<mount_path>`: Local mount point for the Azure container.
- `<path_to_config.yaml>`: Path to your distributed cache config.

### 3. Cluster Health & Validation

- To check cluster state, you can inspect using the debug namespace. 
  ```bash
  cat <mount_path>/fs=debug/clustermap
  ```
  This will show the current cluster map, node states, and health.

- To check the various stats on the dcache.
  ```bash
  cat <mount_path>/fs=debug/stats
  ```

- To collect logs from all nodes in the cluster.
  ```bash
  cat <mount_path>/fs=debug/logs
  ```
  By default the latest log file is fetched from all nodes and is stored in the default working directory ($HOME/.blobfuse2 by default).

- To get the cluster-summary like the health of nodes, RVs, MVs, etc.
  ```bash
  cat <mount_path>/fs=debug/cluster-summary
  ```

- To get the stats of individual nodes.
  ```bash
  cat <mount_path>/fs=debug/nodes-stats
  ```

---

## Example: 3-Node Cluster

Suppose you have three VMs: `vm1`, `vm2`, `vm3`.

- On each VM, set `cache-dirs` to a unique local path (e.g., `/mnt/cache`).
- Use the same `cache-id` and Azure storage credentials on all nodes.
- Start Blobfuse2 on each node as described above.

The cluster becomes writable once at least `min-nodes` nodes (default: 3) are online. The distributed cache will automatically handle data replication and failover.

---


## Setup on Azure Kubernetes Service (AKS)

### 1) Direct Linux Mount

#### 1) Create an AKS Cluster

- Create an AKS cluster using Azure CLI or Portal (ensure nodes have sufficient local SSD/NVMe for cache directories if using hostPath or ephemeral storage).

Example with Azure CLI (adjust as needed):
```bash
az group create -n <rg-name> -l <region>
az aks create -g <rg-name> -n <cluster-name> --node-count 3 --generate-ssh-keys
az aks get-credentials -g <rg-name> -n <cluster-name>
```

#### 2) Install the blobfuse2 using daemonset

Create a Daemonset (adjust parameters as needed):

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: blobfuse-installer
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: blobfuse-installer
  template:
    metadata:
      labels:
        name: blobfuse-installer
    spec:
      containers:
      - name: installer
        image: ubuntu:22.04
        imagePullPolicy: IfNotPresent
        securityContext:
          privileged: true
        command:
        - /bin/bash
        - -c
        - |
          set -eux

          # Enable Microsoft repo
          apt-get update
          apt-get install -y wget apt-transport-https gnupg
          wget https://packages.microsoft.com/config/ubuntu/22.04/packages-microsoft-prod.deb
          dpkg -i packages-microsoft-prod.deb
          apt-get update

          # Remove old version if present on host
          chroot /host apt-get remove -y blobfuse2 || true

          # Install preview build in the container
          apt-get install -y fuse3 blobfuse2-preview
          dpkg -L blobfuse2-preview

          # Copy the new blobfuse2 binary onto the host
          cp /usr/bin/blobfuse2 /host/usr/bin/blobfuse2

          # Prepare mount points and config on host
          mkdir -p /host/mnt/blobfuseGlobal /host/mnt/blobfuseTmp /host/etc/blobfuse2
          cp /etc/blobfuse2/blobfuse2.yaml /host/etc/blobfuse2/blobfuse2.yaml || true
          chmod 777 /host/mnt/blobfuseGlobal /host/mnt/blobfuseTmp

          echo "Mounting blobfuse2..."
          nsenter --target 1 --mount --uts --ipc --net --pid  /usr/bin/blobfuse2 mount /mnt/blobfuseGlobal  --config-file=/etc/blobfuse2/blobfuse2.yaml -o allow_other &

          #chroot /host /usr/bin/blobfuse2 mount /mnt/blobfuseGlobal --config-file=/etc/blobfuse2/blobfuse2.yaml -o allow_other &

          echo "Blobfuse mounting completed, sleeping..."
          sleep infinity

        volumeMounts:
        - name: host-mount
          mountPath: /host/mnt
        - name: host-bin
          mountPath: /host/usr/bin
        - name: host-etc
          mountPath: /host/etc/blobfuse2
        - name: blobfuse2-config
          mountPath: /etc/blobfuse2
          readOnly: true
      hostNetwork: true
      hostPID: true
      tolerations:
      - operator: Exists
      volumes:
      - name: host-mount
        hostPath:
          path: /mnt
      - name: host-bin
        hostPath:
          path: /usr/bin
      - name: host-etc
        hostPath:
          path: /etc/blobfuse2
          type: DirectoryOrCreate
      - name: blobfuse2-config
        secret:
          secretName: blobfuse2-config
```


#### 3) Sample Deployment (Demo) using Distributed Cache

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: blobfuse-consumer
spec:
  replicas: 2
  selector:
    matchLabels:
      app: blobfuse-consumer
  template:
    metadata:
      labels:
        app: blobfuse-consumer
    spec:
      containers:
      - name: app
        image: azstortest.azurecr.io/mittas/fio:custom-3.28
        imagePullPolicy: Always
        command: ["/bin/bash"]
        args: ["-c", "while true ;do sleep 50; done"]

        #image: mcr.microsoft.com/ubuntu:22.04
        #command: ["sleep", "infinity"]
        volumeMounts:
        - name: blobfuse-volume
          mountPath: /mnt/blob-data   # This is where the pod will see the blobfuse data
      volumes:
      - name: blobfuse-volume
        hostPath:
          path: /mnt/blobfuseGlobal   # This must match the path used on the host
          type: Directory
```

### 2) Through CSI Driver

#### 1) Create an AKS Cluster

- Create an AKS cluster using Azure CLI or Portal (ensure nodes have sufficient local SSD/NVMe for cache directories if using hostPath or ephemeral storage).

Example with Azure CLI (adjust as needed):
```bash
az group create -n <rg-name> -l <region>
az aks create -g <rg-name> -n <cluster-name> --node-count 3 --generate-ssh-keys
az aks get-credentials -g <rg-name> -n <cluster-name>
```

#### 2) Install the open-source Azure Blob CSI Driver via Helm

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

#### 3) Verify CSI Driver Pods

```bash
kubectl --namespace=kube-system get pods --selector="app.kubernetes.io/name=blob-csi-driver" --watch
```

- Keep watching until all `blob-csi-driver` node pods are in `Running` state.

> Next steps (not shown here): create a `StorageClass` and `PersistentVolumeClaim` referencing the Azure Storage account/container, and deploy a DaemonSet/Deployment that mounts the volume and runs Blobfuse2 with the distributed cache config per node.

#### 4) Create StorageClass and PVC

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

#### 5) Sample Deployment (Demo) using Distributed Cache

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


#### 4.1) Create a PersistentVolume (optional, static PV)

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

- [Sample Distributed Cache Config](../setup/sampleDistributedCacheConfig.yaml)
- [Cluster Test script](../test/distributed_cache/test-cluster.sh)
- [Blob CSI Driver Documentation](https://github.com/kubernetes-sigs/blob-csi-driver)

---

