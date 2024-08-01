SERVICE="blobfuse2"
SCRIPT="longhaul.sh"
WORKDIR="/home/blobfuse/azure-storage-fuse"

echo "Staring script"
if pgrep -x "$SERVICE" > /dev/null
then
        echo "Check existing run"
        #count=`ps -aux | grep $SCRIPT | wc -l`
        #echo "Existing run count  : $count"

        if [ -e "longhaul.lock" ]
        then
                echo "Script already running"
                echo "`date` :: Already running" >> $WORKDIR/longhaul.log
        else
                touch longhaul.lock
                echo "New script start"
                if [ `stat -c %s $WORKDIR/longhaul.log` -gt 10485760 ]
                then
                        echo "`date` :: Trimmed " > $WORKDIR/longhaul.log
                fi

                echo "`whoami` : `date` :: `$WORKDIR/blobfuse2 --version` Starting test " >> $WORKDIR/longhaul.log

                mem=$(top -b -n 1 -p `pgrep -x blobfuse2` | tail -1)
                elap=$( ps -p `pgrep -x blobfuse2` -o etime | tail -1)
                echo $mem " :: " $elap >> $WORKDIR/longhaul.log

                echo "Delete old data"
                echo "`date` : Cleanup old test data" >> $WORKDIR/longhaul.log
                rm -rf /blob_mnt/kernel

                echo "Start test"
                echo "`date` : Building Kernel"  >> $WORKDIR/longhaul.log
                mkdir /blob_mnt/kernel
                $WORKDIR/build_kernel.sh /blob_mnt/kernel/ 6.10.2

                if [ $? -ne 0 ]; then
                  echo "`date` : Make Failed" >> $WORKDIR/longhaul.log
                fi
                echo "End test"
                echo "`date` : Kernel Build complete"  >> $WORKDIR/longhaul.log

                sleep 30
                echo "Cleanup post test"
                rm -rf /blob_mnt/test/*
                rm -rf /blob_mnt/kernel

                cp  $WORKDIR/longhaul.log  /blob_mnt/
                rm -rf longhaul.lock
        fi
else
        echo "Blobfuse not running"
        echo "`date` :: Re-Starting blobfuse2 *******************" >> $WORKDIR/longhaul.log
        $WORKDIR/blobfuse2 unmount all

        rm -rf /blob_mnt/*

        export AZURE_STORAGE_ACCOUNT=vikasfuseblob
        export AZURE_STORAGE_AUTH_TYPE=msi
        export AZURE_STORAGE_IDENTITY_CLIENT_ID=1f1551d2-2db2-4d4d-a6f5-d7edbe75d98e

        echo "Start blobfuse"
        $WORKDIR/blobfuse2 mount /blob_mnt --log-level=log_debug --log-file-path=$WORKDIR/blobfuse2.log --log-type=base --block-cache --container-name=longhaul

        sleep 2

        if [ `stat -c %s $WORKDIR/restart.log` -gt 10485760 ]
        then
                echo "`date` Trimmed " > $WORKDIR/restart.log
        fi
        echo "`date`: Restart : `$WORKDIR/blobfuse2 --version`" >> $WORKDIR/restart.log

        echo "Send mail"
        # Send email that blobfuse2 has crashed
        echo "Blobfuse2 Failure" | mail -s "Blobfuse2 Restart" -A $WORKDIR/restart.log -a "From: longhaul@blobfuse.com" vibhansa@microsoft.com

        cp $WORKDIR/blobfuse2.log /blob_mnt/
        cp $WORKDIR/longhaul.log  /blob_mnt/
        cp $WORKDIR/restart.log /blob_mnt/
fi
