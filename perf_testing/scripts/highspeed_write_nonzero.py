import os
import shutil
import time
import argparse
import subprocess
import mmap
import io
from multiprocessing import Pool, cpu_count


# Function to create files using dd command
def create_file_dd(file_index, folder, source_file, timestamp):
    filename = os.path.join(folder, f'ddFile_{timestamp}_{file_index}')
    block_size = 1  # in GB
    count = 36
    file_size_gb = (block_size * count)
    
    command = f"dd if={source_file} of={filename} bs={block_size}G count={count} oflag=direct"

    start_time = time.time()
    result = subprocess.run(command, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    end_time = time.time()

    if result.returncode != 0:
        return (filename, 0, 0.0, f"Error creating file {filename}: {result.stderr.decode().strip()}")

    write_time = end_time - start_time
    write_speed = (file_size_gb * 1024) / write_time  # MB/s

    return (filename, write_time, write_speed, file_size_gb, None)

# Main function to handle parallel execution
def main(folder, num_files, source_file):
    if not os.path.exists(folder):
        os.makedirs(folder)

    timestamp = int(time.time())  # Get current timestamp for file naming

    start_time = time.time()
    results = []


    with Pool(processes=cpu_count()) as pool:  # Pool of workers based on the CPU count
        futures=[]
        futures += [pool.apply_async(create_file_dd, (i, folder, source_file, timestamp)) for i in range(num_files)]

        # Collect results from async operations
        for future in futures:
            result = future.get()
            if result[4] is None:  # No error
                results.append(result)
            else:
                print(result[4])  # Print error messages

    end_time = time.time()

    total_time = end_time - start_time
    total_data_written = sum(r[3] for r in results)  # in GB
    speed_gbps = (total_data_written *8 ) / total_time # Convert GB to Gigabits (1 GB = 8 Gb)
    
    throughput = (total_data_written * 1024) / total_time
    print(f"Number of files written: {num_files}")
    print(f"Total amount of data written: {total_data_written:.2f} GB")
    print(f"Total time taken: {total_time:.2f} seconds")
    print(f"Overall Speed: {speed_gbps:.2f} Gbps")
    print(f"Throughput: {throughput:.2f} MiB/s")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Create multiple files using various methods in parallel.')
    parser.add_argument('folder', type=str, help='The folder where the files will be written.')
    parser.add_argument('num_files', type=int, help='The number of files to create.')
    parser.add_argument('source_file', type=str, help='The source file to copy data from.')

    args = parser.parse_args()
    main(args.folder, args.num_files, args.source_file)

#  python3 highspeed_write_nonzero.py <mntPath>~/drs/random_data_test/ <noOfFiles>5 <sourceFile>/mnt/azcopy_test_180GB.log
