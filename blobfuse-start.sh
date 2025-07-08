#!/bin/bash

echo "`hostname` : Executing on `hostname`"

echo "`hostname` : Movde to shared path"
cd /shared/home/blobfuse
pwd

MOUNT_PATH=/mnt/blob_mnt
TEMP_PATH=/mnt/blobfusetmp
LOG_PATH=/mnt/blobfuse2.log

# Create mount path if does not exists
if [ ! -d "$MOUNT_PATH" ]; then
	echo "`hostname` : Creating mount path"
	sudo mkdir $MOUNT_PATH
	sudo chmod 777 $MOUNT_PATH
else
	echo "`hostname` : Mount path already present"
fi

# Create temp cache path if does not exists
if [ ! -d "$TEMP_PATH" ]; then
	echo "`hostname` : Creating temp path"
	sudo mkdir $TEMP_PATH
	sudo chmod 777 $TEMP_PATH
else
	echo "`hostname` : temp path already present"
fi

# Cleanup last runs
if [ -e "$LOG_PATH" ]; then
	echo "`hostname` : Deleting log file"
	sudo rm -rf $LOG_PATH
else
	echo "`hostname` : No old logs"
fi

echo "`hostname` : Creating log file stub"
sudo touch $LOG_PATH
sudo chmod 777 $LOG_PATH

echo "`hostname` : Check for old mounts"
# Unmount blobfuse if any 
if  mountpoint -q $MOUNT_PATH; then
	echo "`hostname` : Unmount old mounts"
	./blobfuse2 unmount $MOUNT_PATH
	sleep 1
fi

# Crate ramdisk for temp-path
echo "`hostname` : Creating ramdisk"
sudo mount -t tmpfs -o rw,size=150G tmpfs $TEMP_PATH

echo "`hostname` : Mount now"
nohup setsid ./blobfuse2 $MOUNT_PATH --tmp-path=$TEMP_PATH --log-file-path=$LOG_PATH --log-level=LOG_DEBUG --log-type=base --cleanup-on-start

echo "`hostname` : Wait for mount to stablize"
sleep 5

df -h

if  mountpoint -q $MOUNT_PATH; then
	echo "`hostname` : Mount successful, listing mount"
	ls -l $MOUNT_PATH
fi

echo "Writing files" 
dd if=/dev/zero of=$MOUNT_PATH/`hostname`.txt bs=1M count=4
echo "File write complete"
ls -l $MOUNT_PATH

echo "`hostname` : Exiting"
