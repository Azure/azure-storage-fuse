#!/bin/bash
# ./file_block_compare.sh <mnt path> <1:create data>

mntPath=$1
tmpPath=$2
fileConfigPath=$3
blockConfigPath=$4

# Create mount directory if it does not exists already
mkdir -p $mntPath
chmod 777 $mntPath

# ------------------------------------------------------------------------------------------------------------------
blobfuse2 unmount all

# ------------------------------------------------------------------------------------------------------------------
# Clean up for new test
echo "Cleaning up old data"
blobfuse2 mount $mntPath --config-file=$fileConfigPath --tmp-path=$tmpPath --file-cache-timeout=0
sleep 3
rm -rf $mntPath/*
blobfuse2 unmount all

rm -rf $tmpPath/*

# ------------------------------------------------------------------------------------------------------------------
# Create the data set

# Generate report format
echo "Going for data creation"
outputPath="./file_block_write.txt"
echo "| File Size (MB) | Block Cache Speed | Block Cache Time | File Cache Speed | File Cache Time | NFS Speed | NFS Time |" > $outputPath
echo "| -- | -- | -- | -- | -- | -- | -- |" >> $outputPath

# Fill the test case data
for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -gu);
do
    echo "| ${file} |" >> $outputPath
done

# Execute the data creation dd test with block-size=32MB
for v2configPath in $blockConfigPath $fileConfigPath "nfs";
do
    sed_line=3
    echo "Running creation with $v2configPath"

    mntPath=$1
    fileBaseName=$(basename $v2configPath | cut -d "." -f1)

    if [ "$v2configPath" != "nfs" ]
    then
        blobfuse2 mount $mntPath --config-file=$v2configPath --tmp-path=$tmpPath --file-cache-timeout=0  -o allow_other
        if [ $? -ne 0 ]; then
            exit 1
        fi
    else    
        sudo mount -t aznfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 datamountnfs.blob.core.windows.net:/datamountnfs/test-nfs  /tmp/mntdirnfs
        mntPath="/tmp/mntdirnfs"
    fi
    # Wait for mount to stabilize
    sleep 3

    for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -gu);
    do
        sudo sysctl -w vm.drop_caches=3
        
        echo "Creating: " $file
        sudo dd if=/dev/urandom of=$mntPath/${fileBaseName}_${file}.data bs=1M count=$file 2> temp.tst

        write_speed=`cat temp.tst | tail -1 | rev | cut -d " " -f1,2 | rev | cut -d "/" -f1`
        write_time=`cat temp.tst | tail -1 |  cut -d "," -f3`
        
        cat temp.tst
        echo "Write Speed ${write_speed} Write Time ${write_time}"

        sed -i "${sed_line}s/$/ ${write_speed}\/s | ${write_time} |/" $outputPath
        (( sed_line++ ))

        sleep 2
    done
    
    if [ "$v2configPath" != "nfs" ]
    then
        blobfuse2 unmount all
    else
        sudo umount -fl /tmp/mntdirnfs
    fi

    sleep 3

done
echo "| -- | -- | -- | -- | -- | -- | -- |" >> $outputPath
cat $outputPath

# ------------------------------------------------------------------------------------------------------------------

# Read test case with fio command
# Generate report format
echo "Going for Read tests"
outputPath="./file_block_read.txt"
echo "| Thread | Block Size (MB) | File Size (MB) | Block Cache Speed | Block Cache Time | File Cache Speed | File Cache Time | AML Speed | AML Time | NFS Speed | NFS Time |" > $outputPath
echo "| -- | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |" >> $outputPath

# Generate the test case data
while IFS=, read -r thread block file; do
    echo "| ${thread} | ${block} | ${file} |" >> $outputPath
done < <(tail -n +3 ./test/scripts/fio_tests.csv)

# Execute the Sequential read FIO test
for v2configPath in $blockConfigPath $fileConfigPath "aml" "nfs";
do
    sed_line=3
    echo "Running read test with $v2configPath"

    fileBaseName=$(basename $v2configPath | cut -d "." -f1)
    mntPath=$1
    name=$v2configPath

    if [ "$v2configPath" == "aml" ]
    then    
        # fileBaseName=$(basename $blockConfigPath | cut -d "." -f1)
        echo "Running for AML fuse"
        fileBaseName=$(basename $blockConfigPath | cut -d "." -f1)
        mntPath=$5
    elif [ "$v2configPath" == "nfs" ]
    then 
        echo "Running for NFS"
        sudo mount -t aznfs -o sec=sys,vers=3,nolock,proto=tcp,nconnect=16 datamountnfs.blob.core.windows.net:/datamountnfs/test-nfs  /tmp/mntdirnfs
        mntPath="/tmp/mntdirnfs"
        sleep 3
    else
        # Mount Blobfuse2
        blobfuse2 mount $mntPath --config-file=$v2configPath --tmp-path=$tmpPath --file-cache-timeout=0 -o allow_other
        if [ $? -ne 0 ]; then
            exit 1
        fi
        # Wait for mount to stabilize
        sleep 3
    fi

    while IFS=, read -r thread block file; do
    	sudo sysctl -w vm.drop_caches=3

        echo "
        [global]
        ioengine=sync
        size=${file}M
        bs=${block}M
        rw=read
        filename=${mntPath}/${fileBaseName}_${file}.data
        numjobs=$thread
        [job]
        name=seq_read" > fio_temp.cfg

        echo "$name Run with $thread threads, $block block size, $file file size"

        fio_result=`sudo fio fio_temp.cfg | tail -1`
        read_bw=$(echo $fio_result | sed -e "s/^.*\(bw=[^ ,]*\).*$/\1/" | cut -d "=" -f 2 | cut -d "/" -f1)
        read_time=$(echo $fio_result | sed -e "s/^.*\(run=[^ ,]*\).*$/\1/" | cut -d "-" -f 2)

        echo $fio_result
        echo "Read Speed ${read_bw} Read Time ${read_time}"
        
        sed -i "${sed_line}s/$/ ${read_bw} | ${read_time} |/" $outputPath
        (( sed_line++ ))
    done < <(tail -n +3 ./test/scripts/fio_tests.csv)

    if [ "$v2configPath" == "aml" ]
    then 
        echo "Nothing to unmount in case of AML"
    elif [ "$v2configPath" == "nfs" ]
    then
        sudo umount -fl /tmp/mntdirnfs
    else
        blobfuse2 unmount all
    fi
    sleep 3
done
echo "| -- | -- | -- | -- | -- | -- | -- | -- | -- | -- | -- |" >> $outputPath
cat $outputPath
# ------------------------------------------------------------------------------------------------------------------


# Read test with dd command
# Generate report format

# NOTE : Commenting DD based read case as dd tends to be random in nature as in it tries to read multiple blocks
# from different positions in parlallel. This is not a sequential read test. Doing random read on 100gb file will be
# very slow and will not be a good test case for blobfuse2.

# echo "Going for Read tests with dd"
# outputPath="./file_block_read_dd.txt"
# echo "| File Size (MB) | Block Cache Speed | Block Cache Time | File Cache Speed | File Cache Time |" > $outputPath
# echo "| -- | -- | -- | -- | -- |" >> $outputPath

# # Generate the test case data
# for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -gu);
# do
#     echo "| ${file} |" >> $outputPath
# done 

# # Execute the Sequential read FIO test
# for v2configPath in $blockConfigPath $fileConfigPath;
# do
#     sed_line=3
#     echo "Running read test with $v2configPath"

#     fileBaseName=$(basename $v2configPath | cut -d "." -f1)

#     # Mount Blobfuse2
#     blobfuse2 mount $mntPath --config-file=$v2configPath --tmp-path=$tmpPath --file-cache-timeout=0
#     if [ $? -ne 0 ]; then
#         exit 1
#     fi

#     # Wait for mount to stabilize
#     sleep 3
#     sudo sysctl -w vm.drop_caches=3

#     for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -gu);
#     do
#         echo "Blobfuse2 Run with $block block size, $file file size"

#         dd of=/dev/null if=$mntPath/$dataPath/${fileBaseName}_${file}.data bs=1M count=$file 2> temp.tst
#         # cat temp.tst
        
#         read_speed=`cat temp.tst | tail -1 | rev | cut -d " " -f1,2 | rev | cut -d "/" -f1`
#         read_time=`cat temp.tst | tail -1 |  cut -d "," -f3`

#         sed -i "${sed_line}s/$/ ${read_speed}\/s | ${read_time} |/" $outputPath
#         (( sed_line++ ))

#         sleep 2
#     done 
    
#     blobfuse2 unmount all
# done
# echo "| -- | -- | -- | -- | -- | -- |" >> $outputPath
# cat $outputPath


# ------------------------------------------------------------------------------------------------------------------


# Post run cleanup
# rm -rf temp*.tst
# rm -rf fio*.cfg

mntPath=$1
echo "Cleaning up data"
blobfuse2 mount $mntPath --config-file=$fileConfigPath --tmp-path=$tmpPath --file-cache-timeout=0
sleep 3
rm -rf $mntPath/*
blobfuse2 unmount all
rm -rf $tmpPath/*