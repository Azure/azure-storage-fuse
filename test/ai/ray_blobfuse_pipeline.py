# python3 -m pip install --upgrade pip
# pip install ray transformers datasets torch
# pip install --upgrade ray pyarrow

import os
import sys
import time
import argparse
import shutil
import torch
import ray
from ray.train.torch import TorchTrainer
from ray.train import ScalingConfig
from datasets import load_dataset
from transformers import AutoModelForSequenceClassification, AutoTokenizer

# --- CONFIGURATION & CLI ---
parser = argparse.ArgumentParser()
parser.add_argument("--prepare", action="store_true", help="Download data/model and upload to BlobFuse")
parser.add_argument("--prefetch", action="store_true", help="Enable BlobFuse prefetching logic")
parser.add_argument("--gpu", action="store_true", help="Use GPU for training")
parser.add_argument("--workers", type=int, default=2, help="Number of Ray workers")
parser.add_argument("--model-name", type=str, default="distilbert-base-uncased", help="Hugging Face model ID to download (e.g., bert-large-uncased)")
parser.add_argument("--train-size", type=str, default="train[:100]", help="Subset spec for dataset split (e.g., train[:5000])")
parser.add_argument("--mount-path", type=str, default="/blob_mnt", help="BlobFuse mount path")

args = parser.parse_args()

MOUNT_PATH = args.mount_path
DATA_DIR = os.path.join(MOUNT_PATH, "dataset")
MODEL_DIR = os.path.join(MOUNT_PATH, "base_model")
CHECKPOINT_DIR = os.path.join(MOUNT_PATH, "worker_checkpoints")
FINAL_MODEL_DIR = os.path.join(MOUNT_PATH, "final_model_output")

# --- PHASE 1: DATA PREPARATION (Optional) ---
if args.prepare:
    print("--- PREPARATION PHASE: Downloading from Hugging Face ---")
    os.makedirs(DATA_DIR, exist_ok=True)
    os.makedirs(MODEL_DIR, exist_ok=True)
    
    # Download dataset subset and save as individual text files
    dataset = load_dataset("imdb", split=args.train_size)
    for i, item in enumerate(dataset):
        file_path = os.path.join(DATA_DIR, f"sample_{i}.txt")
        with open(file_path, "w") as f:
            f.write(item["text"])
    
    # Save Model/Tokenizer locally to BlobFuse
    model_name = args.model_name
    tokenizer = AutoTokenizer.from_pretrained(model_name)
    model = AutoModelForSequenceClassification.from_pretrained(model_name)
    tokenizer.save_pretrained(MODEL_DIR)
    model.save_pretrained(MODEL_DIR)
    print(f"Data and Model uploaded to {MOUNT_PATH}")
    sys.exit(0)

# --- PHASE 2: DISTRIBUTED TRAINING FUNCTION ---
def train_func_per_worker(config):
    # 1. Get Shard
    shard = ray.train.get_dataset_shard("train_set")
    local_files = [row["file_path"] for row in shard.iter_rows()]
    worker_id = ray.train.get_context().get_world_rank()
    
    # 2. Prefetch Hinting
    if args.prefetch:
        hint_file = os.path.join(MOUNT_PATH, ".prefetch_queue")
        try:
            with open(hint_file, "a") as f:
                f.write("\n".join(local_files) + "\n")
            print(f"Worker {worker_id}: Prefetch hints sent.")
        except: pass

    # 3. Load Model from BlobFuse (not internet)
    device = "cuda" if args.gpu and torch.cuda.is_available() else "cpu"
    model = AutoModelForSequenceClassification.from_pretrained(MODEL_DIR).to(device)

    # 4. Training Loop (Simulation or Real)
    io_wait_total = 0
    start_time = time.perf_counter()

    for path in local_files:
        # Measure I/O Latency
        io_s = time.perf_counter()
        with open(path, "r") as f:
            _ = f.read()
        io_wait_total += (time.perf_counter() - io_s)

        if device == "cpu":
            time.sleep(5) # Simulation
        else:
            # Placeholder for actual GPU forward/backward pass
            pass

    # 5. Save Worker Checkpoint to Mount
    worker_ckpt_path = os.path.join(CHECKPOINT_DIR, f"worker_{worker_id}")
    os.makedirs(worker_ckpt_path, exist_ok=True)
    model.save_pretrained(worker_ckpt_path)
    
    total_time = time.perf_counter() - start_time
    print(f"Worker {worker_id} Finished. I/O Wait: {io_wait_total:.2f}s / Total: {total_time:.2f}s")

# --- PHASE 3: ORCHESTRATION ---
if __name__ == "__main__":
    ray.init()
    
    # Cleanup old checkpoints
    if os.path.exists(CHECKPOINT_DIR):
        shutil.rmtree(CHECKPOINT_DIR)
    os.makedirs(CHECKPOINT_DIR)

    # Create Ray Dataset from files in BlobFuse
    all_files = [os.path.join(DATA_DIR, f) for f in os.listdir(DATA_DIR) if f.endswith(".txt")]
    ds = ray.data.from_items([{"file_path": f} for f in all_files])

    # Distributed Training
    trainer = TorchTrainer(
        train_func_per_worker,
        datasets={"train_set": ds},
        scaling_config=ScalingConfig(num_workers=args.workers, use_gpu=args.gpu)
    )
    trainer.fit()

    # PHASE 4: HEAD NODE CONSOLIDATION
    print("--- CONSOLIDATION PHASE: Creating Final Model ---")
    # In a real scenario, you'd average weights. Here we take Worker 0's result as the 'final' model.
    if not os.path.exists(FINAL_MODEL_DIR):
        os.makedirs(FINAL_MODEL_DIR)
    
    source_ckpt = os.path.join(CHECKPOINT_DIR, "worker_0")
    if os.path.exists(source_ckpt):
        for item in os.listdir(source_ckpt):
            shutil.copy(os.path.join(source_ckpt, item), os.path.join(FINAL_MODEL_DIR, item))
        print(f"Final model consolidated and saved to: {FINAL_MODEL_DIR}")

    ray.shutdown()
