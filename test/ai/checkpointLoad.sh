
# This script iterates over list of models and then 
#  'model' as first argument will mean you wish to download the model to a local path or a mounted path
#  nothing as argument then this iterates over the list of models and tries to load it using different caching strategies
# It assumes there is a directory mounted already using master_mount.sh and from there it can get a model subdirectory to mount during the iteration.


rm -rf stats.log
touch stats.log

clear_cache() {
	rm -rf  ~/.cache/huggingface/hub/*
	rm -rf /mnt/hugging_cache/model*
	sudo sysctl -w vm.drop_caches=3
}
	

models=("Qwen/Qwen3-30B-A3B" "mistralai/Mistral-Nemo-Instruct-2407" "mistralai/Mistral-7B-Instruct-v0.2" "meta-llama/Llama-3.1-8B-Instruct" "meta-llama/Llama-Guard-3-8B" "microsoft/Phi-4-reasoning-plus" "EleutherAI/gpt-neox-20b" "facebook/opt-30b")

clear_cache

if [ "$1" == "model" ]
then
	for model_name in "${models[@]}"; do
		echo "--------------------------------------------------------------------" >> stats.log
	        echo "$model_name" >> stats.log
		echo "--------------------------------------------------------------------" >> stats.log

		clear_cache

		# Load from hugging face and then save to given path
		python3 load.py --model_name "$model_name" --dest_path "/mnt/model/$model_name" >> stats.log
		
		clear_cache

		# Again load from checkpoint and measure the time
		last_checkpoint=$(ls -td /mnt/model/${model_name}/* | head -n 1 | xargs -n 1 basename)
		subdir=/mnt/model/${model_name}/${last_checkpoint}
		python3 load.py --checkpoint_path $subdir  >> stats.log
	done
	exit 0
fi


# export AZURE_STORAGE_ACCOUNT=blobfuseaitest2
export AZURE_STORAGE_ACCOUNT=blobfuseuksouthgputest 
export AZURE_STORAGE_AUTH_TYPE="msi"
export AZURE_STORAGE_IDENTITY_CLIENT_ID="ba74e24e-15a1-45cb-be86-9e00a4facac5"

# export AZURE_STORAGE_ACCESS_KEY=<key>

#MOUNT_PATH=/mnt/blobfuse/checkpoint
MOUNT_PATH=/home/blobfuse/mnt

#RAMDISK_PATH=/mnt/cpramdisk
RAMDISK_PATH=/home/blobfuse/cpramdisk

#HUG_CACHE=/mnt/hugging_cache/
HUG_CACHE=/home/blobfuse/hugcache

COMMON_ARGS="--container-name="models" --log-type base --log-level=LOG_ERR --log-file-path=./checkpoint_blobfuse2.log"

# Unmount Ramdisk and recreate it
fusemode=("file-cache" "block-cache" "preload")
for mode in "${fusemode[@]}"; do

	MODE_ARGS=""
	wait_time=0

	if [ "$mode" == "file-cache" ]
	then
		#MODE_ARGS="--tmp-path=/mnt/blobfuse/cache --file-cache-timeout=0"
		MODE_ARGS="--tmp-path=$RAMDISK_PATH --file-cache-timeout=120"
	elif [ "$mode" == "block-cache" ]
	then
		MODE_ARGS="--block-cache"
	elif [ "$mode" == "preload" ]
	then
		#MODE_ARGS="--preload --tmp-path=/mnt/blobfuse/cache"
		MODE_ARGS="--preload --tmp-path=$RAMDISK_PATH"
		wait_time=25
	fi


	for model_name in "${models[@]}"; do
		# Unmount blobfuse
		blobfuse2 unmount $MOUNT_PATH
		sleep 3

		sudo rm -rf $RAMDISK_PATH/*
		sudo umount -f $RAMDISK_PATH
		sudo mount -t tmpfs -o rw,size=200G tmpfs $RAMDISK_PATH

		clear_cache

		sudo rm -rf $MOUNT_PATH/*
		sudo rm -rf /mnt/blobfuse/cache/*

		last_checkpoint=$(ls -td /mnt/blobfuse/mnt/${model_name}/* | head -n 1 | xargs -n 1 basename)
		subdir=${model_name}/${last_checkpoint}

		echo "--------------------------------------------------------------------" >> stats.log
		echo "$model_name" >> stats.log
		echo "--------------------------------------------------------------------" >> stats.log
		
		echo "Mounting $subdir in $mode mode : ($MODE_ARGS)" >> stats.log
		blobfuse2 mount $MOUNT_PATH $COMMON_ARGS $MODE_ARGS --subdirectory=$subdir

		if [ $? -ne 0 ]; then
			echo "Failed to mount $subdir"
			exit 1
		fi

		echo "Cool down time"
		sleep $wait_time

		if [ "$mode" == "preload" ]
		then
			latest_log=$(grep "100.00%" ./checkpoint_blobfuse2.log | tail -n 1)
			files_done=$(echo "$latest_log" | grep -oP '\d+ Done' | awk '{print $1}')
			bytes_transferred=$(echo "$latest_log" | grep -oP 'Bytes transferred \d+' | awk '{print $3}')
			data_transferred_gb=$(echo "scale=2; $bytes_transferred / 1024 / 1024 / 1024" | bc)
			time_taken=$(echo "$latest_log" | grep -oP 'Time: \d+' | awk '{print $2}')

			avg_speed_gbps=$(echo "scale=2; ($data_transferred_gb * 8) / $time_taken" | bc)
			echo "Preload summary: Model of size ${data_transferred_gb} GB loaded in ${time_taken} seconds. Files: ${files_done} Speed: ${avg_speed_gbps} Gbps" >> stats.log
		fi

		echo "Running script now"

		python3 load.py --checkpoint_path $MOUNT_PATH --cache_path $HUG_CACHE --device cpu  >> stats.log
	done
done

blobfuse2 unmount $MOUNT_PATH
sleep 3
sudo rm -rf $RAMDISK_PATH/*
sudo umount -f $RAMDISK_PATH


