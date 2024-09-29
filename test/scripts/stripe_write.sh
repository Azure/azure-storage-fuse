#!/bin/bash

#
# Stress test for various write optimizations:
#
# 1. Multiwrite.
# 2. Dynamic block size.
# 3. Blocklist compaction. For testing blocklist compaction it's best to change the pressure threshold DCs to fairly
#    low values 100/200/400/1000.
#

# pick which test to run.
test_plain_multiwrite=true
test_dynamicblock_multiwrite=true
test_blocklist_compaction=true

# This is the mount directory.
TMPDIR=/blob_mnt

# 1GB file
MAX_4K_BLOCKS=$(expr 300 \* 1024 \* 1)

# Get a random block between 0 and $MAX_4K_BLOCKS
get_rand_block()
{
        shuf -i 0-$MAX_4K_BLOCKS -n 1
}

get_max4k_block()
{
        # Try file sizes from 16K to 160MB.
        shuf -i 4-40960 -n 1
}

#
# For compaction test we create larger files to test reasonably.
#
get_max4k_block_compaction()
{
        # Try file sizes from 16K to 10GB.
        shuf -i 4-2621439 -n 1
}

get_truncate_size()
{
        declare -a truncate_sizes
        # Don't make it less than 1G
        truncate_sizes=("1G" "19G" "20G" "21G" "25G" "50G" "100G" "200G" "400G" "410G" "450G" "500G" "1T" "2T" "3T")
        count=$(echo ${#truncate_sizes[@]})
        let count--
        idx=$(shuf -i 0-$count -n 1)
        echo ${truncate_sizes[$idx]}
}

get_compaction_file_size()
{
        declare -a compaction_sizes
        # Should not be more than ref file size.
        compaction_sizes=("100M" "400M" "1G" "2G" "4G" "5G" "10G")
        count=$(echo ${#compaction_sizes[@]})
        let count--
        idx=$(shuf -i 0-$count -n 1)
        echo ${compaction_sizes[$idx]}
}

# Get random 256 blocks that we will write.
get_random_blocklist()
{
        blocklist=""

        #for((j=0; j<1024; j++))
        for((j=0; j<256; j++))
        {
            blocklist="$blocklist $(get_rand_block)"
        }
        echo $blocklist
}

# 10GB reference file
ref=$TMPDIR/ref_file.txt
ref_desired_size=$(expr 1024 \* 1024 \* 1024 \* 10)
ref_size=0

# Skip if already exists.
if [ -s "$ref" ]; then
    ref_size=$(stat -c %s $ref)
fi

if [ "$ref_desired_size" == "$ref_size" ]; then
    echo "Using existing 10GB ref file..."
else
    echo "Creating 10GB ref file..."

    dd if=/dev/urandom of=$TMPDIR/ref.txt bs=10M count=1
    for((i=0;i<1024;i++))
    {
        cat $TMPDIR/ref.txt >> $ref
    }
    echo "Done"
fi


# to avoid "not found" errors in the logs for the first iteration.
touch ioping1.1G
touch ioping2.1G
touch ioping3.1G

errlog=$TMPDIR/errlog.txt
errlog_final=$TMPDIR/errlog_final.txt
> $errlog
> $errlog_final

oflag="oflag=direct"
#oflag=""

for((i=0; i<1; i++))
{
        # We use variable file sizes (16K to 160MB) in every iteration, to test various scenarios.
        MAX_4K_BLOCKS=$(get_max4k_block)

        if $test_plain_multiwrite; then
                # Create 0-byte files to start with.
                rm -f ioping1.1G
                touch ioping1.1G
                truncate -s 200M ioping1.1G
                #dd if=$ref of=ioping1.1G bs=100M count=1 oflag=direct
                #dd if=$ref of=ioping1.1G bs=512K count=400 &

                rm -f ioping2.1G
                touch ioping2.1G
                truncate -s 200M ioping2.1G
                #dd if=$ref of=ioping2.1G bs=100M count=1 oflag=direct
                #dd if=$ref of=ioping2.1G bs=512K count=400 &

                rm -f ioping3.1G
                touch ioping3.1G
                #truncate -s 200M ioping3.1G
                dd if=/dev/zero of=ioping3.1G bs=512K count=400 &
                #dd if=$ref of=ioping3.1G bs=100M count=1 oflag=direct
                #dd if=$ref of=ioping3.1G bs=512K count=400 &

                echo "Waiting for the 3 writes to complete ..."
                wait
                echo "Done"
        fi

        if $test_dynamicblock_multiwrite; then
                # Now create two files for testing dynamic block sizes.
                # All are same size, pick any one.
                orig_size=200M
                trunc_size=$(get_truncate_size)

                # First truncate to a possibly large size to get large size zero blocks.
                # Then truncate it back to the final size so that the dd's we do later work on small block sizes.
                # Help us test conversion of large to small blocks.
                rm -f ioping4.1G
                truncate -s $trunc_size ioping4.1G
                dd if=$ref of=ioping4.1G conv=notrunc bs=4K count=2 $oflag seek=2000 skip=2000
                truncate -s $orig_size ioping4.1G

                rm -f ioping5.1G
                truncate -s $trunc_size ioping5.1G
                dd if=$ref of=ioping5.1G conv=notrunc bs=4K count=2 $oflag seek=2000 skip=2000
                truncate -s $orig_size ioping5.1G
        fi

        if $test_blocklist_compaction; then
                # Now create two files for testing compaction.
                compact_size=$(get_compaction_file_size)

                rm -f ioping6.1G
                truncate -s $compact_size ioping6.1G
                
                rm -f ioping7.1G
                truncate -s $compact_size ioping7.1G
        fi


        blocklist=$(get_random_blocklist)

        if $test_plain_multiwrite; then
                # Shuffle block list once and write.
                blocklist1=$(shuf -e $blocklist)
                for seek in $(echo $blocklist1); do
                        #echo seek=$seek
                        (dd if=$ref of=ioping1.1G conv=notrunc bs=4K count=2 $oflag seek=$seek skip=$seek || echo "dd ret=$? ioping1.1G seek=$seek, failed") >> $errlog 
                done
                #dd if=$ref of=ioping1.1G conv=notrunc bs=10M count=1 oflag=append &
                
                # Shuffle block list once more and write.
                blocklist2=$(shuf -e $blocklist)
                if [ "$blocklist2" = "$blocklist1" ]; then
                        date
                        echo "Shuffled blocklists are same"
                        exit 2
                fi

                for seek in $(echo $blocklist2); do
                        #echo seek=$seek
                       (dd if=$ref of=ioping2.1G conv=notrunc bs=4K count=2 $oflag seek=$seek skip=$seek || echo "dd ret=$? ioping2.1G seek=$seek, failed") >> $errlog 
                done
                #dd if=$ref of=ioping2.1G conv=notrunc bs=10M count=1 oflag=append &

                # Shuffle block list once more and write.
                blocklist3=$(shuf -e $blocklist)
                for seek in $(echo $blocklist3); do
                        #echo seek=$seek
                       (dd if=$ref of=ioping3.1G conv=notrunc bs=4K count=2 $oflag seek=$seek skip=$seek || echo "dd ret=$? ioping3.1G seek=$seek, failed") >> $errlog 
                done
                #dd if=$ref of=ioping3.1G conv=notrunc bs=10M count=1 oflag=append &
        fi

        #
        # ioping4 and ioping5 are used for dynamic block testing.
        # They start with potentially large block size (zero blocks of large size) and then they are truncated to
        # smaller size so that blocks used by writes caused by following dd's are small.
        # This helps test handling of larger to smaller blocks conversion.
        #
        if $test_dynamicblock_multiwrite; then
                # Shuffle block list once more and write.
                blocklist4=$(shuf -e $blocklist)
                for seek in $(echo $blocklist4); do
                        #echo seek=$seek
                       (dd if=$ref of=ioping4.1G conv=notrunc bs=4K count=2 $oflag seek=$seek skip=$seek || echo "dd ret=$? ioping4.1G seek=$seek, failed") >> $errlog 
                done

                # Shuffle block list once more and write.
                blocklist5=$(shuf -e $blocklist)
                for seek in $(echo $blocklist5); do
                        #echo seek=$seek
                       (dd if=$ref of=ioping5.1G conv=notrunc bs=4K count=2 $oflag seek=$seek skip=$seek || echo "dd ret=$? ioping5.1G seek=$seek, failed") >> $errlog 
                done
        fi

        #
        # ioping6 and ioping7 are used for blocklist compaction testing.
        # They need larger test files, so bump up MAX_4K_BLOCKS.
        #
        MAX_4K_BLOCKS=$(get_max4k_block_compaction)
        blocklist=$(get_random_blocklist)

        if $test_blocklist_compaction; then
                # Shuffle block list and write.
                blocklist6=$(shuf -e $blocklist)
                for seek in $(echo $blocklist6); do
                        #echo seek=$seek
                        # Compaction test file can be large so we cannot keep skip==seek.
                        #let skip=${seek}%${MAX_4K_BLOCKS}
                        (dd if=$ref of=ioping6.1G conv=notrunc bs=4K count=2 $oflag seek=$seek skip=$seek || echo "dd ret=$? ioping6.1G seek=$seek, failed") >> $errlog 
                done

                # Shuffle block list once more and write.
                blocklist7=$(shuf -e $blocklist)
                for seek in $(echo $blocklist7); do
                        #echo seek=$seek
                        #let skip=${seek}%${MAX_4K_BLOCKS}
                       (dd if=$ref of=ioping7.1G conv=notrunc bs=4K count=2 $oflag seek=$seek skip=$seek || echo "dd ret=$? ioping7.1G seek=$seek, failed") >> $errlog 
                done

        fi


        # Wait for Writes on all files to complete.
        wait

        # If one or more dd's failed, continue. These mostly failed due to failure to spawn them for lack of ram.
        if [ -s $errlog ]; then

            echo >> $errlog_final
            echo "**** Continuing, since one or more dd's failed ****" >> $errlog_final
            echo "--[ $(date) ]-------" >> $errlog_final
            cat $errlog >> $errlog_final
            > $errlog
            echo >> $errlog_final

            cat $errlog_final

            continue
        fi

        echo "========= $i : $(date) ========="
        echo "Using MAX_4K_BLOCKS=$MAX_4K_BLOCKS"

	    sudo sysctl -w vm.drop_caches=3
        echo "Starting md5sum ..."
        # Both writes update the same blocks with same content (albeit in different orders), so they should
        # result in the same file in the end.
        if $test_plain_multiwrite; then
                sum1=$(md5sum ioping1.1G | awk '{print $1}')
                sum2=$(md5sum ioping2.1G | awk '{print $1}')
                sum3=$(md5sum ioping3.1G | awk '{print $1}')
        fi

        if $test_dynamicblock_multiwrite; then
                sum4=$(md5sum ioping4.1G | awk '{print $1}')
                sum5=$(md5sum ioping5.1G | awk '{print $1}')
        fi

        if $test_blocklist_compaction; then
                sum6=$(md5sum ioping6.1G | awk '{print $1}')
                sum7=$(md5sum ioping7.1G | awk '{print $1}')
        fi

        echo "md5sum done."

        echo "sum1=$sum1"
        echo "sum2=$sum2"
        echo "sum3=$sum3"
        echo "sum4=$sum4"
        echo "sum5=$sum5"
        echo "sum6=$sum6"
        echo "sum7=$sum7"

        sleep 10

        if $test_plain_multiwrite; then
                if [ "$sum1" != "$sum2" ]; then
                    date
                    echo "*** md5sum don't match : $sum1 vs $sum2 ***"
                    echo errlog:
                    cat $errlog
                    exit
                fi

                if [ "$sum1" != "$sum3" ]; then
                    date
                    echo "*** +md5sum don't match : $sum1 vs $sum3 ***"
                    echo errlog:
                    cat $errlog
                    exit
                fi
        fi

        if $test_dynamicblock_multiwrite; then
                if [ "$sum4" != "$sum5" ]; then
                    date
                    echo "*** [D] md5sum don't match : $sum4 vs $sum5 ***"
                    echo errlog:
                    cat $errlog
                    exit
                fi
        fi

        if $test_blocklist_compaction; then
                if [ "$sum6" != "$sum7" ]; then
                    date
                    echo "*** [C] md5sum don't match : $sum6 vs $sum6 ***"
                    echo errlog:
                    cat $errlog
                    exit
                fi
        fi
}

