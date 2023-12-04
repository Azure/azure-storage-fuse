#!/bin/bash
# ./file_block_compare.sh <mnt path> <1:create data>

mntPath=$1
tmpPath=$2
fileConfigPath=$3
blockConfigPath=$4

dataPath="fio_sample"

# Create mount directory if it does not exists already
mkdir -p $mntPath
chmod 777 $mntPath

# ------------------------------------------------------------------------------------------------------------------
./blobfuse2 unmount all

# ------------------------------------------------------------------------------------------------------------------
# Clean up for new test
echo "Cleaning up old data"
./blobfuse2 mount $mntPath --config-file=$fileConfigPath --tmp-path=$tmpPath --file-cache-timeout=0
sleep 3
rm -rf $mntPath/$dataPath/*
mkdir -p $mntPath/$dataPath
./blobfuse2 unmount all

rm -rf $tmpPath/*

# ------------------------------------------------------------------------------------------------------------------
# Create the data set

# Generate report format
echo "Going for data creation"
outputPath="./file_block_write.txt"
echo "| File Size (MB) | Block Cache Speed | Block Cache Time | File Cache Speed | File Cache Time |" > $outputPath
echo "| -- | -- | -- | -- | -- |" >> $outputPath

# Fill the test case data
for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -u);
do
    echo "| ${file} |" >> $outputPath
done

# Execute the data creation dd test with block-size=32MB
for v2configPath in $blockConfigPath $fileConfigPath;
do
    sed_line=3
    echo "Running creation with $v2configPath"

    fileBaseName=$(basename $v2configPath | cut -d "." -f1)

    ./blobfuse2 mount $mntPath --config-file=$v2configPath --tmp-path=$tmpPath --file-cache-timeout=0
    if [ $? -ne 0 ]; then
        exit 1
    fi

    # Wait for mount to stabilize
    sleep 3

    for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -u);
    do
        sudo sysctl -w vm.drop_caches=3
        
        echo "Creating: " $file
        dd if=/dev/urandom of=$mntPath/$dataPath/${fileBaseName}_${file}.data bs=1M count=$file 2> temp.tst
        
        write_speed=`cat temp.tst | tail -1 | rev | cut -d " " -f1,2 | rev | cut -d "/" -f1`
        write_time=`cat temp.tst | tail -1 |  cut -d "," -f3`
        
        cat temp.tst
        echo "Write Speed ${write_speed} Write Time ${write_time}"

        sed -i "${sed_line}s/$/ ${write_speed}\/s | ${write_time} |/" $outputPath
        (( sed_line++ ))

        sleep 2
    done
    ./blobfuse2 unmount all

done
echo "| -- | -- | -- |" >> $outputPath
cat $outputPath

# ------------------------------------------------------------------------------------------------------------------

# Read test case with fio command
# Generate report format
echo "Going for Read tests"
outputPath="./file_block_read.txt"
echo "| Thread | Block Size (MB) | File Size (MB) | Block Cache Speed | Block Cache Time | File Cache Speed | File Cache Time |" > $outputPath
echo "| -- | -- | -- | -- | -- | -- | -- |" >> $outputPath

# Generate the test case data
while IFS=, read -r thread block file; do
    echo "| ${thread} | ${block} | ${file} |" >> $outputPath
done < <(tail -n +3 ./test/scripts/fio_tests.csv)

# Execute the Sequential read FIO test
for v2configPath in $blockConfigPath $fileConfigPath;
do
    sed_line=3
    echo "Running read test with $v2configPath"

    fileBaseName=$(basename $v2configPath | cut -d "." -f1)
        
    # Mount Blobfuse2
    ./blobfuse2 mount $mntPath --config-file=$v2configPath --tmp-path=$tmpPath --file-cache-timeout=0
    if [ $? -ne 0 ]; then
        exit 1
    fi

    # Wait for mount to stabilize
    sleep 3

    while IFS=, read -r thread block file; do
    
    	sudo sysctl -w vm.drop_caches=3

        echo "
        [global]
        ioengine=sync
        size=${file}M
        bs=${block}M
        rw=read
        filename=$mntPath/$dataPath/${fileBaseName}_${file}.data
        numjobs=$thread
        [job]
        name=seq_read" > fio_temp.cfg

        echo "Blobfuse2 Run with $thread threads, $block block size, $file file size"

        fio_result=`fio fio_temp.cfg | tail -1`
        read_bw=$(echo $fio_result | sed -e "s/^.*\(bw=[^ ,]*\).*$/\1/" | cut -d "=" -f 2 | cut -d "/" -f1)
        read_time=$(echo $fio_result | sed -e "s/^.*\(run=[^ ,]*\).*$/\1/" | cut -d "-" -f 2)

        echo $fio_result
        echo "Read Speed ${read_bw} Read Time ${read_time}"
        
        sed -i "${sed_line}s/$/ ${read_bw} | ${read_time} |/" $outputPath
        (( sed_line++ ))
    done < <(tail -n +3 ./test/scripts/fio_tests.csv)

    ./blobfuse2 unmount all
done
echo "| -- | -- | -- | -- | -- | -- | -- |" >> $outputPath
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
# for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -u);
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
#     ./blobfuse2 mount $mntPath --config-file=$v2configPath --tmp-path=$tmpPath --file-cache-timeout=0
#     if [ $? -ne 0 ]; then
#         exit 1
#     fi

#     # Wait for mount to stabilize
#     sleep 3
#     sudo sysctl -w vm.drop_caches=3

#     for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -u);
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
    
#     ./blobfuse2 unmount all
# done
# echo "| -- | -- | -- | -- | -- | -- |" >> $outputPath
# cat $outputPath


# ------------------------------------------------------------------------------------------------------------------


# Post run cleanup
rm -rf temp*.tst
rm -rf fio*.cfg

echo "Cleaning up data"
./blobfuse2 mount $mntPath --config-file=$fileConfigPath --tmp-path=$tmpPath --file-cache-timeout=0
sleep 3
rm -rf $mntPath/$dataPath/*
mkdir $mntPath/$dataPath
./blobfuse2 unmount all
rm -rf $tmpPath/*