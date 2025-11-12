#!/bin/bash

export OMPI_MCA_btl_tcp_if_include=eth0

# Change these paths as necessary
MOUNT_PATH=/mnt/blob_mnt
BENCHMARK_RESULTS=~/mlperf/benchmark_results

START_HOST_INDEX=1 # Starting index for hostnames, for ccw-hpc-21 to ccw-hpc-30, set this to 21
COUNT=10 # count of number of hosts allocated
EXCLUDE_LIST="" # e.g., "2,5" to exclude hosts 2 and 5
NUM_HOSTS=0

for i in $(seq $START_HOST_INDEX $((START_HOST_INDEX + COUNT - 1))); do
    if [[ $EXCLUDE_LIST =~ (^|,)$i(,|$) ]]; then
        continue
    fi

    node="ccw-hpc-$i"
    HOSTS="${HOSTS}${HOSTS:+,}$node"
    NUM_HOSTS=$((NUM_HOSTS+1))
done

# echo "Hosts: $HOSTS"
# echo "Number of Hosts: $NUM_HOSTS"

mlpstorage training run \
    --hosts $HOSTS \
    --num-client-hosts $NUM_HOSTS \
    --client-host-memory-in-gb 128 \
    --num-accelerators $((NUM_HOSTS*1)) \
    --accelerator-type a100 \
    --model unet3d \
    --data-dir $MOUNT_PATH/unet3d_data \
    --results-dir $BENCHMARK_RESULTS \
    --params dataset.num_files_train=30000 reader.odirect=True
