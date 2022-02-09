
#!/bin/bash

#rm -rf /home/vibhansa/blob_mnt/testfile*

#time dd if=/dev/zero of=/home/vibhansa/blob_mnt/testfile200 bs=1M count=200
#time dd if=/dev/zero of=/home/vibhansa/blob_mnt/testfile500 bs=1M count=500
#time dd if=/dev/zero of=/home/vibhansa/blob_mnt/testfile0_1 bs=1M count=1024
#time dd if=/dev/zero of=/home/vibhansa/blob_mnt/testfile0_2 bs=1M count=1024
#time dd if=/dev/zero of=/home/vibhansa/blob_mnt/testfile0_3 bs=1M count=1024

for i in {1,2,3,4,5,6}
do
	echo "-----------------------------------------------------"
	./test/scripts/write.sh $i
	sudo rm -rf /mnt/blobfusetmp/*
	sudo rm -rf /mnt/ramdisk/*
	sudo rm -rf /tmp/ramdisk/*
	./test/scripts/read.sh $i
	#rm -rf /home/vibhansa/blob_mnt/test*
	sudo rm -rf /mnt/ramdisk/*
	sudo rm -rf /mnt/blobfusetmp/*
	sudo rm -rf /tmp/ramdisk/*
done

