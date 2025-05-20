import os
import json
import sys
import time
import threading
import argparse
from datasets import load_dataset, Dataset, DatasetDict

# CLI parameters
parser = argparse.ArgumentParser(description="Test script to load the model")
parser.add_argument('--data_path', type=str, help='Training data set path')
parser.add_argument('--subset', type=str, help='Subset of data to be loaded')
parser.add_argument('--dest_path', type=str, help='Directory to store the load data')
args = parser.parse_args()

data_path = args.data_path
subset = args.subset
dest_path = args.dest_path

# ------- MODEL STUFF -------------------
start_time = time.time()
dataset = load_dataset(data_path, subset, num_proc=25)
end_time = time.time()

elapsed_time = end_time - start_time
print(f"Loading {data_path} took {elapsed_time: .2f} seconds")


if dest_path is not None:
    print("Saving data locally.")
    start_time = time.time()
    dataset.save_to_disk(dest_path, num_proc=25)
    end_time = time.time()
    elapsed = end_time - start_time
    print(f"Time taken to save data: {elapsed:.2f} seconds")
	

