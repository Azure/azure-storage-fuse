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
./blobfuse2 mount $mntPath --config-file=$v2configPath --block-cache-block-size=32 --file-cache-timeout=0
sleep 3
rm -rf $mntPath/$dataPath/*
mkdir $mntPath/$dataPath
./blobfuse2 unmount all

rm -rf $tmpPath/*

# ------------------------------------------------------------------------------------------------------------------
# Create the data set
if [[ $2 == 1 ]]
then
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
        ./blobfuse2 mount $mntPath --config-file=$v2configPath --block-cache-block-size=32 --file-cache-timeout=0
        if [ $? -ne 0 ]; then
            exit 1
        fi

        # Wait for mount to stabilize
        sleep 3

        for file in $(cat ./test/scripts/fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -u);
        do
            echo "Creating: " $file
            dd if=/dev/urandom of=$mntPath/$dataPath/$v2configPath$file.data bs=1M count=$file 2> temp.tst
            write_speed=`cat temp.tst | tail -1 | rev | cut -d " " -f1,2 | rev | cut -d "/" -f1`
            write_time=`cat temp.tst | tail -1 |  cut -d "," -f3`
            
            sed -i "${sed_line}s/$/ ${write_speed} | ${write_time} |/" $outputPath
            (( sed_line++ ))
        done
        ./blobfuse2 unmount all
         echo "| -- | -- | -- |" >> $outputPath
         cat $outputPath
    done
else
    echo "Skipping data creation"
fi

# ------------------------------------------------------------------------------------------------------------------

# Read test case starts here
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

    while IFS=, read -r thread block file; do
        echo "
        [global]
        ioengine=sync
        size=${file}M
        bs=${block}M
        rw=read
        filename=$mntPath/$dataPath/$v2configPath$file.data
        numjobs=$thread
        [job]
        name=seq_read" > fio_temp.cfg

        echo "Blobfuse2 Run with $thread threads, $block block size, $file file size"
        
        # Mount Blobfuse2
        ./blobfuse2 mount $mntPath --config-file=$v2configPath --block-cache-prefetch-on-open=true --block-cache-block-size=$block --file-cache-timeout=0
        if [ $? -ne 0 ]; then
            exit 1
        fi

        # Wait for mount to stabilize
        sleep 3

        fio_result=`fio fio_temp.cfg | tail -1`
        read_bw=$(echo $fio_result | sed -e "s/^.*\(bw=[^ ,]*\).*$/\1/" | cut -d "=" -f 2 | cut -d "/" -f1)
        read_time=$(echo $fio_result | sed -e "s/^.*\(run=[^ ,]*\).*$/\1/" | cut -d "-" -f 2)

        sed -i "${sed_line}s/$/ ${read_bw} | ${read_time} |/" $outputPath
        (( sed_line++ ))

        # Unmount Blobfuse2
        ./blobfuse2 unmount all
    done < <(tail -n +3 ./test/scripts/fio_tests.csv)
done
echo "| -- | -- | -- | -- | -- | -- | -- |" >> $outputPath
cat $outputPath
# ------------------------------------------------------------------------------------------------------------------


