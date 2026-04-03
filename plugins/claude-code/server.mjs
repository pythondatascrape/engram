import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  ListToolsRequestSchema,
  CallToolRequestSchema,
} from '@modelcontextprotocol/sdk/types.js';

import { DaemonClient } from './lib/daemon-client.mjs';

const TOOLS = [
  {
    name: 'derive_codebook',
    description: 'Derive a codebook from CLAUDE.md content. Extracts structured identity dimensions via pattern matching.',
    inputSchema: {
      type: 'object',
      properties: {
        content: { type: 'string', description: 'The CLAUDE.md text content to derive dimensions from.' },
        yamlOverridePath: { type: 'string', description: 'Optional path to .engram-codebook.yaml for manual overrides.' },
      },
      required: ['content'],
    },
  },
  {
    name: 'compress_identity',
    description: 'Compress identity dimensions into compact key=value format.',
    inputSchema: {
      type: 'object',
      properties: {
        dimensions: { type: 'object', description: 'Dimension key/value map to compress. If not provided, uses active codebook dimensions.' },
      },
    },
  },
  {
    name: 'check_redundancy',
    description: 'Check content for redundant identity reinforcement against the active codebook.',
    inputSchema: {
      type: 'object',
      properties: {
        content: { type: 'string', description: 'The content to check for redundant identity mentions.' },
      },
      required: ['content'],
    },
  },
  {
    name: 'get_stats',
    description: 'Get current session token accounting statistics.',
    inputSchema: { type: 'object', properties: {} },
  },
  {
    name: 'generate_report',
    description: 'Generate a markdown token savings report.',
    inputSchema: {
      type: 'object',
      properties: {
        name: { type: 'string', description: 'Report name / title.' },
        description: { type: 'string', description: 'Report description.' },
        savingsLogPath: { type: 'string', description: 'Path to a CSV savings log file.' },
        pricing: {
          type: 'object',
          description: 'Pricing info: { model: string, inputPer1k: number }.',
          properties: { model: { type: 'string' }, inputPer1k: { type: 'number' } },
        },
      },
    },
  },
];

const TOOL_TO_METHOD = {
  derive_codebook: 'engram.deriveCodebook',
  compress_identity: 'engram.compressIdentity',
  check_redundancy: 'engram.checkRedundancy',
  get_stats: 'engram.getStats',
  generate_report: 'engram.generateReport',
};

const daemon = new DaemonClient();

async function ensureConnected() {
  if (!daemon.connected) {
    try {
      await daemon.connect();
    } catch (err) {
      throw new Error(
        `Engram daemon not reachable at ~/.engram/engram.sock. ` +
        `Start it with: engram serve\n` +
        `Error: ${err.message}`
      );
    }
  }
}

const server = new Server(
  { name: 'engram-ccode', version: '0.2.0' },
  { capabilities: { tools: {} } },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({ tools: TOOLS }));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;
  const method = TOOL_TO_METHOD[name];

  if (!method) {
    return { content: [{ type: 'text', text: `Unknown tool: ${name}` }], isError: true };
  }

  try {
    await ensureConnected();
    const result = await daemon.call(method, args ?? {});
    return {
      content: [{
        type: 'text',
        text: typeof result === 'string' ? result : JSON.stringify(result, null, 2),
      }],
    };
  } catch (err) {
    return {
      content: [{ type: 'text', text: `Error in tool "${name}": ${err.message ?? String(err)}` }],
      isError: true,
    };
  }
});

const transport = new StdioServerTransport();
await server.connect(transport);
