import { experimental_createMCPClient } from "@ai-sdk/mcp";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import type { ToolSet } from "ai";

type MCPClientLike = {
  tools: () => Promise<ToolSet>;
  close: () => Promise<void>;
};

let client: MCPClientLike | null = null;

export async function getTerraformTools(): Promise<ToolSet> {
  if (!client) {
    const url = process.env.TERRAFORM_MCP_URL;
    if (!url) throw new Error("TERRAFORM_MCP_URL is not set");
    const transport = new StreamableHTTPClientTransport(new URL(url));
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call
    client = (await experimental_createMCPClient({
      transport,
    })) as MCPClientLike;
  }
  const tools = await client.tools();
  return tools;
}
