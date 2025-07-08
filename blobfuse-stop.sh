#!/bin/bash

echo "`hostname` : Executing on `hostname`"

echo "`hostname` : Movde to shared path"
cd /shared/home/blobfuse
pwd

MOUNT_PATH=/mnt/blob_mnt

# Unmount blobfuse if any 
if  mountpoint -q $MOUNT_PATH; then
	echo "`hostname` : Unmounting..."
	./blobfuse2 unmount $MOUNT_PATH
	sleep 1
fi


