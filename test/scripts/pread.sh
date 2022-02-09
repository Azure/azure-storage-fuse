
#!/bin/bash


start_time=`date +%s`
time seq 1 $2 | parallel --will-cite -j $1 -I{} time dd if=$4/$3_{}.tst of=/dev/null bs=1M
end_time=`date +%s`

time_diff=$(( $end_time - $start_time ))

if [ $time_diff -eq 0 ]
then
	time_diff=1
fi

total_size=$(($2 * $3 * 8))
rate=$(( $total_size / $time_diff ))

echo "---------------------------------------------------"
echo "Thread : " $1 " Files : " $2 " Size : " $3 " MB"
echo "Download time is        : " $time_diff " Seconds"
echo "Download rate is        : " $rate " Mbps"

echo "---------------------------------------------------" >> results.txt
echo "Thread : " $1 " Files : " $2 " Size : " $3 " MB" >> results.txt
echo "Download time is        : " $time_diff " Seconds" >> results.txt
echo "Download rate is        : " $rate " Mbps" >> results.txt

