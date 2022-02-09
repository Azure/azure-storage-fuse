
#!/bin/bash
sudo fusermount -u ~/blob_mnt
rm -rf ~/blob_mnt/*
sudo rm -rf /mnt/ramdisk/*
pwd

for i in {1,2,3}
do
	echo "--------------- Test $i ------------------------"
	./blobfuse2 mount /home/vibhansa/blob_mnt --config-file=./config.yaml &
	sleep 5
	rm -rf ~/blob_mnt/*
	ls -l ~/blob_mnt

	./test/scripts/multi_write.sh

	rm -rf ~/blob_mnt/*
	sudo fusermount -u ~/blob_mnt

done
