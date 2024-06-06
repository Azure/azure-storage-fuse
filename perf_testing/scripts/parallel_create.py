import os
import subprocess
import concurrent.futures
import time
import multiprocessing
import argparse

def create_file(file_index, folder):
    timestamp = int(time.time())  # Get current timestamp
    filename = os.path.join(folder, f'20GFile_{timestamp}_{file_index}')
    command = f"dd if=/dev/zero of={filename} bs=16M count=1280 oflag=direct"
    start_time = time.time()
    subprocess.run(command, shell=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    end_time = time.time()
    return (filename, end_time - start_time)

def main(folder, num_files):
    if not os.path.exists(folder):
        os.makedirs(folder)

    start_time = time.time()
    with concurrent.futures.ThreadPoolExecutor(max_workers=multiprocessing.cpu_count()) as executor:
        futures = [executor.submit(create_file, i, folder) for i in range(num_files)]
        results = [f.result() for f in concurrent.futures.as_completed(futures)]
    end_time = time.time()

    total_time = end_time - start_time
    total_data_written = num_files * 20  # in GB
    speed_gbps = (total_data_written * 8) / total_time  # converting GB to Gb and then calculating Gbps

    print(f"Number of files written: {num_files}")
    print(f"Total amount of data written: {total_data_written} GB")
    print(f"Total time taken: {total_time:.2f} seconds")
    print(f"Speed: {speed_gbps:.2f} Gbps")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Create multiple 20GB files in parallel.')
    parser.add_argument('folder', type=str, help='The folder where the files will be written.')
    parser.add_argument('num_files', type=int, help='The number of 20GB files to create.')

    args = parser.parse_args()
    main(args.folder, args.num_files)