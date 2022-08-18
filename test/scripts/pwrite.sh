#!/bin/bash

thread=$1
count=$2
size=$3 
mntPath=$4 
outputPath=$5 
sed_line=$6

start_time=`date +%s`
time seq 1 $count | parallel --will-cite -j $thread -I{} dd if=/dev/zero of=$mntPath/$size_{}.tst bs=1M count=$size
end_time=`date +%s`

time_diff=$(( $end_time - $start_time ))

if [ $time_diff -eq 0 ]
then
	time_diff=1
fi

total_size=$(($count * $size * 8))
rate=$(( $total_size / $time_diff ))

echo "---------------------------------------------------"
echo "Thread : " $thread " Files : " $count " Size : " $size " MB"
echo "Upload time is        : " $time_diff " Seconds"
echo "Upload rate is        : " $rate " Mbps"

sed -i "${sed_line}s/$/ ${time_diff} |/" $outputPath


