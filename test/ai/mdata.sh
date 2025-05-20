
# Script to download differnet type of datasets
rm -rf stats.log


echo "--------------------------------------------------------------------" >> stats.log
echo "cosmopedia" >> stats.log
echo "--------------------------------------------------------------------" >> stats.log
./data.sh "hugging" "cosmopedia"
python3 datasetDownload.py --data_path "HuggingFaceTB/cosmopedia" --subset "web_samples_v1" >> stats.log

./data.sh "hugging" "nvidia/OpenMathReasoning"
python3 datasetDownload.py --data_path "nvidia/OpenMathReasoning" --subset "default" >> stats.log

models=("cosmopedia" "nvidia/OpenMathReasoning")
for model_name in "${models[@]}"; do
	./data.sh "file-cache" "cosmopedia" >> stats.log
	python3 datasetDownload.py --data_path "/mnt/blobfuse/mnt" --subset "default" >> stats.log
	echo "--------------------------------------------------------------------" >> stats.log
	
	./data.sh "block-cache" "cosmopedia" >> stats.log
	python3 datasetDownload.py --data_path "/mnt/blobfuse/mnt" --subset "default" >> stats.log
	echo "--------------------------------------------------------------------" >> stats.log
	
	./data.sh "preload" "cosmopedia" >> stats.log
	python3 datasetDownload.py --data_path "/mnt/blobfuse/mnt" --subset "default" >> stats.log
	echo "--------------------------------------------------------------------" >> stats.log
	
	./data.sh "preload" "cosmopedia" 10 >> stats.log
	python3 datasetDownload.py --data_path "/mnt/blobfuse/mnt" --subset "default" >> stats.log
	echo "--------------------------------------------------------------------" >> stats.log
	
	./data.sh "ramdisk" "cosmopedia" 10 >> stats.log
	python3 datasetDownload.py --data_path "/mnt/blobfuse/mnt" --subset "default" >> stats.log
	echo "--------------------------------------------------------------------" >> stats.log
done
