#!/bin/bash

# To run this script from your workspace execute this command
#   ./test/scripts/git_clone.sh /mnt/blob_mnt /mnt/blobfusetmp ./config.yaml ./v1.cfg 2&> results.txt
mntPath=$1
tmpPath=$2
v2configPath=$3
v1configPath=$4
outputPath=results_git_clone.txt
rm $outputPath

echo "| Case | latest v2 | v1 |" >> $outputPath
echo "| -- | -- | -- |" >> $outputPath

for i in {1..3}; 
do 
	echo "| Run $i |" >> $outputPath
done

echo "| Average |" >> $outputPath
echo "| % Diff |" >> $outputPath

sudo fusermount3 -u $mntPath
rm -rf $mntPath
rm -rf $tmpPath
mkdir -p $mntPath
mkdir -p $tmpPath

sed_line=3
blobfuse2_average=0
for i in {1..3}; 
do 
	echo "Blobfuse2 Run $i"
	rm -rf $mntPath
	rm -rf $tmpPath
	mkdir -p $mntPath
	mkdir -p $tmpPath
	./blobfuse2 mount $mntPath --config-file=$v2configPath &
	if [ $? -ne 0 ]; then
        exit 1
    fi
	sleep 3
    ps -aux | grep blobfuse2
	rm -rf $mntPath/vscode

	start_time=`date +%s`
	time git clone https://github.com/microsoft/vscode.git $mntPath/vscode
	end_time=`date +%s`

	time_diff=$(( $end_time - $start_time ))

	if [ $time_diff -eq 0 ]
	then
		time_diff=1
	fi	
	echo $time_diff
	sed -i "${sed_line}s/$/ ${time_diff} |/" $outputPath

	ls -la $mntPath
	rm -rf $mntPath/vscode
	sudo fusermount3 -u $mntPath

	(( sed_line++ ))
	blobfuse2_average=$(( $blobfuse2_average + $time_diff ))
	echo "========================================================="
done

sed_line=3
blobfuse_average=0
for i in {1..3}; 
do 
	echo "Blobfuse Run $i"
	rm -rf $mntPath
	rm -rf $tmpPath
	mkdir -p $mntPath
	mkdir -p $tmpPath
	blobfuse $mntPath --tmp-path=$tmpPath --config-file=$v1configPath --log-level=LOG_ERR --file-cache-timeout-in-seconds=0 --use-attr-cache=true
	if [ $? -ne 0 ]; then
        exit 1
    fi
	sleep 3
    ps -aux | grep blobfuse
	rm -rf $mntPath/vscode

	start_time=`date +%s`
	time git clone https://github.com/microsoft/vscode.git $mntPath/vscode
	end_time=`date +%s`

	time_diff=$(( $end_time - $start_time ))

	if [ $time_diff -eq 0 ]
	then
		time_diff=1
	fi	
	echo $time_diff
	sed -i "${sed_line}s/$/ ${time_diff} |/" $outputPath

	ls -la $mntPath
	rm -rf $mntPath/vscode
	sudo fusermount3 -u $mntPath

	(( sed_line++ ))
	blobfuse_average=$(( $blobfuse_average + $time_diff ))
	echo "========================================================="
done
blobfuse2_average=$(( $blobfuse2_average / 3 ))
blobfuse_average=$(( $blobfuse_average / 3 ))

sed -i "6s/$/ ${blobfuse2_average} | ${blobfuse_average} |/" $outputPath

# Calculate the % difference
diff=$(( $blobfuse2_average - $blobfuse_average ))
percent=`echo "scale=2; $diff * 100 / $blobfuse_average" | bc`

sed -i "7s/$/ ${percent} | |/" $outputPath