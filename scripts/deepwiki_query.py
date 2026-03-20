import asyncio
import os
import sys
from contextlib import AsyncExitStack
from typing import Optional
import subprocess

import httpx
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from dotenv import load_dotenv

load_dotenv()

# The URL for the DeepWiki MCP server's Streamable HTTP transport
MCP_URL = "https://mcp.deepwiki.com/mcp"

class MCPClient:
    def __init__(self):
        self.session: Optional[ClientSession] = None
        self._streams_context = None
        self._session_context = None

    async def connect_to_server(self, server_url: str):
        self._streams_context = streamablehttp_client(url=server_url)
        streams = await self._streams_context.__aenter__()
        # streamablehttp_client yields (read, write, get_session_id)
        self._session_context = ClientSession(streams[0], streams[1])
        self.session = await self._session_context.__aenter__()
        await self.session.initialize()
        
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
        await client.connect_to_server(server_url=MCP_URL)
        
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
