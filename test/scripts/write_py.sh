#!/bin/bash

#"USAGE: ./test/scripts/write_py.sh true true read write "

blockCache=$1
NFS=$2
read=$3
write=$4

echo "Tool Size OpenTime WriteTime CloseTime TotalTime WriteSpeed AverageSpeed" > ./results_write.csv

sudo mkdir /tmp/mntdir /tmp/mntdirnfs /tmp/tempcache
sudo chmod 777 /tmp/mntdir /tmp/mntdirnfs /tmp/tempcache

echo "Cleaning with Blobfuse2"
sudo ./blobfuse2 unmount all
sudo ./blobfuse2 unmount all
sleep 2
sudo ./blobfuse2 /tmp/mntdir --config-file=/home/blobfuse/code/azure-storage-fuse/block_cache.yaml
sleep 3
sudo rm -rf /tmp/mntdir/pythonWrite_*

echo "Cleaning with NFSv3"
sudo umount -fl /tmp/mntdirnfs
sleep 2
sudo mount -t aznfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 blobfuseperftestnfs.blob.core.windows.net:/blobfuseperftestnfs/nfs-test /tmp/mntdirnfs
sudo sh -c 'echo 16384 > /sys/class/bdi/0:$(stat -c "%d" /tmp/mntdirnfs)/read_ahead_kb'
sleep 3
sudo rm -rf /tmp/mntdirnfs/pythonWrite_*

fileList="8 10 50 100 1024 8192 10240 51200 102400"

if [ $write == "write" ]
then
    for i in $fileList
    do
        if [ $blockCache == "true" ]
        then
            echo "Running with Blobfuse2 for " $i " GB file size WRITE"
            sudo python3 ./test/scripts/write.py /tmp/mntdir $i Blobfuse2 >> ./results_write.csv
        fi

        if [ $NFS == "true" ]
        then    
            echo "Running with NFSv3 for " $i " GB file size WRITE"
            sudo python3 ./test/scripts/write.py /tmp/mntdirnfs $i NFSv3 >> ./results_write.csv
        fi
    done
else 
    echo "Not running WRITE tests for Blobfuse and NFS"
fi

if [ $read = "read" ]
then
    for j in {1..3}
    do
        sudo ./blobfuse2 unmount all
        sudo ./blobfuse2 unmount all
        sleep 2
        sudo ./blobfuse2 /tmp/mntdir --config-file=/home/blobfuse/code/azure-storage-fuse/block_cache.yaml
        sleep 3

        sudo umount -fl /tmp/mntdirnfs
        sleep 2
        sudo mount -t aznfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 blobfuseperftestnfs.blob.core.windows.net:/blobfuseperftestnfs/nfs-test /tmp/mntdirnfs
        sudo sh -c 'echo 16384 > /sys/class/bdi/0:$(stat -c "%d" /tmp/mntdirnfs)/read_ahead_kb'
        sleep 3

        echo "Running $j th iteration for READ"
        echo "Tool Size OpenTime ReadTime CloseTime TotalTime ReadSpeed AverageSpeed" > ./results_read_${j}.csv

        for i in $fileList
        do
            if [ $blockCache == "true" ]
            then
                echo "Running with Blobfuse2 for " $i " GB file size READ"
                sudo python3 ./test/scripts/read.py /tmp/mntdir $i Blobfuse2 >> ./results_read_${j}.csv
            fi

            if [ $NFS == "true" ]
            then    
                echo "Running with NFSv3 for " $i " GB file size READ"
                sudo python3 ./test/scripts/read.py /tmp/mntdirnfs $i NFSv3 >> ./results_read_${j}.csv
            fi
        done
    done
else 
    echo "Not running READ tests for Blobfuse and NFS"
fi