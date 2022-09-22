#!/bin/bash

# To run this script from your workspace execute this command
#   ./test/scripts/run.sh /mnt/blob_mnt /mnt/blobfusetmp ./config.yaml ./v1.cfg 2&> results.txt
mntPath=$1
tmpPath=$2
v2configPath=$3
v1configPath=$4
outputPath=results.txt
rm $outputPath

sudo fusermount3 -u $mntPath
rm -rf $mntPath
rm -rf $tmpPath
mkdir -p $mntPath
mkdir -p $tmpPath

echo "| Case |" >> $outputPath
echo "| -- |" >> $outputPath
cnt=1
while IFS=, read -r thread count size; do
	echo "| $cnt ($thread threads: $count files : $size MB) |" >> $outputPath
	(( cnt++ ))
done < <(tail -n +3 ./test/scripts/test_cases.csv)
# Run on current branch
sed -i '1s/$/ latest v2 write | latest v2 read | /' $outputPath
sed -i '2s/$/ -- | -- |/' $outputPath

./test/scripts/goparrun.sh $mntPath $tmpPath $v2configPath $outputPath
if [ $? -ne 0 ]; then
    	exit 1
fi

# Run v1
sed -i '1s/$/ v1 write | v1 read | /' $outputPath
sed -i '2s/$/ -- | -- |/' $outputPath

./test/scripts/cppparrun.sh $mntPath $tmpPath $v1configPath $outputPath
if [ $? -ne 0 ]; then
    	exit 1
fi

# Calculate the % difference
tail -n +3 $outputPath > temp.out

sed -i '1s/$/ write % improve | read % improve | /' $outputPath
sed -i '2s/$/ -- | -- |/' $outputPath
count=0
sed_line=3

while IFS=\| read -r casenum case v2write v2read v1write v1read; do
	# Swapping order of blobfuse and blobfuse2 so that blobfuse2 taking less time reflects it being better than blobfuse. 
	writeDiff=$(( $v1write - $v2write ))
	readDiff=$(( $v1read - $v2read ))

    writePercent=`echo "scale=2; $writeDiff * 100 / $v1write" | bc`
    readPercent=`echo "scale=2; $readDiff * 100 / $v1read" | bc`

	echo $writePercent $readPercent
	sed -i "${sed_line}s/$/ ${writePercent} |/" $outputPath
	sed -i "${sed_line}s/$/ ${readPercent} |/" $outputPath
	sed_line=$(($sed_line + 1))
	count=$(( $count + 1))
done < temp.out

tail -n +3 $outputPath > temp.out
totalWriteImprove=`cut -d "|" -f 7 temp.out | sed -e 's/ //g' | paste -sd+ | bc`
totalReadImprove=`cut -d "|" -f 8 temp.out | sed -e 's/ //g' | paste -sd+ | bc`

echo "| $count Test Case Average | -- | -- | -- | -- | `echo "scale=2; $totalWriteImprove / $count" | bc` | `echo "scale=2; $totalReadImprove / $count" | bc` |" >> $outputPath 
rm -rf temp.out