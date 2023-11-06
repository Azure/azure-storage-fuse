#!/bin/bash

mntPath=$1

while IFS=, read -r thread block file; do
    echo "
    [global]
    ioengine=sync
    size=${file}M
    bs=${block}M
    rw=read
    filename=$mntPath/$file.data
    numjobs=$thread
    [job]
    name=seq_read" > fio_temp.cfg

    for i in {1..3}; 
    do 
        echo "Blobfuse2 Run $i with $thread threads, $block block size, $file file size"
        
        # Mount Blobfuse2
        ./blobfuse2 mount $mntPath --config-file=$v2configPath &
        if [ $? -ne 0 ]; then
            exit 1
        fi

        # Wait for mount to stablize
        sleep 3

        # Print process summary to validte blobfuse2 is indeed mounted
        ps -aux | grep blobfuse2

        time fio fio_temp.cfg --output fio_result.csv --output-format csv

    done
   
done < <(tail -n +3 ./test/scripts/fio_tests.csv)





