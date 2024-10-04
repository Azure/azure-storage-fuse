./blobfuse2 unmount all
echo "Mount blobfuse and create 10GB file"
./blobfuse2 ~/mntdir && dd if=/dev/urandom of=~/mntdir/new10 bs=1M count=10000
echo "--------------------------------------------------------------------------------"
echo "File created in mntdir"
echo "--------------------------------------------------------------------------------"
./blobfuse2 unmount all
echo "Mount and Run FIO test"
./blobfuse2 ~/mntdir && fio fio_temp.cfg
echo "--------------------------------------------------------------------------------"
echo "FIO test completed"
echo "--------------------------------------------------------------------------------"
./blobfuse2 unmount all