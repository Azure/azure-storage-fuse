#!/bin/bash
mntPath=$1
tmpPath=$2
v2configPath=$3
v1configPath=$4
outputPath=results.txt
rm $outputPath

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

# Run v1
sed -i '1s/$/ v1 write | v1 read | /' $outputPath
sed -i '2s/$/ -- | -- |/' $outputPath

./test/scripts/cppparrun.sh $mntPath $tmpPath $v1configPath $outputPath