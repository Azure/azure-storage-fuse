

#!/bin/bash
cnt=1
mntPath=$1
tmpPath=$2

sudo fusermount -u $mntPath
rm -rf $mntPath/*
sudo rm -rf $tmpPath/*
rm results.txt


while IFS=, read -r thread count size; do

	echo "--------------- Blobfuse2 Test $cnt ($thread : $count : $size)------------------------" >> results.txt
	$3 $mntPath --tmp-path=$tmpPath --config-file=$4 --log-level=LOG_ERR -o allow_other --file-cache-timeout-in-seconds=0 --use-attr-cache=true --max-concurrency=32
	sleep 3
	rm -rf $mntPath/*

	./test/scripts/pwrite.sh $thread $count $size $mntPath
	sudo rm -rf $tmpPath/*
	./test/scripts/pread.sh $thread $count $size $mntPath $mntPath

	rm -rf $mntPath/*
	sudo fusermount -u $mntPath

	(( cnt++ ))

done < <(tail -n +3 ./test/scripts/test_cases.csv)




