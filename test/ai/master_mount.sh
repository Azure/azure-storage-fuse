export AZURE_STORAGE_ACCOUNT=blobfuseaitest2
#export AZURE_STORAGE_ACCESS_KEY=<key>
export AZURE_STORAGE_AUTH_TYPE="msi"

# Base directory for entire container mount
MOUNT_PATH=/mnt/blobfuse/mnt

# Unmount and mount again
blobfuse2 unmount $MOUNT_PATH
sleep 3

# Unmount Ramdisk and recreate it
sudo umount /mnt/ramdisk
sudo mkdir /mnt/ramdisk
sudo chmod 777 /mnt/ramdisk
sudo mount -t tmpfs -o rw,size=200G tmpfs /mnt/ramdisk

sudo rm -rf $MOUNT_PATH/*

blobfuse2 mount $MOUNT_PATH --tmp-path=/mnt/ramdisk --container-name="models" --block-cache --log-type base --log-file-path=./master_blobfuse2.log
sleep 5






