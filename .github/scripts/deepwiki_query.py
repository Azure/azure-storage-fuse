import asyncio
import os
import sys
from contextlib import AsyncExitStack
from typing import Optional
import subprocess

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
        # print("Connecting to MCP SSE server...")
        self._streams_context = sse_client(url=server_url)
        streams = await self._streams_context.__aenter__()
        self._session_context = ClientSession(*streams)
        self.session: ClientSession = await self._session_context.__aenter__()
        await self.session.initialize()
        # print("Connected and initialized.")
        
    async def cleanup(self):
        if self._session_context:
            await self._session_context.__aexit__(None, None, None)
        if self._streams_context:
            await self._streams_context.__aexit__(None, None, None)

    async def ask_deepwiki(self, repo: str, question: str) -> str:
        if not self.session:
            raise RuntimeError("Client not connected.")

        # The MCP SDK handles the JSON-RPC call for you
        result = await self.session.call_tool(
            "ask_question", {"repoName": repo, "question": question}
        )
        
        # Join the list of content parts into a single string
        return result.content

async def main(repo, title, body):
    client = MCPClient()
    try:
        await client.connect_to_sse_server(server_url=MCP_SSE_URL)
        
        question = f"{title}\n\n{body}"        
        response = await client.ask_deepwiki(repo, question)
        print (response)
        
    finally:
        await client.cleanup()

if __name__ == "__main__":
    if len(sys.argv) < 4:
        print("Usage: python deepwiki_query.py <repo> <question title> <question body>")
        sys.exit(1)
    
    repo_arg = sys.argv[1]
    issue_title = sys.argv[2]
    issue_body = sys.argv[3]
    
    asyncio.run(main(repo_arg, issue_title, issue_body))
