#!/bin/bash

# To run this script from your workspace execute this command
#   ./test/scripts/fio.sh /mnt/blob_mnt /mnt/blobfusetmp ./config.yaml ./v1.cfg 2&> results.txt
mntPath=$1
tmpPath=$2
v2configPath=$3
v1configPath=$4
testname=$5

outputPath=results_fio_$testname.txt
rm $outputPath

echo "| Case | latest v2 write IOPS | latest v2 read IOPS | v1 write IOPS | v2 read IOPS |" >> $outputPath
echo "| -- | -- | -- | -- | -- |" >> $outputPath

for i in {1..5}; 
do 
	echo "| Run $i |" >> $outputPath
done

echo "| Average |" >> $outputPath
echo "| % Improvement |" >> $outputPath

sudo fusermount3 -u $mntPath
rm -rf $mntPath
rm -rf $tmpPath
mkdir -p $mntPath
mkdir -p $tmpPath

fiocmd=""
if [ "$6" == "csi" ]
then
echo "Special fio command."
fiocmd=`echo fio --randrepeat=0 --verify=0 --ioengine=libaio --lat_percentiles=1 --name=rw_mix --bs=4K --iodepth=64 --size=4G --readwrite=randrw --rwmixread=70 --time_based --ramp_time=10s --runtime=60s --filename=$mntPath/testfile4G`
else
echo "Regular fio command."
fiocmd=`echo fio --randrepeat=1 --ioengine=libaio --gtod_reduce=1 --name=test--bs=4k --iodepth=64 --readwrite=$testname --rwmixread=75 --size=4G --filename=$mntPath/testfile4G`
fi

echo -n "Test command: "
echo $fiocmd
echo 

# Mount Blobfuse2
./blobfuse2 mount $mntPath --config-file=$v2configPath &
if [ $? -ne 0 ]; then
    exit 1
fi
sleep 3
ps -aux | grep blobfuse2

sed_line=3
blobfuse2_write_average=0
blobfuse2_read_average=0

for i in {1..5}; 
do 
	echo "Blobfuse2 Run $i"

    fio_result=`$fiocmd$i`
    echo $fio_result
    read_iops=$(echo $fio_result | sed -n "s/^.*read: IOPS=\s*\(\S*\),.*$/\1/p")
    read_iops=$(echo $read_iops | tr '[:lower:]' '[:upper:]')
    read_iops=$(echo $read_iops | numfmt --from=si)
    echo $read_iops
    write_iops=$(echo $fio_result | sed -n "s/^.*write: IOPS=\s*\(\S*\),.*$/\1/p")
    write_iops=$(echo $write_iops | tr '[:lower:]' '[:upper:]')
    write_iops=$(echo $write_iops | numfmt --from=si)
    echo $write_iops

	sed -i "${sed_line}s/$/ ${write_iops} | ${read_iops} |/" $outputPath

	rm $mntPath/testfile4G$i

	(( sed_line++ ))
    blobfuse2_write_average=$(( $blobfuse2_write_average + $write_iops ))
    blobfuse2_read_average=$(( $blobfuse2_read_average + $read_iops ))
    echo "========================================================="
done
sudo fusermount3 -u $mntPath

# Mount Blobfuse
blobfuse $mntPath --tmp-path=$tmpPath --config-file=$v1configPath --log-level=LOG_ERR --file-cache-timeout-in-seconds=0 --use-attr-cache=true
if [ $? -ne 0 ]; then
    exit 1
fi
sleep 3
ps -aux | grep blobfuse

sed_line=3
blobfuse_write_average=0
blobfuse_read_average=0
for i in {1..5}; 
do 
	echo "Blobfuse Run $i"

    fio_result=`$fiocmd$i`
    echo $fio_result
    read_iops=$(echo $fio_result | sed -n "s/^.*read: IOPS=\s*\(\S*\),.*$/\1/p")
    read_iops=$(echo $read_iops | tr '[:lower:]' '[:upper:]')
    read_iops=$(echo $read_iops | numfmt --from=si)
    echo $read_iops
    write_iops=$(echo $fio_result | sed -n "s/^.*write: IOPS=\s*\(\S*\),.*$/\1/p")
    write_iops=$(echo $write_iops | tr '[:lower:]' '[:upper:]')
    write_iops=$(echo $write_iops | numfmt --from=si)
    echo $write_iops

	sed -i "${sed_line}s/$/ ${write_iops} | ${read_iops} |/" $outputPath

	rm $mntPath/testfile4G$i

	(( sed_line++ ))
    blobfuse_write_average=$(( $blobfuse_write_average + $write_iops ))
    blobfuse_read_average=$(( $blobfuse_read_average + $read_iops ))
    echo "========================================================="
done
sudo fusermount3 -u $mntPath

blobfuse2_write_average=$(( $blobfuse2_write_average / 5 ))
blobfuse2_read_average=$(( $blobfuse2_read_average / 5 ))
blobfuse_write_average=$(( $blobfuse_write_average / 5 ))
blobfuse_read_average=$(( $blobfuse_read_average / 5 ))

sed -i "8s/$/ ${blobfuse2_write_average} | ${blobfuse2_read_average} | ${blobfuse_write_average} | ${blobfuse_read_average} |/" $outputPath

# Calculate the % difference
diff_write=$(( $blobfuse2_write_average - $blobfuse_write_average ))
percent_write=`echo "scale=2; $diff_write * 100 / $blobfuse_write_average" | bc`

diff_read=$(( $blobfuse2_read_average - $blobfuse_read_average ))
percent_read=`echo "scale=2; $diff_read * 100 / $blobfuse_read_average" | bc`

sed -i "9s/$/ ${percent_write} | ${percent_read} |/" $outputPath
