import os
import time
import argparse
from transformers import AutoModelForCausalLM, AutoTokenizer

def get_directory_size(directory):
    total_size = 0
    for dirpath, dirnames, filenames in os.walk(directory):
        for f in filenames:
            fp = os.path.join(dirpath, f)
            total_size += os.path.getsize(fp)
    return total_size

def load_model(model_name, cache_path, device):
    """
    Load the model and tokenizer from the specified model name.
    """
    
    print(f"Loading model {model_name}...")
    start_time = time.time()
    
    # Load the tokenizer
    tokenizer = AutoTokenizer.from_pretrained(model_name,
                                              torch_dtype="auto", 
                                              trust_remote_code=True,
                                              use_safetensors=True,
                                              cache_dir=cache_path)
    
    # Load the model
    model = AutoModelForCausalLM.from_pretrained(model_name,
                                                 torch_dtype="auto", 
                                                 trust_remote_code=True,
                                                 use_safetensors=True,
                                                 cache_dir=cache_path).to(device)
    
    end_time = time.time()
    print(f"Model loaded in {end_time - start_time:.2f} seconds.")
        
    return model, tokenizer

def load_checkpoint(checkpoint_path, cache_path, device):
    """
    Load the model and tokenizer from the specified model name.
    """
    
    print(f"Loading model from checkpoint {checkpoint_path}...")
    start_time = time.time()
    
    # Load the tokenizer
    tokenizer = AutoTokenizer.from_pretrained(checkpoint_path,
                                              torch_dtype="auto", 
                                              trust_remote_code=True,
                                              use_safetensors=True,
                                              cache_dir=cache_path)
    
    # Load the model
    model = AutoModelForCausalLM.from_pretrained(checkpoint_path,
                                                 torch_dtype="auto", 
                                                 trust_remote_code=True,
                                                 use_safetensors=True,
                                                 cache_dir=cache_path).to(device)
    
    end_time = time.time()
    
    time_taken = end_time - start_time
    checkpoint_size = get_directory_size(checkpoint_path)
    checkpoint_size_GB = checkpoint_size / (1024 * 1024 * 1024)
    avg_speed_gbps = (checkpoint_size_GB * 8) / time_taken
     
    print(f"Checkpoint loaded from {checkpoint_path} in {time_taken:.2f} seconds. Size: {checkpoint_size_GB:.2f} GB. Bandwidth: {avg_speed_gbps:.2f} Gbps" )
        
    return model, tokenizer


def save_checkpoint(model, tokenizer, dest_path, shard_size):
    """
    Save the model and tokenizer to a checkpoint.
    """
    
    # On dest_path create a directory with current date/time and use that as destination
    timestamp = time.strftime("%Y%m%d-%H%M%S")
    checkpoint_path = os.path.join(dest_path, f"{timestamp}")
    
    # Create the directory if it doesn't exist
    os.makedirs(checkpoint_path, exist_ok=True)
    
    print(f"Taking Checkpoint...")
    start_time = time.time()
    
    # Save the model and tokenizer
    model.save_pretrained(checkpoint_path,  
                          safe_serialization=True, 
                          max_shard_size=shard_size)
    tokenizer.save_pretrained(checkpoint_path,
                              safe_serialization=True, 
                              max_shard_size=shard_size)
    
    end_time = time.time()
    
    time_taken = end_time - start_time
    checkpoint_size = get_directory_size(checkpoint_path)
    checkpoint_size_GB = checkpoint_size / (1024 * 1024 * 1024)
    avg_speed_gbps = (checkpoint_size_GB * 8) / time_taken
     
    print(f"Checkpoint saved to {checkpoint_path} in {time_taken:.2f} seconds. Size: {checkpoint_size_GB:.2f} GB. Bandwidth: {avg_speed_gbps:.2f} Gbps" )
    
    
def main():
    parser = argparse.ArgumentParser(description="Load a model and tokenizer.")
    
    # One of these is a must have parameter
    parser.add_argument("--model_name", type=str, help="Name of the model to load.")
    parser.add_argument("--checkpoint_path", type=str, help="Name of the model to load.")
    parser.add_argument("--dest_path", type=str, help="Name of the model to load.")
    parser.add_argument("--cache_path", type=str, help="Path for the Name of the model to load.")

    # Optional parameters not used casually
    parser.add_argument("--shard_size", type=str, default="5GB", help="Name of the model to load.")
    parser.add_argument("--device", type=str, default="cpu", help="Device to load the model on (e.g., 'cpu', 'cuda').")
    
    # Parse the arguments
    args = parser.parse_args()
    
    # Check if the device is valid
    if args.device not in ["cpu", "cuda"]:
        raise ValueError("Invalid device. Choose 'cpu' or 'cuda'.")
    
    # Load the model and tokenizer
    # if model_name is provided, load the model from the model hub
    if args.model_name:
        model, tokenizer = load_model(args.model_name, args.cache_path, args.device)
        
        # Save the model and tokenizer to a checkpoint if dest_path is given
        if args.dest_path:
            # Check if the destination path exists
            if not os.path.exists(args.dest_path):
                # Create the destination path if it doesn't exist
                os.makedirs(args.dest_path, exist_ok=True)
                
            # Save the model and tokenizer to a checkpoint
            save_checkpoint(model, tokenizer, args.dest_path, args.shard_size)
        
    # if checkpoint_path is provided, load the model from the checkpoint
    elif args.checkpoint_path:
        model, tokenizer = load_checkpoint(args.checkpoint_path, args.cache_path, args.device)

        # Save the model and tokenizer to a checkpoint if dest_path is given
        if args.dest_path:
            # Check if the destination path exists
            if not os.path.exists(args.dest_path):
                # Create the destination path if it doesn't exist
                os.makedirs(args.dest_path, exist_ok=True)
                
            # Save the model and tokenizer to a checkpoint
            save_checkpoint(model, tokenizer, args.dest_path, args.shard_size)


if __name__ == "__main__":
	main()
