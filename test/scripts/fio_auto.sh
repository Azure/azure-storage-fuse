#!/bin/bash
# ./fio_auto.sh <mnt path> <1:create data>


mntPath=$1
dataPath="fio_sample"

#------------------------------------------------------------------------------------------------------------------
# Create the data set
if [[ $2 == 1 ]]
then
    echo "Going for data creation"
    v2configPath="./config_block.yaml"
    ./blobfuse2 mount $mntPath --config-file=$v2configPath 
    if [ $? -ne 0 ]; then
        exit 1
    fi

    # Wait for mount to stablize
    sleep 3

    # Print process summary to validte blobfuse2 is indeed mounted
    ps -aux | grep blobfuse2

    mkdir $mntPath/$dataPath
    for file in $(cat fio_tests.csv  | cut -d "," -f3 | tail -n +3 | sort -u);
    do
        echo "Creating: " $file
        time dd if=/dev/urandom of=$mntPath/$dataPath/$file.data bs=1M count=$file
    done
    ./blobfuse2 unmount all
else
    echo "Skipping data creation"
fi
#------------------------------------------------------------------------------------------------------------------


# Execute the Sequential read FIO test
for v2configPath in "./config_block.yaml" "./config_file.yaml";
do
    while IFS=, read -r thread block file; do
        echo "
        [global]
        ioengine=sync
        size=${file}M
        bs=${block}M
        rw=read
        filename=$mntPath/$dataPath/$file.data
        numjobs=$thread
        [job]
        name=seq_read" > fio_temp.cfg

        for i in {1..3}; 
        do 
            echo "Blobfuse2 Run $i with $thread threads, $block block size, $file file size"
            
            # Mount Blobfuse2
            ./blobfuse2 mount $mntPath --config-file=$v2configPath
            if [ $? -ne 0 ]; then
                exit 1
            fi

            # Wait for mount to stablize
            sleep 3

            # Print process summary to validte blobfuse2 is indeed mounted
            ps -aux | grep blobfuse2

            # Run sequential read test
            time fio fio_temp.cfg 
            #--output fio_result_$file$i.csv --output-format csv

            # Unmount Blobfuse2
            ./blobfuse2 unmount all

        done

    done < <(tail -n +3 ./test/scripts/fio_tests.csv)
done

cat fio_result_*.csv 




