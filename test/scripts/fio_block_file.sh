#!/bin/bash

# To run this script from your workspace execute this command
#   ./test/scripts/fio_block_file.sh /mnt/blob_mnt /mnt/blobfusetmp ./file-config.yaml ./block-config.yaml 2&> results_fio_fb.txt
mntPath=$1
tmpPath=$2
fileConfigPath=$3
blockConfigPath=$4
testname=$5

outputPath=results_fio_fb.txt
rm $outputPath

echo "| Case | v2 file write IOPS | v2 file read IOPS | v2 block write IOPS | v2 block read IOPS |" >> $outputPath
echo "| -- | -- | -- | -- | -- |" >> $outputPath

for i in {1..5}; 
do 
	echo "| Run $i |" >> $outputPath
done

echo "| Average |" >> $outputPath
echo "| % Improvement |" >> $outputPath

sudo fusermount3 -u $mntPath
rm -rf $mntPath
rm -rf $tmpPath
mkdir -p $mntPath
mkdir -p $tmpPath

writecmd=`echo fio --randrepeat=1 --ioengine=libaio --gtod_reduce=1 --name=test --bs=1m --readwrite=rw --rwmixread=1 --size=10G --filename=$mntPath/testfile10G`
readcmd=`echo fio --randrepeat=1 --ioengine=libaio --gtod_reduce=1 --name=test --bs=1m --readwrite=read --size=10G --filename=$mntPath/testfile10G`

blobfuse2_write_average=0
blobfuse2_read_average=0

for v2configPath in $fileConfigPath $blockConfigPath;
do
    sed_line=3
    for i in {1..5}; 
    do 
        # Mount Blobfuse2
        ./blobfuse2 mount $mntPath --config-file=$v2configPath &
        if [ $? -ne 0 ]; then
            exit 1
        fi

        # Wait for the mount
        sleep 3
        ps -aux | grep blobfuse2

        # Start the run
        echo "Blobfuse2 Run $i"

        fio_result=`$writecmd$i`
        echo $fio_result
        write_iops=$(echo $fio_result | sed -n "s/^.*write: IOPS=\s*\(\S*\),.*$/\1/p")
        write_iops=$(echo $write_iops | tr '[:lower:]' '[:upper:]')
        write_iops=$(echo $write_iops | numfmt --from=si)
       
        sed -i "${sed_line}s/$/ ${write_iops} |/" $outputPath
        (( sed_line++ ))
        blobfuse2_write_average=$(( $blobfuse2_write_average + $write_iops ))

        sudo fusermount3 -u $mntPath
        echo "========================================================="
    done

    sed_line=3
    for i in {1..5}; 
    do 
        # Mount Blobfuse2
        ./blobfuse2 mount $mntPath --config-file=$v2configPath &
        if [ $? -ne 0 ]; then
            exit 1
        fi

        # Wait for the mount
        sleep 3
        ps -aux | grep blobfuse2

        # Start the run
        echo "Blobfuse2 Run $i"

        fio_result=`$readcmd$i`
        echo $fio_result
        read_iops=$(echo $fio_result | sed -n "s/^.*read: IOPS=\s*\(\S*\),.*$/\1/p")
        read_iops=$(echo $read_iops | tr '[:lower:]' '[:upper:]')
        read_iops=$(echo $read_iops | numfmt --from=si)
       
        sed -i "${sed_line}s/$/ ${read_iops} |/" $outputPath
        (( sed_line++ ))
        blobfuse2_read_average=$(( $blobfuse2_read_average + $read_iops ))

        sudo fusermount3 -u $mntPath
        echo "========================================================="
    done

    blobfuse2_write_average=$(( $blobfuse2_write_average / 5 ))
    blobfuse2_read_average=$(( $blobfuse2_read_average / 5 ))
    sed -i "8s/$/ ${blobfuse2_write_average} | ${blobfuse2_read_average} |/" $outputPath
done
