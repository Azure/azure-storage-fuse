

logFile="./overnight.log"

echo "Starting Test Suite" > $logFile

for i in {1,2,3}
do
    ./test/scripts/cppparrun.sh /home/AzureUser/mcdhee/mcdhee_fs /home/AzureUser/mcdhee/ramdisk /home/AzureUser/mcdhee/azure-storage-fuse/build/blobfuse /home/AzureUser/myblob.cfg >> $logFile
done



for i in {1,2,3}
do
    echo "fusesa No Empty" >> $logFile
    ./test/scripts/goparrun.sh /home/AzureUser/mcdhee/mcdhee_fs /home/AzureUser/mcdhee/ramdisk ./config_fuse_noemp.yaml >> $logFile
    echo "fusesa Empty" > $logFile
    ./test/scripts/goparrun.sh /home/AzureUser/mcdhee/mcdhee_fs /home/AzureUser/mcdhee/ramdisk ./config_fuse_emp.yaml >> $logFile
    echo "GoFuse No Empty" > $logFile
    ./test/scripts/goparrun.sh /home/AzureUser/mcdhee/mcdhee_fs /home/AzureUser/mcdhee/ramdisk ./config_gofuse_noemp.yaml >> $logFile
    echo "fusesa Empty" > $logFile
    ./test/scripts/goparrun.sh /home/AzureUser/mcdhee/mcdhee_fs /home/AzureUser/mcdhee/ramdisk ./config_gofuse_emp.yaml >> $logFile
done

echo "Ending Test Suite" >> $logFile
