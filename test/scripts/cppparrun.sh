#!/bin/bash

mntPath=$1
tmpPath=$2
config=$3
outputPath=$4

sudo fusermount3 -u $mntPath
rm -rf $mntPath/*
sudo rm -rf $tmpPath/*

cnt=1
sed_line=3
while IFS=, read -r thread count size; do

	echo "Blobfuse | $cnt ($thread threads: $count files : $size MB) |"
	blobfuse $mntPath --tmp-path=$tmpPath --config-file=$config --log-level=LOG_ERR -o allow_other --file-cache-timeout-in-seconds=0 --use-attr-cache=true --max-concurrency=32
	sleep 3
	rm -rf $mntPath/*

	./test/scripts/pwrite.sh $thread $count $size $mntPath $outputPath $sed_line
	sudo rm -rf $tmpPath/*
	./test/scripts/pread.sh $thread $count $size $mntPath $outputPath $sed_line

	rm -rf $mntPath/*
	sudo fusermount3 -u $mntPath

	(( cnt++ ))
	(( sed_line++ ))

done < <(tail -n +3 ./test/scripts/test_cases.csv)