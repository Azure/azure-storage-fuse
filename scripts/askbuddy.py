from openai import OpenAI
from azure.identity import DefaultAzureCredential, get_bearer_token_provider

token_provider = get_bearer_token_provider(
    DefaultAzureCredential(),
    "https://ai.azure.com/.default"
)

client = OpenAI(
    api_key=token_provider,
    base_url="https://blobfuse-buddy-resource.services.ai.azure.com/api/projects/blobfuse-buddy/applications/blobfuse-buddy/protocols/openai",
    default_query={"api-version": "2025-11-15-preview"}
)

response = client.responses.create(
    input="Test request: Should I use direct-io or disable-kernel-cache?"
)

print(response.output_text)