

for i in {1..500}
do
	rm -rf $1/* 
    out=`python multi_open_test.py`

    cnt=`echo $out | grep 5000 | wc -c`
    if [ "$cnt" = "0" ]
    then
        echo "  $i : Test case Failed  "
        echo $out
    else
        echo -n "."
    fi

    python blobrace.py /home/vikas/blob_mnt/blobfuse.log
done
