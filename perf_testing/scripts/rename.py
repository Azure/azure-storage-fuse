import datetime
import os
import time
import shutil
import json

# Function to create unique folder for each run
def create_folder(folder_path):
    if os.path.exists(folder_path):
        shutil.rmtree(folder_path)
    os.makedirs(folder_path)

# Function to create files of a specified size
def create_files(folder_path, num_files, file_size):
    start_time = time.time()
    for i in range(num_files):
        file_path = os.path.join(folder_path, f"file_{i}.txt")
        with open(file_path, 'wb') as f:
            f.write(b'\0' * file_size)
    end_time = time.time()
    return end_time - start_time

# Function to rename files
def rename_files(folder_path):
    start_time = time.time()
    for i, filename in enumerate(os.listdir(folder_path)):
        old_file_path = os.path.join(folder_path, filename)
        new_file_path = os.path.join(folder_path, f"new_file_{i}.txt")
        os.rename(old_file_path, new_file_path)
    end_time = time.time()
    return end_time - start_time

# Specify the folder path
base_folder = "./"
timestamp = datetime.datetime.now().strftime("%Y%m%d%H%M%S")
folder_path = os.path.join(base_folder, f"test_folder_{timestamp}")

# Specify the number of files
num_files = 5000

# Specify the file size in bytes (1 MB)
file_size = 1024 * 1024

# Output file
output_file = "output.txt"

# Create unique folder for each run
create_folder(folder_path)

# Measure the time taken to create files
create_time = create_files(folder_path, num_files, file_size)
# print(f"Time taken to create {num_files} files: {create_time:.4f} seconds")

# Measure the time taken to rename files
rename_time = rename_files(folder_path)
# print(f"Time taken to rename {num_files} files: {rename_time:.4f} seconds")

# Clear the test data
shutil.rmtree(folder_path)

print(json.dumps({"name": "rename_5000_1MB_files", "rename_time": rename_time, "create_time": create_time, "unit": "seconds"}))
