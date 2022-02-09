
#!/bin/bash
sudo fusermount -u ~/blob_mnt
rm -rf ~/blob_mnt/*
sudo rm -rf /mnt/ramdisk/*

for i in {1,2,3}
do
	echo "--------------- Test $i ------------------------"
	/home/vibhansa/blobfuse/azure-storage-fuse/build/blobfuse /home/vibhansa/blob_mnt --tmp-path=/mnt/ramdisk --config-file=/home/vibhansa/myblob.cfg --log-level=LOG_ERR -o allow_other --file-cache-timeout-in-seconds=0 --use-attr-cache=true --max-concurrency=32
	sleep 5
	rm -rf ~/blob_mnt/*

	./test/scripts/multi_write.sh

	rm -rf ~/blob_mnt/*
	sudo fusermount -u ~/blob_mnt

done
