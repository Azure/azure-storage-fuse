## 1. install flex volume driver on every linux agent node
```
sudo apt install jq -y

sudo mkdir -p /etc/kubernetes/volumeplugins/azure~blobfuse/bin
cd /etc/kubernetes/volumeplugins/azure~blobfuse/bin
sudo wget -O blobfuse https://raw.githubusercontent.com/andyzhangx/Demo/master/linux/flexvolume/blobfuse/binary/ubuntu1604-4.4.0-104-generic/blobfuse
sudo chmod a+x blobfuse

cd /etc/kubernetes/volumeplugins/azure~blobfuse
sudo wget -O blobfuse https://raw.githubusercontent.com/andyzhangx/Demo/master/linux/flexvolume/blobfuse/blobfuse
sudo chmod a+x blobfuse
```
#### Note:
Make sure `jq` package is installed on every node.

## 2. specify `volume-plugin-dir` in kubelet service config (skip this step from acs-engine v0.12.0)
```
sudo vi /etc/systemd/system/kubelet.service
  --volume=/etc/kubernetes/volumeplugins:/etc/kubernetes/volumeplugins:rw \
        --volume-plugin-dir=/etc/kubernetes/volumeplugins \
sudo systemctl daemon-reload
sudo systemctl restart kubelet
```

Note:
1. `/etc/kubernetes/volumeplugins` has already been the default flexvolume plugin directory in acs-engine (starting from v0.12.0)
2. There would be one line of [kubelet log](https://github.com/andyzhangx/Demo/tree/master/debug#q-how-to-get-k8s-kubelet-logs-on-linux-agent) like below showing that `flexvolume-azure/blobfuse` is loaded correctly
```
I0122 08:24:47.761479    2963 plugins.go:469] Loaded volume plugin "flexvolume-azure/blobfuse"
```

## 3. create a secret which stores blobfuse account name and password
```
kubectl create secret generic blobfusecreds --from-literal username=USERNAME --from-literal password="PASSWORD" --type="azure/blobfuse"
```

## 4. create a pod with flexvolume blobfuse mount driver on linux
 - download `nginx-flex-blobfuse.yaml` file and modify `container` field
```
wget -O nginx-flex-blobfuse.yaml https://raw.githubusercontent.com/andyzhangx/Demo/master/linux/flexvolume/blobfuse/nginx-flex-blobfuse.yaml
vi nginx-flex-blobfuse.yaml
```
 - create a pod with flexvolume blobfuse driver mount
```
kubectl create -f nginx-flex-blobfuse.yaml
```

#### watch the status of pod until its Status changed from `Pending` to `Running`
watch kubectl describe po nginx-flex-blobfuse

## 5. enter the pod container to do validation
kubectl exec -it nginx-flex-blobfuse -- bash

```
root@nginx-flex-blobfuse:/# df -h
Filesystem      Size  Used Avail Use% Mounted on
overlay          30G  5.5G   24G  19% /
tmpfs           3.4G     0  3.4G   0% /dev
tmpfs           3.4G     0  3.4G   0% /sys/fs/cgroup
blobfuse         30G  5.5G   24G  19% /data
/dev/sda1        30G  5.5G   24G  19% /etc/hosts
shm              64M     0   64M   0% /dev/shm
tmpfs           3.4G   12K  3.4G   1% /run/secrets/kubernetes.io/serviceaccount
```

### about this blobfuse flexvolume driver usage
1. You will get following error if you don't specify your secret type as driver name `blobfuse/blobfuse`
```
MountVolume.SetUp failed for volume "azure" : Couldn't get secret default/azure-secret
```

### Links
[azure-storage-fuse](https://github.com/Azure/azure-storage-fuse)

[Flexvolume doc](https://github.com/kubernetes/community/blob/master/contributors/devel/flexvolume.md)

More clear steps about flexvolume by Redhat doc: [Persistent Storage Using FlexVolume Plug-ins](https://docs.openshift.org/latest/install_config/persistent_storage/persistent_storage_flex_volume.html)
