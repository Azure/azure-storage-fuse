#/!bin/bash



mnt="/tmp/mntdir"

# make directory with current time to store results
results=`date | tr ' ' '_' | tr ":" "-"`
mkdir $results

# TODO : Add steps to mount using AML and run same bechmark tests
# TODO : Add steps to mount NFS and run same benchmark tests
sudo blobfuse2 unmount all
sudo blobfuse2 unmount all
rm -rf $mnt/*
rm -rf /tmp/tempcache/*


rm -rf bfuse_graphs
mkdir bfuse_graphs

# for each work flow
for flow in block_cache #file_cache
do
    # TODO: unmount blobfuse, mount blobfuse
    # TODO: have two config files block_cache.yaml file_cache.yaml for respective config
    sudo blobfuse2 unmount all
    echo $flow
    sudo blobfuse2 $mnt --config-file=./${flow}.yaml    
    sleep 2

    # validate blobfuse mount was successful or not
    cnt=`sudo ps -aux | grep blobfuse | head -1 | wc -l`
    if [ $cnt -ne 1 ]
    then
        exit 1
    fi

    # for each file-size (in MB)
    for i in 1 #10 20 100 1024 2048 8192
    do
        echo "Testing for file $i with $flow"

        # create file name
        file="file_${i}M"

        # create output directory path for fio results
        output="./$results/$flow"

        # create output directory
        mkdir -p $output

        # cleanup source file
        echo "Deleting $file"
        rm -rf $mnt/$file

        # generate data
        echo "Creating $file"
        dd if=/dev/urandom of=$mnt/$file bs=1M count=$i

        # run fio test
        echo "Running bench-fio"

        block=""
        if [ $i == 1 ]
        then
            block="1M"
        elif [ $i == 10 ]
        then
            block="1M 8M"
        else
            block="8M 16M"
        fi
        echo "Block Size: $block"

        # python3 __main__.py --target $mnt/$file --output result -b $block --size ${i}M -t file -m read write randread randwrite --iodepth 1 8 16 --numjobs 1 8 16 32 --runtime=60 --destructive
        sleep 1
        python3 __main__.py --target /tmp/mntdir/file_1M --output results  -b 1M --size 1M -t file -m read --iodepth 1 8 16 --numjobs 1 8 16 32 --runtime=60 --destructive
        sleep 2

        for size in $block
        do
            for type in bw #lat iops 
            do 
                fio-plot -i ./results/file_1M/1M --source "anubhuti-blobfuse-test"  -T "Blobfuse-Perf" -L -t $type -r write -o ./bfuse_graphs/${flow}_${file}_${size}_${type}_wr
                fio-plot -i ./results/file_1M/1M --source "anubhuti-blobfuse-test"  -T "Blobfuse-Perf" -L -t $type -r read -o ./bfuse_graphs/${flow}_${file}_${size}_${type}_re
                fio-plot -i ./results/file_1M/1M --source "anubhuti-blobfuse-test"  -T "Blobfuse-Perf" -L -t $type -r randwrite -o ./bfuse_graphs/${flow}_${file}_${size}_${type}_randwr
                fio-plot -i ./results/file_1M/1M --source "anubhuti-blobfuse-test"  -T "Blobfuse-Perf" -L -t $type -r randread -o ./bfuse_graphs/${flow}_${file}_${size}_${type}_randre
            done
        done
    done
done

        
