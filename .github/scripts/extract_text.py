import re
import sys

def extract_text_field(text_content: str) -> str:
    """
    Extracts the 'text' field from a string representation of a TextContent object.
    
    Args:
        text_content: The input string containing the TextContent representation.
    
    Returns:
        The extracted text field as a string, or an empty string if not found.
    """
    # The new pattern uses a greedy match `(.*)` to capture all characters,
    # including newlines (re.DOTALL). The positive lookahead `(?=...)`
    # ensures that the match ends exactly before the specific trailing part
    # of the string (', annotations=None, meta=None)]'). This prevents
    # the regex from stopping at single quotes within the text content.
    try:
        pattern = ""
        if 'text="' in text_content:
            pattern = r"text=\"(.*)\"(?=, annotations=None, meta=None\)])"
        elif "text='" in text_content:
            pattern = r"text='(.*)'(?=, annotations=None, meta=None\)])"
        else:
            print("Error: The input text does not contain a recognizable text field.")
            sys.exit(1)

        match = re.search(pattern, text_content, re.DOTALL)
        if match:
            decoded_text = match.group(1).encode('utf-8').decode('unicode_escape')

            # Remove everything after either "## Notes\n" or "Notes:\n"
            for delimiter in ["## Notes\n", "Notes:\n"]:
                if delimiter in decoded_text:
                    decoded_text = decoded_text.split(delimiter)[0]
                    
            return decoded_text.strip() 
        else:
            print("No match found for the text field.")
            sys.exit(1)
            
    except re.error as e:
        print(f"Regular expression error: {e}")
    
    return ""

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python extract_text.py <path_to_input_file>")
        sys.exit(1)

    file_path = sys.argv[1]
    
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            input_text = f.read()
            
            input_len = len(input_text)
            if input_len < 100:   
                print("Input text is too short to process.")
                sys.exit(1)
                
            extracted_content = extract_text_field(input_text)
            if extracted_content:
                print(extracted_content)
            else:
                print("Error: Could not extract text content.")
                sys.exit(1)
                
    except FileNotFoundError:
        print(f"Error: The file '{file_path}' was not found.")
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
