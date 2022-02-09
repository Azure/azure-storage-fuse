
#!/bin/bash


size=$(($1 * 1024))
out_file="/home/vibhansa/blob_mnt/testfile$1"
#out_file="/testhdd/blob_mnt/testfile$1"
echo "Going for $1 GB file"
#time dd if=/dev/zero of=$out_file bs=1M count=$size oflag=direct
time dd if=/dev/zero of=$out_file bs=1M count=$size

curr_time=`date +%s`
last_mod=`date +%s -r $out_file`
upload_time=$(( $curr_time - $last_mod ))

if [ $upload_time -eq 0 ]
then
	        upload_time=1
fi

rate=$(( $size / $upload_time ))

echo "Last modified time is : " $last_mod
echo "Current time is       : " $curr_time
echo "Upload time is        : " $upload_time
echo "Upload rate is        : " $rate " MBps"

