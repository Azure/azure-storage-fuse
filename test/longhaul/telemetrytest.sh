
#!/bin/bash

while true
do
        while read -r line
        do
                echo "Running with : " $line

                rm -rf ~/blob_mnt2/stress*
                rm -rf ~/blob_mnt2/myfile*

                echo "Un-Mounting Blobfuse2"
                ./blobfuse2 unmount all
                rm -rf ~/blob_mnt2/*
                rm -rf ~/blob_mnt2/.*
                rm -rf /mnt/ramdisk/*

                echo "Mounting Blobfuse2"
                ./blobfuse2 mount ~/blob_mnt2 --config-file=./config.yaml --file-cache-timeout=0 --telemetry="$line"
                sleep 2

                echo "Blobfuse2 pid : " `pidof blobfuse2`
                echo "`whoami` : `date` :: Starting stress test " >> ./longhaul2.log
                ./test/longhaul/stresstest.sh
                echo "`whoami` : `date` :: Ending stress test " >> ./longhaul2.log

                sleep 300
        done < $1
done

