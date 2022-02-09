
#!/bin/bash


in_file="/home/vibhansa/blob_mnt/testfile$1"
#in_file="/testhdd/blob_mnt/testfile$1"

echo .
echo "Reading back"
time dd if=$in_file of=/dev/null bs=1M

