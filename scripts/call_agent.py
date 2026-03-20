import json
import os
import re
import sys
import requests
from pathlib import Path
from openai import OpenAI
from azure.identity import DefaultAzureCredential, get_bearer_token_provider

# -----------------------------
# Load GitHub event payload
# -----------------------------
with open(os.environ["GITHUB_EVENT_PATH"], "r") as f:
    event = json.load(f)

is_issue = "issue" in event
is_discussion = "discussion" in event

if is_issue:
    title = event["issue"]["title"]
    body = event["issue"].get("body", "")
    comments_url = event["issue"]["comments_url"]
elif is_discussion:
    title = event["discussion"]["title"]
    body = event["discussion"].get("body", "")
    discussion_node_id = event["discussion"]["node_id"]
    comments_url = None  # discussions use GraphQL, not REST
else:
    dry_run = os.getenv("DRY_RUN", "false").lower() == "true"
    if not dry_run:
        raise RuntimeError("Unsupported GitHub event")
    title = os.getenv("MOCK_TITLE", "Dry-run workflow test")
    body = os.getenv("MOCK_BODY", "No issue/discussion payload found; using dry-run fallback.")
    repository = os.getenv("GITHUB_REPOSITORY", "owner/repo")
    comments_url = f"https://api.github.com/repos/{repository}/issues/1/comments"


# -----------------------------
# Query DeepWiki for context (via MCP SSE)
# -----------------------------
DEEPWIKI_REPO = os.getenv("DEEPWIKI_REPO")  # e.g. "Azure/azure-storage-fuse"

def query_deepwiki(question, repo_name):
    """Ask DeepWiki a question about the repo via MCP SSE and return its answer."""
    if not repo_name:
        print("DEEPWIKI_REPO not set, skipping DeepWiki query.")
        return ""
    try:
        import subprocess
        result = subprocess.run(
            [sys.executable, os.path.join(os.path.dirname(__file__), "deepwiki_query.py"),
             repo_name, question, ""],
            capture_output=True, text=True, timeout=120,
        )
        if result.returncode != 0:
            print(f"DeepWiki query failed (non-fatal): {result.stderr.strip()}")
            return ""
        answer = result.stdout.strip()
        print(f"DeepWiki returned {len(answer)} chars.")
        print("----- DeepWiki Answer -----")
        print(answer)
        print("----- End DeepWiki Answer -----")
        return answer
    except Exception as e:
        print(f"DeepWiki query failed (non-fatal): {e}")
        return ""


# -----------------------------
# Search local repo docs for context
# -----------------------------
DOC_FILES = [
    "TSG.md",
    "README.md",
    "MIGRATION.md",
    "KnownLimitations.txt",
    "CHANGELOG.md",
    "setup/baseConfig.yaml",
    "setup/advancedConfig.yaml",
    "setup/readme.md",
    "sampleFileCacheConfig.yaml",
    "sampleBlockCacheConfig.yaml",
    "sampleFileCacheWithSASConfig.yaml",
]
# Include all CLI docs from doc/
DOC_DIR = Path("doc")
if DOC_DIR.is_dir():
    DOC_FILES.extend(str(p) for p in DOC_DIR.glob("*.md"))

MAX_CONTEXT_CHARS = 12000  # cap to avoid overwhelming the prompt


def search_local_docs(question):
    """Search repo documentation for paragraphs relevant to the question."""
    # Extract keywords (3+ char words, lowercased, deduplicated)
    words = set(re.findall(r"[a-z][a-z0-9_-]{2,}", question.lower()))
    # Add common blobfuse terms that might appear as sub-strings
    extra = {"mount", "unmount", "cache", "fuse", "config", "auth", "sas",
             "spn", "msi", "key", "permission", "error", "fail", "timeout"}
    keywords = words | (extra & set(question.lower().split()))

    scored_chunks = []
    for filepath in DOC_FILES:
        path = Path(filepath)
        if not path.is_file():
            continue
        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue

        # Split into paragraphs (double-newline or markdown heading boundary)
        paragraphs = re.split(r"\n{2,}|\n(?=#)", text)
        for para in paragraphs:
            para = para.strip()
            if len(para) < 30:
                continue
            lower_para = para.lower()
            score = sum(1 for kw in keywords if kw in lower_para)
            if score > 0:
                scored_chunks.append((score, filepath, para))

    # Sort by relevance and assemble context within budget
    scored_chunks.sort(key=lambda x: x[0], reverse=True)
    context_parts = []
    total = 0
    for score, source, chunk in scored_chunks:
        if total + len(chunk) > MAX_CONTEXT_CHARS:
            break
        context_parts.append(f"[Source: {source}]\n{chunk}")
        total += len(chunk)

    result = "\n\n---\n\n".join(context_parts)
    print(f"Local doc search: {len(context_parts)} relevant snippets, {total} chars.")
    return result


# -----------------------------
# Gather context
# -----------------------------
user_question = f"{title}\n{body}"
deepwiki_context = query_deepwiki(user_question, DEEPWIKI_REPO)
local_doc_context = search_local_docs(user_question)

# -----------------------------
# Build prompt
# -----------------------------
context_block = ""
if deepwiki_context:
    context_block += f"""
--- DeepWiki Reference Answer ---
{deepwiki_context}
--- End DeepWiki Reference ---
"""
if local_doc_context:
    context_block += f"""
--- Relevant Documentation Snippets ---
{local_doc_context}
--- End Documentation ---
"""

prompt = f"""
New GitHub item received.

Title:
{title}

Description:
{body}
{context_block}
Using the reference information above as grounding context, please analyze the issue/question
and provide a helpful, accurate response. Prioritize factual accuracy from the documentation.
Where the documentation doesn't cover the topic, use your own knowledge but note it clearly.
"""

# -----------------------------
# Call Azure AI Foundry Agent
# -----------------------------
dry_run = os.getenv("DRY_RUN", "false").lower() == "true"
token_provider = get_bearer_token_provider(
    DefaultAzureCredential(),
    "https://ai.azure.com/.default"
)

client = OpenAI(
    api_key=token_provider,
    base_url=os.environ["FOUNDRY_BASE_URL"],
    default_query={"api-version": os.environ["FOUNDRY_API_VERSION"]}
)

response = client.responses.create(
    input=prompt,
    timeout=60
)

agent_reply = response.output_text.strip()

# -----------------------------
# Strip unwanted sections from agent reply
# -----------------------------
# Remove "Summary Table" and "Primary Sources" sections (and variants)
agent_reply = re.sub(
    r"(?:^|\n)#+\s*\*?\*?Summary\s+Table\*?\*?.*?(?=\n#+\s|\Z)",
    "", agent_reply, flags=re.DOTALL | re.IGNORECASE
)
agent_reply = re.sub(
    r"(?:^|\n)\*?\*?Summary\s+Table\*?\*?\s*\n\|.*?(?=\n#+\s|\n[^|\s]|\Z)",
    "", agent_reply, flags=re.DOTALL | re.IGNORECASE
)
agent_reply = re.sub(
    r"(?:^|\n)#+\s*\*?\*?(?:Primary\s+Sources?|References?)\*?\*?.*?(?=\n#+\s|\Z)",
    "", agent_reply, flags=re.DOTALL | re.IGNORECASE
)
agent_reply = re.sub(
    r"(?:^|\n)\*?\*?(?:Primary\s+Sources?|References?)[:\*]*\*?\*?\s*\n.*?(?=\n#+\s|\n\n[^-*\s]|\Z)",
    "", agent_reply, flags=re.DOTALL | re.IGNORECASE
)
agent_reply = agent_reply.strip()

# -----------------------------
# Prepend AI disclaimer
# -----------------------------
final_reply = f"""⚠️ **AI‑generated response**

This response was generated by an AI agent and may not be fully accurate.  
Please validate recommendations before acting on them.

---

{agent_reply}
"""

# -----------------------------
# Auto-summarize if too long
# -----------------------------
GITHUB_COMMENT_CHAR_LIMIT = 65536
SAFE_LIMIT = 60000          # leave headroom for markdown overhead
SUMMARIZE_TARGET = 55000    # target length for the summarized version

if len(final_reply) > SAFE_LIMIT:
    print(f"Response length ({len(final_reply)} chars) exceeds safe limit ({SAFE_LIMIT}). Requesting summarization...")
    summarize_prompt = (
        f"The following answer is too long to post as a single GitHub comment "
        f"(limit ~{GITHUB_COMMENT_CHAR_LIMIT} characters). "
        f"Please condense it to at most {SUMMARIZE_TARGET} characters while preserving "
        f"all key information, code examples, and actionable recommendations. "
        f"Keep the same markdown formatting.\n\n"
        f"--- ORIGINAL ANSWER ---\n{agent_reply}"
    )
    try:
        summary_resp = client.responses.create(
            input=summarize_prompt,
            timeout=90,
        )
        summarized = summary_resp.output_text.strip()
        final_reply = f"""⚠️ **AI‑generated response** *(summarized — original exceeded comment size limit)*

This response was generated by an AI agent and may not be fully accurate.  
Please validate recommendations before acting on them.

---

{summarized}
"""
        print(f"Summarized response to {len(final_reply)} chars.")
    except Exception as e:
        print(f"Summarization failed ({e}). Falling back to hard truncation.")

# Hard-truncate as a last-resort safety net
if len(final_reply) > GITHUB_COMMENT_CHAR_LIMIT:
    truncation_notice = "\n\n---\n⚠️ *Response was truncated because it exceeded the GitHub comment size limit.*"
    max_body = GITHUB_COMMENT_CHAR_LIMIT - len(truncation_notice)
    final_reply = final_reply[:max_body] + truncation_notice
    print(f"Hard-truncated response to {len(final_reply)} chars.")

if dry_run:
    question = f"Title: {title}\nDescription: {body}"
    print("DRY_RUN enabled. Skipping GitHub comment post.")
    print(f"Target URL: {comments_url}")
    print("----- Validation Q&A -----")
    print("Question Asked:")
    print(question)
    print()
    print("Agent Answer:")
    print(final_reply)
    print("----- End Validation Q&A -----")
    raise SystemExit(0)

# -----------------------------
# Post reply back to GitHub
# -----------------------------
headers = {
    "Authorization": f"Bearer {os.environ['GITHUB_TOKEN']}",
    "Accept": "application/vnd.github+json",
}

try:
    if is_discussion:
        # GitHub discussions require the GraphQL API for commenting
        graphql_url = "https://api.github.com/graphql"
        mutation = """
        mutation($discussionId: ID!, $body: String!) {
          addDiscussionComment(input: {discussionId: $discussionId, body: $body}) {
            comment { id url }
          }
        }
        """
        gql_resp = requests.post(
            graphql_url,
            headers=headers,
            json={"query": mutation, "variables": {"discussionId": discussion_node_id, "body": final_reply}},
        )
        gql_resp.raise_for_status()
        gql_data = gql_resp.json()
        if "errors" in gql_data:
            print(f"GraphQL errors: {gql_data['errors']}", file=sys.stderr)
            sys.exit(1)
        print(f"Discussion comment posted: {gql_data['data']['addDiscussionComment']['comment']['url']}")
    else:
        # Issues use the REST API
        post_resp = requests.post(
            comments_url,
            headers=headers,
            json={"body": final_reply},
        )
        post_resp.raise_for_status()
        print(f"Issue comment posted: {post_resp.json().get('html_url', comments_url)}")
except requests.exceptions.HTTPError as e:
    print(f"Failed to post comment: {e}", file=sys.stderr)
    print(f"Response body: {e.response.text}", file=sys.stderr)
    sys.exit(1)