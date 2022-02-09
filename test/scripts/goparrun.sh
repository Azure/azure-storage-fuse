
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
	./blobfuse2 mount $mntPath --config-file=$3 &
	sleep 3
	rm -rf $mntPath/*

	./test/scripts/pwrite.sh $thread $count $size $mntPath
	sudo rm -rf $tmpPath/*
	./test/scripts/pread.sh $thread $count $size $mntPath $mntPath

	rm -rf $mntPath/*
	sudo fusermount -u $mntPath

	(( cnt++ ))

done < <(tail -n +3 ./test/scripts/test_cases.csv)




