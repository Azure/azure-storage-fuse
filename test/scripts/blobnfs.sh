
#!/bin/bash

MNTPOINTBF="/tmp/mntdir"
MNTPOINTBN="/tmp/mntdirnfs"

# Update this variable for file size
file_size=10
bs="1M"
count=10240
op="write"

file_size_str=${file_size}G
file_size_bytes=$((file_size*1024))

echo "EXECUTING WRITE FOR FILE SIZE $file_size GB AND BLOCK SIZE $bs mb"

# sudo echo 3 > /proc/sys/vm/drop_caches
sudo sysctl -w vm.drop_caches=3

START=$(date +%s.%N)
sudo mount -t nfs -o vers=3,proto=tcp datamountnfs.blob.core.windows.net:/datamountnfs/test-nfs $MNTPOINTBN
END=$(date +%s.%N)

DEVICE=$(stat --format=%d $MNTPOINTBN)

echo 16384 > /sys/class/bdi/0:$DEVICE/read_ahead_kb

DIFF=$(bc <<< "scale=3; $END-$START")
echo " BlobNFS mount ($MNTPOINTBN) took $DIFF seconds"

START=$(date +%s.%N)
blobfuse2 mount $MNTPOINTBF --config-file=/home/azureuser/cloudfiles/code/Users/ashruti/fio-plot/bench_fio/block_cache.yaml
END=$(date +%s.%N)

DIFF=$(bc <<< "scale=3; $END-$START")
echo " BlobFuse mount $MNTPOINTBF took $DIFF seconds"

# echo "Starting the NFS test hook"
# Write test hook
# dd if=/mnt/shubhamnfsmntpoint/..TestHook status=progress bs=1M count=10240
# Read test hook


#Running top in backgroup
# nohup ./top-background.sh &

sleep 4

start_total_nfs=$(date +%s.%N)

echo "File opening for nfs" | tee -a output2.txt
# Record start time for open
start_time_open=$(date +%s.%N)

nfs_file="fusewrite.$file_size_str"
#nfs_file=nfswrite$start_time_open.$file_size_str
#touch "$MNTPOINTBN/$nfs_file"
#nfs_file=nfswrite.$file_size_str

echo "$MNTPOINTBN/$nfs_file"

# Open the file
sudo exec 3< "$MNTPOINTBN/$nfs_file"

# Record end time for open
end_time_open=$(date +%s.%N)

echo "File opened for nfs" | tee -a output2.txt

# Calculate open time
elapsed_time_open=$(bc <<< "scale=3; $end_time_open-$start_time_open")

echo "Nfs file opening time: $elapsed_time_open seconds"

START=$(date +%s.%N)
echo "Starting execution for BlobNFS, endpoint=$MNTPOINTBN"

# Write command
echo "DD Command executing for nfs" | tee -a output2.txt
time dd of="/dev/fd/3" if=/dev/zero status=progress bs=$bs count=$count | tee -a output2.txt

# Read command
time dd if="/dev/fd/3" of=/dev/null status=progress bs=$bs conv=sync | tee -a output2.txt

echo "DD Command executed for nfs" | tee -a output2.txt

END=$(date +%s.%N)

DIFF=$(bc <<< "scale=3; $END-$START")
echo " rw for Nfs took $DIFF seconds"

# Record start time for close
start_time_close=$(date +%s.%N)

echo "File closing for nfs" | tee -a output2.txt

# Close the file
exec 3<&-

echo "File closed for nfs" | tee -a output2.txt

# Record end time for close
end_time_close=$(date +%s.%N)

# Calculate close time
elapsed_time_close=$(bc <<< "scale=3; $end_time_close-$start_time_close")

echo "Nfs file close time: $elapsed_time_close seconds"

close_total_nfs=$(date +%s.%N)

# Calculate close time

total_nfs_time=$(bc <<< "scale=3; $close_total_nfs-$start_total_nfs")

echo "NFS TOTAL EXECUTION TIME: $total_nfs_time"

nfs_speed=$(bc <<< "scale=3; $file_size_bytes/$total_nfs_time")
echo "NFS EXECUTION SPEED: $nfs_speed"


echo 3 > /proc/sys/vm/drop_caches
sleep 10

start_total_fuse=$(date +%s.%N)

# Record start time for open
start_time_open=$(date +%s.%N)

echo "File opening for fuse" | tee -a output2.txt

fuse_file="fusewrite.$file_size_str"
#fuse_file=nfswrite.$file_size_str
#fuse_file=fusewrite$start_time_open.$file_size_str
#touch "$MNTPOINTBF/$fuse_file"

# Open the file
exec 3< "$MNTPOINTBF/$fuse_file"

echo "File opened for fuse" | tee -a output2.txt

# Record end time for open
end_time_open=$(date +%s.%N)

# Calculate open time
elapsed_time_open=$(bc <<< "scale=3; $end_time_open-$start_time_open")

echo "Fuse file opening time: $elapsed_time_open seconds"

START=$(date +%s.%N)
echo "Starting execution for BlobFuse, endpoint=$MNTPOINTBF"

echo "DD Command executing for fuse" | tee -a output2.txt

# Write command
# time dd of="/dev/fd/3" if=/dev/zero status=progress bs=$bs count=$count | tee -a output2.txt

# Read command
time dd if="/dev/fd/3" of=/dev/null status=progress bs=$bs conv=sync | tee -a output2.txt
echo "DD Command executed for fuse" | tee -a output2.txt
END=$(date +%s.%N)

DIFF=$(bc <<< "scale=3; $END-$START")
echo " rw for Fuse took $DIFF seconds"

# Record start time for close
start_time_close=$(date +%s.%N)

echo "File closin for fuse" | tee -a output2.txt
# Close the file
exec 3<&-

echo "File closed for fuse" | tee -a output2.txt
# Record end time for close
end_time_close=$(date +%s.%N)

# Calculate close time
elapsed_time_close=$(bc <<< "scale=3; $end_time_close-$start_time_close")

echo "Fuse file close time: $elapsed_time_close seconds"

close_total_fuse=$(date +%s.%N)

# Calculate close time

total_fuse_time=$(bc <<< "scale=3; $close_total_fuse-$start_total_fuse")

echo "FUSE TOTAL EXECUTION TIME: $total_fuse_time"

fuse_speed=$(bc <<< "scale=3; $file_size_bytes/$total_fuse_time")
echo "FUSE EXECUTION SPEED: $fuse_speed"

sleep 4

kill $(jobs -p)

START=$(date +%s.%N)
umount -f -l $MNTPOINTBN
END=$(date +%s.%N)

DIFF=$(bc <<< "scale=3; $END-$START")
echo " BlobNFS unmount ($MNTPOINTBN) took $DIFF seconds"

START=$(date +%s.%N)
fusermount -u $MNTPOINTBF
END=$(date +%s.%N)

DIFF=$(bc <<< "scale=3; $END-$START")
echo " BlobFuse unmount ($MNTPOINTBF) took $DIFF seconds"
