import sys
import os
from transformers import pipeline

def summarize_text_with_llm(text, num_sentences=5):
    """
    Summarizes text using a Hugging Face model from the `transformers` library.
    """
    try:
        # Load a summarization pipeline with a pre-trained model.
        # This will download the model the first time it is run.
        summarizer = pipeline("summarization", model="sshleifer/distilbart-cnn-12-6")

        # The summarizer pipeline takes a single string and returns a list of dictionaries.
        # We extract the 'summary_text' from the first dictionary.
        summary = summarizer(text, max_length=150, min_length=50, do_sample=False)
        return summary[0]['summary_text']

    except Exception as e:
        print(f"Error summarizing with LLM: {e}", file=sys.stderr)
        # Fallback to a basic summarization or return an error message
        return "Could not generate AI summary. Full response below."

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
        "*Disclaimer: This summary is AI-generated and may not be fully accurate.*"
    )
    
    with open(output_file_path, 'w', encoding='utf-8') as f:
        f.write(final_comment)
