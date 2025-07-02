# Script to download dataset from Kaggle to local disk
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

api = kaggle.KaggleApi()
api.authenticate()

print("Downloading dataset.")
api.dataset_download_files("miguelcalado/resnet50rafa", path="/mnt/blobfuse/mnt/resnet50rafa", unzip=True)


