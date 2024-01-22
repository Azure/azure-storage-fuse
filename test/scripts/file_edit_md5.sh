#!/bin/bash

echo "Pre start cleanup"

rm -rf ./localfile*
rm -rf /usr/blob_mnt/remotefile*
rm -rf ./md5sum*

list="1 2 3 5 10 100 101 200"

echo "Creating local files"
for i in $list; do echo $i; done | parallel --will-cite -j 5 'head -c {}M < /dev/urandom > ./localfile_{}'
md5sum ./localfile_* > ./md5sum_local.txt

echo "Creating Remote files"
for i in $list; do echo $i; done | parallel --will-cite -j 5 'cp ./localfile_{} /usr/blob_mnt/remotefile_{}'
md5sum /usr/blob_mnt/remotefile_* > ./md5sum_remote.txt

echo "Comparing local and remote files"
cat ./md5sum_local.txt | cut -d " " -f1 > ./md5sum_local.txt1
cat ./md5sum_remote.txt | cut -d " " -f1 > ./md5sum_remote.txt1
diff ./md5sum_local.txt1 ./md5sum_remote.txt1 > ./md5sum_diff.txt
if [ $? -ne 0 ]; then
    exit 1
fi

head -c 10M < /dev/urandom > ./additionaldata

echo "Editing local files"
for i in $list; do echo $i; done | parallel --will-cite -j 5 'cat  ./additionaldata >> ./localfile_{}'
md5sum ./localfile_* > md5sum_local_edited.txt

echo "Editing Remote files"
for i in $list; do echo $i; done | parallel --will-cite -j 5 'cat  ./additionaldata >> /usr/blob_mnt/remotefile_{}'
md5sum /usr/blob_mnt/remotefile_* > md5sum_remote_edited.txt

echo "Comparing Local and Remote files post editing"
cat ./md5sum_local_edited.txt | cut -d " " -f1 > ./md5sum_local_edited.txt1
cat ./md5sum_remote_edited.txt | cut -d " " -f1 > ./md5sum_remote_edited.txt1
diff ./md5sum_local_edited.txt1 ./md5sum_remote_edited.txt1 > ./md5sum_edited_diff.txt
if [ $? -ne 0 ]; then
    exit 1
fi
