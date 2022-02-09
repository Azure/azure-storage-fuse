
logFile="./overnightstress.log"

mntPath=/home/AzureUser/mcdhee/mcdhee_fs
tmpPath=/home/AzureUser/mcdhee/ramdisk

blobfuse=/home/AzureUser/mcdhee/azure-storage-fuse/build/blobfuse
blobfusecfg=/home/AzureUser/myblob.cfg

# Common methods to be used
#########################################################################
unmount_fuse()
{
    rm -rf $mntPath/*
    sudo fusermount -u $mntPath
    rm -rf $tmpPath/*
}

mount_blobfuse()
{
    rm -rf $tmpPath/*
    $blobfuse $mntPath --tmp-path=$tmpPath --config-file=$blobfusecfg --log-level=LOG_ERR -o allow_other --file-cache-timeout-in-seconds=0 --use-attr-cache=true --max-concurrency=32
    sleep 3
    rm -rf $mntPath/*
}

mount_blobfuse2()
{
    rm -rf $tmpPath/*
    ./blobfuse2 mount $mntPath --config-file=$1 &
    sleep 3
    rm -rf $mntPath/*
}

stress_test()
{
    ./test/scripts/stresstest.sh $mntPath >> $logFile
    rm -rf $mntPath/*
    rm -rf $tmpPath/*
    unmount_fuse
}
#########################################################################



unmount_fuse

echo  "Starting StressTest Suite" > $logFile

for i in {1,2,3}
do
    echo ":: Blobfuse Empty : $i" >> $logFile
    mount_blobfuse
    stress_test
done



for i in {1,2,3}
do
    echo ":: Fuse No Empty : $i" >> $logFile
    mount_blobfuse2 ./config_fuse_noemp.yaml
    stress_test

    echo ":: Fuse Empty : $i" >> $logFile
    mount_blobfuse2 ./config_fuse_emp.yaml
    stress_test

done

echo "Ending Test Suite" >> $logFile



