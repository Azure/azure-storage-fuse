#!/bin/bash
set -e

output="./results_bandwidth"
iterations=3
mount_dir=$1

rm -rf ./blobfuse2.log
rm -rf ${output}
mkdir -p ${output}
chmod 777 ${output}

mount_blobfuse
rm -rf ${mount_dir}/*
./blobfuse2 unmount all

mount_blobfuse() {
  set +e
  ./blobfuse2 mount ${mount_dir} --config-file=./config.yaml
  mount_status=$?
  set -e
  if [ $mount_status -ne 0 ]; then
    echo "Failed to mount file system"
    exit 1
  fi
  sleep 3
}

execute_test() {
  job_file=$1
  bench_file=$2
  log_dir=$4

  job_name=$(basename "${job_file}")
  job_name="${job_name%.*}"

  echo -n "Running job ${job_name} for ${iterations} iterations... "

  for i in $(seq 1 $iterations);
  do
    echo -n "${i};"
    set +e
    timeout 300s fio --thread \
      --output=${output}/${job_name}trial${i}.json \
      --output-format=json \
      --directory=${mount_dir} \
      --filename=${bench_file}${i} \
      --eta=never \
      ${job_file}
    job_status=$?
    set -e
    if [ $job_status -ne 0 ]; then
      echo "Job ${job_name} failed : ${job_status}"
      exit 1
    fi
  done

  jq -n 'reduce inputs.jobs[] as $job (null; .name = $job.jobname | .len += 1 | .value += (if ($job."job options".rw == "read")
      then $job.read.bw / 1024
      elif ($job."job options".rw == "randread") then $job.read.bw / 1024
      elif ($job."job options".rw == "randwrite") then $job.write.bw / 1024
      else $job.write.bw / 1024 end)) | {name: .name, value: (.value / .len), unit: "MiB/s"}' ${output}/${job_name}trial*.json | tee ${output}/${job_name}_summary.json
}

read_fio_benchmark () {
  jobs_dir=./benchmark/fio_read_config

  for job_file in "${jobs_dir}"/*.fio; do
    job_name=$(basename "${job_file}")
    job_name="${job_name%.*}"

    echo "Running Read benchmark for ${job_name}"
    mount_blobfuse

    execute_test $job_file ${job_name}.dat

    ./blobfuse2 unmount all
    sleep 5
  done
}

write_fio_benchmark () {
  jobs_dir=./benchmark/fio_write_config

  for job_file in "${jobs_dir}"/*.fio; do
    job_name=$(basename "${job_file}")
    job_name="${job_name%.*}"
    
    echo "Running Write benchmark for ${job_name}"
    mount_blobfuse

    execute_test $job_file ${job_name}.dat

    ./blobfuse2 unmount all
    sleep 5
  done
}

write_fio_benchmark
read_fio_benchmark

# combine all bench results into one json file
jq -n '[inputs]' ${output}/*_summary.json | tee ./benchmark/results.json