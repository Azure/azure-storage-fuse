import asyncio
import os
import sys
from contextlib import AsyncExitStack
from typing import Optional

import httpx
from mcp import ClientSession
from mcp.client.sse import sse_client
from dotenv import load_dotenv

load_dotenv()

# The URL for the DeepWiki MCP server's SSE transport
MCP_SSE_URL = "https://mcp.deepwiki.com/sse" # Example URL

class MCPClient:
    def __init__(self):
        self.session: Optional[ClientSession] = None
        self.exit_stack = AsyncExitStack()

    async def connect_to_sse_server(self, server_url: str):
        print("Connecting to MCP SSE server...")
        self._streams_context = sse_client(url=server_url)
        streams = await self._streams_context.__aenter__()
        self._session_context = ClientSession(*streams)
        self.session: ClientSession = await self._session_context.__aenter__()
        await self.session.initialize()
        print("Connected and initialized.")
        
    async def cleanup(self):
        if self._session_context:
            await self._session_context.__aexit__(None, None, None)
        if self._streams_context:
            await self._streams_context.__aexit__(None, None, None)

    async def ask_deepwiki(self, repo: str, question: str) -> str:
        if not self.session:
            raise RuntimeError("Client not connected.")

        print(f"Calling 'ask_question' for repo: {repo} with question: {question}")
        
        # The MCP SDK handles the JSON-RPC call for you
        result = await self.session.call_tool(
            "ask_question", {"repoName": repo, "question": question}
        )
        return result.content

async def main(repo, question):
    client = MCPClient()
    try:
        await client.connect_to_sse_server(server_url=MCP_SSE_URL)
        response_content = await client.ask_deepwiki(repo, question)
        print("\n" + "="*50)
        print("DeepWiki Response:")
        print("="*50)
        print(response_content)

    finally:
        await client.cleanup()

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python ask_question.py <repo> <question>")
        sys.exit(1)
    
    repo_arg = sys.argv[1]
    question_arg = sys.argv[2]
    
    asyncio.run(main(repo_arg, question_arg))
