import { Check, Copy } from 'lucide-react';
import { useState } from 'react';

import { Button } from './ui/Button';
import { Label } from './ui/Label';
import { cn } from '../utils/cn';

interface ClientContext {
  agentSlug: string;
  bundleUrl: string | null;
  mcpUrl: string;
  apiKey: string;
}

interface ConfigBlock {
  hint: string;
  code: string;
}

interface ClientSpec {
  id: string;
  name: string;
  build: (ctx: ClientContext) => ConfigBlock[];
}

const KEY_PLACEHOLDER = '<your-api-key>';
const CODEX_ENV_VAR = 'DATACLAW_API_KEY';

const CLIENTS: readonly ClientSpec[] = [
  {
    id: 'as-skill',
    name: 'As a Skill',
    build: ({ bundleUrl }) => {
      return [
        {
          hint: bundleUrl ? 'Ask the agent to install the access point as a skill' : 'Generating install link…',
          code: bundleUrl ? `install dataclaw from ${bundleUrl}` : 'Loading…',
        },
      ];
    },
  },
  {
    id: 'openclaw',
    name: 'OpenClaw',
    build: ({ agentSlug, bundleUrl, mcpUrl, apiKey }) => {
      const payload = JSON.stringify({
        url: mcpUrl,
        headers: { Authorization: `Bearer ${apiKey}` },
      });
      return [
        {
          hint: bundleUrl ? 'Ask OpenClaw to install the access point as a skill' : 'Generating install link…',
          code: bundleUrl ? `install dataclaw from ${bundleUrl}` : 'Loading…',
        },
        {
          hint: 'Or register the MCP server directly',
          code: `openclaw mcp set ${agentSlug} '${payload}'`,
        },
      ];
    },
  },
  {
    id: 'claude-code',
    name: 'Claude Code',
    build: ({ agentSlug, mcpUrl, apiKey }) => [
      {
        hint: 'Run in your terminal',
        code: `claude mcp add --transport http ${agentSlug} ${mcpUrl} --header "Authorization: Bearer ${apiKey}"`,
      },
    ],
  },
  {
    id: 'codex',
    name: 'Codex',
    build: ({ agentSlug, mcpUrl, apiKey }) => [
      {
        hint: 'Export your key, then add the server',
        code:
          `export ${CODEX_ENV_VAR}=${apiKey}\n` +
          `codex mcp add ${agentSlug} --url ${mcpUrl} --bearer-token-env-var ${CODEX_ENV_VAR}`,
      },
    ],
  },
  {
    id: 'vscode',
    name: 'VS Code',
    build: ({ agentSlug, mcpUrl, apiKey }) => [
      {
        hint: 'Add to .vscode/mcp.json',
        code: JSON.stringify(
          {
            servers: {
              [agentSlug]: {
                type: 'http',
                url: mcpUrl,
                headers: { Authorization: `Bearer ${apiKey}` },
              },
            },
          },
          null,
          2,
        ),
      },
    ],
  },
  {
    id: 'cursor',
    name: 'Cursor',
    build: ({ agentSlug, mcpUrl, apiKey }) => [
      {
        hint: 'Add to ~/.cursor/mcp.json',
        code: JSON.stringify(
          {
            mcpServers: {
              [agentSlug]: {
                url: mcpUrl,
                headers: { Authorization: `Bearer ${apiKey}` },
              },
            },
          },
          null,
          2,
        ),
      },
    ],
  },
];

interface Props {
  agentSlug: string;
  bundleUrl: string | null;
  mcpUrl: string;
  apiKey: string | null;
}

const DEFAULT_CLIENT_ID = 'as-skill';
const SKILL_INSTALL_CLIENT_IDS = new Set(['as-skill']);

export function AgentConnectionClients({ agentSlug, bundleUrl, mcpUrl, apiKey }: Props): JSX.Element {
  const [selected, setSelected] = useState<string>(DEFAULT_CLIENT_ID);
  const client =
    CLIENTS.find((c) => c.id === selected) ??
    CLIENTS.find((c) => c.id === DEFAULT_CLIENT_ID);
  if (!client) return <></>;
  const needsInlineAPIKey = !SKILL_INSTALL_CLIENT_IDS.has(client.id);
  const blocks = client.build({
    agentSlug,
    bundleUrl,
    mcpUrl,
    apiKey: apiKey ?? KEY_PLACEHOLDER,
  });

  return (
    <div className="space-y-3">
      <Label>MCP server configuration</Label>
      <div
        role="radiogroup"
        aria-label="MCP client"
        className="flex flex-wrap gap-1 rounded-xl border border-border-light bg-surface-secondary/60 p-1"
      >
        {CLIENTS.map((c) => {
          const active = c.id === selected;
          return (
            <button
              key={c.id}
              type="button"
              role="radio"
              aria-checked={active}
              onClick={() => setSelected(c.id)}
              className={cn(
                'flex-1 min-w-max rounded-lg px-3 py-1.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-purple',
                active
                  ? 'bg-surface-submit text-white shadow-sm'
                  : 'text-text-secondary hover:bg-surface-hover hover:text-text-primary',
              )}
            >
              {c.name}
            </button>
          );
        })}
      </div>

      <div className="space-y-3 pt-1">
        {blocks.map((block, index) => (
          <ConfigBlockView key={index} block={block} />
        ))}
      </div>

      {!apiKey && needsInlineAPIKey && (
        <p className="text-xs text-text-tertiary">
          Reveal the API key above to inline it in the configuration.
        </p>
      )}
    </div>
  );
}

function ConfigBlockView({ block }: { block: ConfigBlock }): JSX.Element {
  const [copied, setCopied] = useState(false);

  const onCopy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(block.code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      // swallow — clipboard failures are non-fatal
    }
  };

  return (
    <div className="space-y-1.5">
      <p className="text-xs font-medium text-text-tertiary">{block.hint}</p>
      <div className="relative">
        <pre className="overflow-x-auto whitespace-pre rounded-xl border border-border-light bg-slate-950 p-4 pr-14 font-mono text-xs leading-relaxed text-slate-100">
          {block.code}
        </pre>
        <Button
          type="button"
          size="sm"
          variant="outline"
          className="absolute right-2 top-2"
          onClick={() => void onCopy()}
          aria-label="Copy to clipboard"
        >
          {copied ? <Check className="h-4 w-4 text-emerald-600" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
}
