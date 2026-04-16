import { Cable, Copy, Eye, EyeOff, RefreshCw } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import { PageHeader } from '../components/PageHeader';
import { StatusBanner } from '../components/StatusBanner';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { getOpenClaw, getStatus, rotateOpenClawKey } from '../services/api';
import type { RuntimeStatus } from '../types/datasource';
import type { OpenClawConfig } from '../types/openclaw';

function buildInstallCommand(runtime: RuntimeStatus | null, config: OpenClawConfig): string {
  if (config.installCommand) {
    return config.installCommand;
  }
  const endpoint = config.endpointUrl ?? `http://127.0.0.1:${runtime?.port ?? 18790}/mcp`;
  return `openclaw mcp set dataclaw '{"url":"${endpoint}","transport":"streamable-http","headers":{"Authorization":"Bearer \${DATACLAW_API_KEY}"}}'`;
}

export default function OpenClawPage(): JSX.Element {
  const [runtime, setRuntime] = useState<RuntimeStatus | null>(null);
  const [config, setConfig] = useState<OpenClawConfig | null>(null);
  const [revealKey, setRevealKey] = useState(false);
  const [feedback, setFeedback] = useState<{ tone: 'info' | 'success' | 'danger'; message: string } | null>(null);
  const [busy, setBusy] = useState<'loading' | 'rotating' | null>('loading');

  useEffect(() => {
    void (async () => {
      try {
        const nextRuntime = await getStatus();
        const nextConfig = await getOpenClaw(nextRuntime);
        setRuntime(nextRuntime);
        setConfig(nextConfig);
      } catch (error) {
        setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to load OpenClaw details.' });
      } finally {
        setBusy(null);
      }
    })();
  }, []);

  const installCommand = useMemo(() => buildInstallCommand(runtime, config ?? { apiKey: '' }), [runtime, config]);
  const displayedKey = revealKey ? (config?.apiKey ?? '') : (config?.maskedApiKey ?? '••••••••••••••••');
  const endpoint = config?.endpointUrl ?? `http://127.0.0.1:${runtime?.port ?? 18790}/mcp`;

  const copy = async (value: string, label: string): Promise<void> => {
    try {
      await navigator.clipboard.writeText(value);
      setFeedback({ tone: 'success', message: `${label} copied to clipboard.` });
    } catch {
      setFeedback({ tone: 'danger', message: `Failed to copy ${label.toLowerCase()}.` });
    }
  };

  const rotateKey = async (): Promise<void> => {
    setBusy('rotating');
    try {
      const nextConfig = await rotateOpenClawKey(runtime);
      setConfig(nextConfig);
      setRevealKey(true);
      setFeedback({ tone: 'success', message: 'OpenClaw API key rotated.' });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to rotate API key.' });
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="OpenClaw"
        description="DataClaw exposes one local MCP endpoint and one API key. Point OpenClaw at the streamable HTTP endpoint below and use the generated bearer token in your outbound MCP config."
      />
      {feedback ? <StatusBanner tone={feedback.tone} message={feedback.message} /> : null}
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-xl">
              <Cable className="h-5 w-5" />
              OpenClaw connection details
            </CardTitle>
            <CardDescription>
              Based on OpenClaw&apos;s remote MCP support: use the local DataClaw `/mcp` endpoint with `transport: streamable-http` and a bearer token.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
              <div className="text-sm font-medium text-text-primary">Local MCP endpoint</div>
              <div className="mt-2 flex flex-wrap items-center gap-3">
                <code className="rounded-lg bg-surface-primary px-3 py-2 text-sm text-text-primary">{endpoint}</code>
                <Button type="button" variant="outline" size="sm" onClick={() => void copy(endpoint, 'Endpoint URL')}>
                  <Copy className="h-4 w-4" />
                  Copy
                </Button>
              </div>
            </div>
            <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
              <div className="text-sm font-medium text-text-primary">Single API key</div>
              <div className="mt-2 flex flex-wrap items-center gap-3">
                <code className="rounded-lg bg-surface-primary px-3 py-2 text-sm text-text-primary">{displayedKey}</code>
                <Button type="button" variant="outline" size="sm" onClick={() => setRevealKey((current) => !current)}>
                  {revealKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  {revealKey ? 'Hide' : 'Reveal'}
                </Button>
                <Button type="button" variant="outline" size="sm" onClick={() => void copy(config?.apiKey ?? '', 'API key')}>
                  <Copy className="h-4 w-4" />
                  Copy
                </Button>
                <Button type="button" variant="outline" size="sm" onClick={() => void rotateKey()} disabled={busy !== null}>
                  <RefreshCw className="h-4 w-4" />
                  Rotate key
                </Button>
              </div>
            </div>
            <div className="rounded-2xl border border-border-light bg-surface-primary p-4">
              <h3 className="text-sm font-semibold text-text-primary">Suggested OpenClaw command</h3>
              <p className="mt-2 text-sm text-text-secondary">
                OpenClaw-managed outbound MCP definitions support remote servers via JSON. This command targets the local DataClaw streamable HTTP endpoint.
              </p>
              <pre className="mt-4 overflow-x-auto rounded-xl bg-slate-950 p-4 text-sm text-slate-100">
                <code>{installCommand}</code>
              </pre>
              <div className="mt-3 flex gap-3">
                <Button type="button" onClick={() => void copy(installCommand, 'OpenClaw command')}>
                  <Copy className="h-4 w-4" />
                  Copy command
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-xl">What OpenClaw can do through DataClaw</CardTitle>
            <CardDescription>
              DataClaw keeps the MCP surface intentionally small so setup stays fast and predictable.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-5 text-sm text-text-secondary">
            <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
              <div className="font-medium text-text-primary">MCP tools</div>
              <ul className="mt-3 space-y-2">
                <li><code>query</code> for raw read-only SQL against the configured datasource</li>
                <li><code>list_queries</code>, <code>create_query</code>, <code>update_query</code>, and <code>delete_query</code> for approved-query management</li>
              </ul>
            </div>
            <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
              <div className="font-medium text-text-primary">Recommended setup order</div>
              <ol className="mt-3 list-decimal space-y-2 pl-5">
                <li>Save and test one datasource on the Datasource page.</li>
                <li>Create at least one approved query, such as <code>SELECT true AS connected</code>.</li>
                <li>Run the <code>openclaw mcp set</code> command shown here.</li>
                <li>Ask OpenClaw to inspect or update approved queries through MCP tool calls.</li>
              </ol>
            </div>
            <div className="rounded-2xl border border-dashed border-border-light bg-surface-primary p-4">
              <div className="font-medium text-text-primary">Notes</div>
              <ul className="mt-3 list-disc space-y-2 pl-5">
                <li>There is no website auth in DataClaw v1; the API key is only for MCP access.</li>
                <li>DataClaw listens on port <code>{runtime?.port ?? 18790}</code> by default and may increment if that port is already occupied.</li>
                <li>The OpenClaw instructions here assume a local server installed on the same machine.</li>
              </ul>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
