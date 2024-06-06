import os
import sys
import subprocess
import time
from multiprocessing import Pool, cpu_count

def copy_file(src):
    try:
        process = subprocess.Popen(['dd', f'if={src}', 'of=/dev/null', 'bs=4M', 'status=none'], stdout=subprocess.PIPE)
        bytes_transferred = 0
        start_time = time.time()
        for line in process.stdout:
            bytes_transferred += len(line)
            # Calculate speed in Gbps and update live
            elapsed_time = time.time() - start_time
            speed_gbps = (bytes_transferred * 8) / (elapsed_time * 10**9)
            # print(f"\rBytes transferred: {bytes_transferred} bytes | Speed: {speed_gbps:.2f} Gbps", end="")
            sys.stdout.flush()
        process.wait()
        file_size = os.path.getsize(src)
        return file_size
    except subprocess.CalledProcessError as e:
        return 0

def main(file_paths):
    cpu_cores = cpu_count()
    total_size = 0

    start_time = time.time()

    with Pool(cpu_cores) as pool:
        sizes = pool.map(copy_file, file_paths)
        total_size = sum(sizes)

    end_time = time.time()
    time_taken = end_time - start_time

    total_size_gb = total_size / (1024 ** 3)  # Convert bytes to GB
    speed_gbps = (total_size * 8) / (time_taken * 10**9)  # Convert bytes to bits and calculate speed in Gbps

    print(json.dumps({"name": "read_10_20GB_file", "total_time": time_taken, "speed": speed_gbps, "unit": "GiB/s"}))

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python parallel_copy.py <src1> [<src2> ... <srcN>]")
        sys.exit(1)

    file_paths = sys.argv[1:]

    main(file_paths)