#!/bin/bash
set -e

# Each test will be performed 3 times
iterations=3

# Mount path for blobfuse is supplied on command line while executing this script
mount_dir=$1

# Name of tests we are going to perform
test_name=$2

# Directory where output logs will be generated by fio
output="./${test_name}"

# Additional mount parameters
log_type="syslog"
log_level="log_err"
cache_path=""

# --------------------------------------------------------------------------------------------------
# Method to mount blobfuse and wait for system to stabilize
mount_blobfuse() {
  set +e

  blobfuse2 mount ${mount_dir} --config-file=./config.yaml --log-type=${log_type} --log-level=${log_level} ${cache_path}
  mount_status=$?
  set -e
  if [ $mount_status -ne 0 ]; then
    echo "Failed to mount file system"
    exit 1
  else
    echo "File system mounted successfully on ${mount_dir}"
  fi

  # Wait for daemon to come up and stablise
  sleep 5

  df -h | grep blobfuse
  df_status=$?
  if [ $df_status -ne 0 ]; then
    echo "Failed to find blobfuse mount"
    exit 1
  else
    echo "File system stable now on ${mount_dir}"
  fi
}

# --------------------------------------------------------------------------------------------------
# Method to execute fio command for a given config file and generate summary result
execute_test() {
  job_file=$1

  job_name=$(basename "${job_file}")
  job_name="${job_name%.*}"

  echo -n "Running job ${job_name} for ${iterations} iterations... "

  for i in $(seq 1 $iterations);
  do
    echo -n "${i};"
    set +e

    timeout 300m fio --thread \
      --output=${output}/${job_name}trial${i}.json \
      --output-format=json \
      --directory=${mount_dir} \
      --eta=never \
      ${job_file}

    job_status=$?
    set -e
    if [ $job_status -ne 0 ]; then
      echo "Job ${job_name} failed : ${job_status}"
      exit 1
    fi
  done

  # From the fio output get the bandwidth details and put it in a summary file
  jq -n 'reduce inputs.jobs[] as $job (null; .name = $job.jobname | .len += 1 | .value += (if ($job."job options".rw == "read")
      then $job.read.bw / 1024
      elif ($job."job options".rw == "randread") then $job.read.bw / 1024
      elif ($job."job options".rw == "randwrite") then $job.write.bw / 1024
      else $job.write.bw / 1024 end)) | {name: .name, value: (.value / .len), unit: "MiB/s"}' ${output}/${job_name}trial*.json | tee ${output}/${job_name}_bandwidth_summary.json

  # From the fio output get the latency details and put it in a summary file
  jq -n 'reduce inputs.jobs[] as $job (null; .name = $job.jobname | .len += 1 | .value += (if ($job."job options".rw == "read")
      then $job.read.lat_ns.mean / 1000000
      elif ($job."job options".rw == "randread") then $job.read.lat_ns.mean / 1000000
      elif ($job."job options".rw == "randwrite") then $job.write.lat_ns.mean / 1000000
      else $job.write.lat_ns.mean / 1000000 end)) | {name: .name, value: (.value / .len), unit: "milliseconds"}' ${output}/${job_name}trial*.json | tee ${output}/${job_name}_latency_summary.json
}

# --------------------------------------------------------------------------------------------------
# Method to iterate over fio files in given directory and execute each test
iterate_fio_files() {
  jobs_dir=$1
  job_type=$(basename "${jobs_dir}")

  for job_file in "${jobs_dir}"/*.fio; do
    job_name=$(basename "${job_file}")
    job_name="${job_name%.*}"
    
    mount_blobfuse
    
    execute_test $job_file

    blobfuse2 unmount all
    sleep 5

    rm -rf ~/.blobfuse2/*
  done
}

# --------------------------------------------------------------------------------------------------
# Method to list files on the mount path and generate report
list_files() {
  # Mount blobfuse and creat files to list
  mount_blobfuse
  total_seconds=0

  # List files and capture the time related details
  cd ${mount_dir}
  /usr/bin/time -o lst.txt -v ls -U --color=never  > /dev/null 
  ce -
  cat lst.txt

  # Extract Elapsed time for listing files
  list_time=`cat lst.txt | grep "Elapsed" | rev | cut -d " " -f 1 | rev`
  echo $list_time

  IFS=':'; time_fragments=($list_time); unset IFS;
  list_min=`printf '%5.5f' ${time_fragments[0]}`
  list_sec=`printf '%5.5f' ${time_fragments[1]}`

  avg_list_time=`printf %5.5f $(echo "scale = 10; ($list_min * 60) + $list_sec" | bc)`

  # ------------------------------
  # Measure time taken to delete these files
  cd ${mount_dir}
  /usr/bin/time -o del.txt -v find . -name "create_1l_files_in_20_threads*" -delete > /dev/null 
  cd -
  cat del.txt

  # Extract Deletion time 
  del_time=`cat del.txt | grep "Elapsed" | rev | cut -d " " -f 1 | rev`
  echo $del_time

  IFS=':'; time_fragments=($del_time); unset IFS;
  del_min=`printf '%5.5f' ${time_fragments[0]}`
  del_sec=`printf '%5.5f' ${time_fragments[1]}`
  
  avg_del_time=`printf %5.5f $(echo "scale = 10; ($del_min * 60) + $del_sec" | bc)`

  # Unmount and cleanup now
  blobfuse2 unmount all

  echo $avg_list_time " : " $avg_del_time

  jq -n --arg list_time $avg_list_time --arg del_time $avg_del_time '{name: "list_100k_files", value: $list_time, unit: "seconds"},
      {name: "delete_100k_files", value: $del_time, unit: "seconds"}' | tee ${output}/list_results.json
}


# --------------------------------------------------------------------------------------------------
# Method to prepare the system for test
prepare_system() {
  blobfuse2 unmount all
  # Clean up logs and create output directory
  mkdir -p ${output}
  chmod 777 ${output}
}


# --------------------------------------------------------------------------------------------------
# Prepare the system for test
prepare_system

# --------------------------------------------------------------------------------------------------
executed=1
if [[ ${test_name} == "write" ]] 
then
  # Execute write benchmark using fio
  echo "Running Write test cases"
  cache_path="--block-cache-path=/mnt/tempcache"
  iterate_fio_files "./perf_testing/config/write" 
  
elif [[ ${test_name} == "read" ]] 
then
  # Execute read benchmark using fio
  echo "Running Read test cases"
  iterate_fio_files "./perf_testing/config/read" 
elif [[ ${test_name} == "highlyparallel" ]] 
then
  # Execute multi-threaded benchmark using fio
  echo "Running Highly Parallel test cases"
  cache_path="--block-cache-path=/mnt/tempcache"
  iterate_fio_files "./perf_testing/config/high_threads"
elif [[ ${test_name} == "create" ]] 
then  
  # Set log type to silent as this is going to generate a lot of logs
  log_type="silent"
  iterations=1

  # Pre creation cleanup
  mount_blobfuse
  echo "Deleting old data"
  cd ${mount_dir}
  find . -name "create_1000_files_in_10_threads*" -delete  
  find . -name "create_1000_files_in_100_threads*" -delete  
  find . -name "create_1l_files_in_20_threads*" -delete  
  cd -
  ./blobfuse2 unmount all

  # Execute file create tests
  echo "Running Create test cases"
  iterate_fio_files "./perf_testing/config/create" 
elif [[ ${test_name} == "list" ]] 
then 
  # Set log type to silent as this is going to generate a lot of logs
  log_type="silent"
  
  # Execute file listing tests
  echo "Running File listing test cases"
  list_files 
  
  # No need to generate bandwidth or latecy related reports in this case
  executed=0  
else
  executed=0  
  echo "Invalid argument. Please provide either 'read', 'write', 'multi' or 'create' as argument"
fi

# --------------------------------------------------------------------------------------------------
if [[ $executed -eq 1 ]] 
then
  # Merge all results and generate a json summary for bandwidth
  jq -n '[inputs]' ${output}/*_bandwidth_summary.json | tee ./${output}/bandwidth_results.json

  # Merge all results and generate a json summary for latency
  jq -n '[inputs]' ${output}/*_latency_summary.json | tee ./${output}/latency_results.json
fi

# --------------------------------------------------------------------------------------------------
