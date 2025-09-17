import sys
import os
from transformers import pipeline, AutoTokenizer

# The maximum sequence length for this model. This is a fixed constraint.
MAX_SEQUENCE_LENGTH = 1024

def summarize_text_with_llm(text):
    """
    Summarizes text using a Hugging Face model from the `transformers` library.
    This function now handles large texts by splitting them into chunks and then
    performing a second summarization pass on the combined results.
    """
    try:
        # Load the summarization pipeline with the specified model.
        summarizer = pipeline("summarization", model="sshleifer/distilbart-cnn-12-6")
        
        # Load the tokenizer for the same model to correctly handle token limits.
        tokenizer = AutoTokenizer.from_pretrained("sshleifer/distilbart-cnn-12-6")

        # Get the number of tokens in the full text.
        token_count = len(tokenizer.encode(text, truncation=False))

        # If the text is within the model's limit, summarize it directly.
        if token_count <= MAX_SEQUENCE_LENGTH:
            summary = summarizer(text, max_length=150, min_length=50, do_sample=False)
            return summary[0]['summary_text']
        else:
            # If the text is too long, split it into chunks, summarize, and then refine.
            return refine_summary_from_chunks(text, summarizer, tokenizer)

    except Exception as e:
        print(f"Error summarizing with LLM: {e}", file=sys.stderr)
        # Fallback to a basic summarization or return an error message
        return "Could not generate AI summary. Full response below."

def refine_summary_from_chunks(text, summarizer, tokenizer):
    """
    Splits the input text into chunks, summarizes each one, and then performs a second
    summarization pass on the combined summaries to remove redundancy.
    """
    # Split the text into sentences to avoid cutting in the middle of one.
    sentences = text.split('. ')
    chunks = []
    current_chunk = ""

    for sentence in sentences:
        # Check if adding the next sentence exceeds the max token limit.
        temp_chunk = current_chunk + ('. ' if current_chunk else '') + sentence
        if len(tokenizer.encode(temp_chunk, truncation=False)) <= MAX_SEQUENCE_LENGTH:
            current_chunk = temp_chunk
        else:
            # The chunk is full, add it to the list and start a new one.
            chunks.append(current_chunk)
            current_chunk = sentence

    # Add the last chunk if it's not empty.
    if current_chunk:
        chunks.append(current_chunk)

    # First pass: summarize each chunk
    intermediate_summaries = []
    for i, chunk in enumerate(chunks):
        print(f"Summarizing chunk {i+1} of {len(chunks)}...", file=sys.stderr)
        summary = summarizer(chunk, max_length=150, min_length=50, do_sample=False)
        intermediate_summaries.append(summary[0]['summary_text'])
    
    # Second pass: combine and refine the summaries
    combined_summary_text = " ".join(intermediate_summaries)
    print("Refining combined summaries into a final output...", file=sys.stderr)
    
    # The refinement summary can be shorter to focus on key points.
    final_summary = summarizer(combined_summary_text, max_length=150, min_length=50, do_sample=False)
    
    return final_summary[0]['summary_text']


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python ai_summarize.py <input_file_path> <output_file_path>", file=sys.stderr)
        sys.exit(1)

    input_file_path = sys.argv[1]
    output_file_path = sys.argv[2]
    
    # Check if the input file exists
    if not os.path.exists(input_file_path):
        print(f"Error: Input file not found at {input_file_path}", file=sys.stderr)
        sys.exit(1)

    with open(input_file_path, 'r', encoding='utf-8') as f:
        full_text = f.read()
    
    summary = summarize_text_with_llm(full_text)
    
    # Add the disclaimer
    final_comment = (
        "### AI Generated Response\n\n"
        f"{summary}\n\n"
        "---\n"
        "**Kindly share mount command, config file and debug-logs for further investigation.**\n\n"
        "---\n"
        "*Disclaimer: This summary is AI-generated and may not be fully accurate.*"
    )
    
    with open(output_file_path, 'w', encoding='utf-8') as f:
        f.write(final_comment)
