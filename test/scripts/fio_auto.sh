#!/bin/bash

# for i in {1024,2048,4096,5192,102400,519200,1024000}; do dd if=/dev/urandom of=./$i.data bs=1M count=$i; done

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

        # Run sequential read test
        time fio fio_temp.cfg --output fio_result_$file$i.csv --output-format csv

        # Unmount Blobfuse2
        ./blobfuse2 unmount all

    done

done < <(tail -n +3 ./test/scripts/fio_tests.csv)

cat fio_result_*.csv 




