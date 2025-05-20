
import os
import kaggle
import torch
import json
import sys
import time
from transformers import Trainer, TrainingArguments
from transformers import AutoTokenizer, AutoModelForCausalLM
from datasets import load_dataset, Dataset, DatasetDict
from huggingface_hub import login

# Login to Hugging Face using your token
login("<HUGGING FACE TOKEN HERE")
destination = sys.argv[2]

# Step 3: Download and save the LLaMA 7B model locally
print("Checking model existance...")
model_path = sys.argv[1]
model_local_path = destination

print("Loading model.")
start_time = time.time()
tokenizer = AutoTokenizer.from_pretrained(model_path)
model = AutoModelForCausalLM.from_pretrained(model_path)
end_time = time.time()
elapsed = end_time - start_time
print(f"Time taken to load model: {elapsed:.2f} seconds")

# Save the model and tokenizer locally
print("Saving model locally.")
start_time = time.time()
model.save_pretrained(model_local_path)
tokenizer.save_pretrained(model_local_path)
end_time = time.time()
elapsed = end_time - start_time
print(f"Time taken to save model: {elapsed:.2f} seconds")
