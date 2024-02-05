#!/bin/bash

echo "Tool Size OpenTime WriteTime CloseTime TotalTime WriteSpeed AverageSpeed" > ./results_write.csv

sudo mkdir /tmp/mntdir /tmp/mntdirnfs /tmp/tempcache
sudo chmod 777 /tmp/mntdir /tmp/mntdirnfs /tmp/tempcache

echo "Cleaning with Blobfuse2"
sudo ./test/scripts/blobfuse2 unmount all
sudo ./test/scripts/blobfuse2 unmount all
sleep 2
sudo ./test/scripts/blobfuse2 /tmp/mntdir --config-file=/home/azureuser/cloudfiles/code/Users/ashruti/fio-plot/bench_fio/block_cache.yaml
sleep 3
sudo rm -rf /tmp/mntdir/pythonWrite_*

echo "Cleaning with NFSv3"
sudo umount -fl /tmp/mntdirnfs
sleep 2
sudo mount -t nfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 datamountnfs.blob.core.windows.net:/datamountnfs/test-nfs /tmp/mntdirnfs
sleep 3
sudo rm -rf /tmp/mntdirnfs/pythonWrite_*

fileList="100"

# for i in $fileList
# do
#     echo "Running with Blobfuse2 for " $i " GB file size WRITE"
#     # sudo blobfuse2 unmount all
#     # sudo blobfuse2 unmount all
#     # sleep 2
#     # sudo ./blobfuse2 /tmp/mntdir --config-file=/home/azureuser/cloudfiles/code/Users/ashruti/fio-plot/bench_fio/block_cache.yaml
#     # sleep 3
#     sudo python3 ./test/scripts/write.py /tmp/mntdir $i Blobfuse2 >> ./results_write.csv

#     # echo "Running with NFSv3 for " $i " GB file size WRITE"
#     # # sudo umount -fl /tmp/mntdirnfs
#     # # sleep 2
#     # # sudo mount -t nfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 datamountnfs.blob.core.windows.net:/datamountnfs/test-nfs /tmp/mntdirnfs
#     # # sleep 3
#     sudo python3 ./test/scripts/write.py /tmp/mntdirnfs $i NFSv3 >> ./results_write.csv
# done

for j in {1..5}
do
    sudo ./test/scripts/blobfuse2 unmount all
    sudo ./test/scripts/blobfuse2 unmount all
    sleep 2
    sudo ./test/scripts/blobfuse2 /tmp/mntdir --config-file=/home/azureuser/cloudfiles/code/Users/ashruti/fio-plot/bench_fio/block_cache.yaml
    sleep 3

    sudo umount -fl /tmp/mntdirnfs
    sleep 2
    sudo mount -t nfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 datamountnfs.blob.core.windows.net:/datamountnfs/test-nfs /tmp/mntdirnfs
    sleep 3

    echo "Running $j th iteration for READ"
    echo "Tool Size OpenTime ReadTime CloseTime TotalTime ReadSpeed AverageSpeed" > ./results_read_${j}.csv

    for i in $fileList
    do
        echo "Running with Blobfuse2 for " $i " GB file size READ"
        # sudo blobfuse2 unmount all
        # sudo blobfuse2 unmount all
        # sleep 2
        # sudo ./blobfuse2 /tmp/mntdir --config-file=/home/azureuser/cloudfiles/code/Users/ashruti/fio-plot/bench_fio/block_cache.yaml
        # sleep 3
        sudo python3 ./test/scripts/read.py /tmp/mntdir $i Blobfuse2 >> ./results_read_${j}.csv

        echo "Running with NFSv3 for " $i " GB file size READ"
        # sudo umount -fl /tmp/mntdirnfs
        # sleep 2
        # sudo mount -t nfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 datamountnfs.blob.core.windows.net:/datamountnfs/test-nfs /tmp/mntdirnfs
        # sleep 3
        sudo python3 ./test/scripts/read.py /tmp/mntdirnfs $i NFSv3 >> ./results_read_${j}.csv
    done
done