#!/bin/bash

# Small file test
echo "Creating small files"
for i in {1..2000}; do echo $i; done | parallel --will-cite -j 20 'head -c 1M < /dev/zero > /home/vibhansa/blob_mnt2/myfile_small_{}'
sleep 5

echo "Reading small files"
for i in {1..2000}; do echo $i; done | parallel --will-cite -j 20 'hexdump /home/vibhansa/blob_mnt2/myfile_small_{} > /dev/null'
sleep 5

rm -rf /home/vibhansa/blob_mnt2/myfile_small*



# Medium file test
#echo "Creating medium files"
#for i in {1..20}; do echo $i; done | parallel --will-cite -j 10 'head -c 200M < /dev/urandom > /home/vibhansa/blob_mnt2/myfile_med_{}'
#sleep 5

#echo "Reading medium files"
#for i in {1..20}; do echo $i; done | parallel --will-cite -j 10 'hexdump /home/vibhansa/blob_mnt2/myfile_med_{} > /dev/null'
#sleep 5

#rm -rf /home/vibhansa/blob_mnt2/myfile_med*



# Large file test
#echo "Creating large files"
#for i in {1..2}; do echo $i; done | parallel --will-cite -j 2 'head -c 1G < /dev/urandom > /home/vibhansa/blob_mnt2/myfile_large_{}'
#sleep 5

#echo "Reading large files"
#for i in {1..2}; do echo $i; done | parallel --will-cite -j 2 'hexdump /home/vibhansa/blob_mnt2/myfile_large_{} > /dev/null'
#sleep 5

#rm -rf /home/vibhansa/blob_mnt2/myfile_large*
