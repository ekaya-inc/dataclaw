import { Bot, Copy, Eye, EyeOff, Pencil, Plus, RefreshCw, ShieldAlert, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useOutletContext } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { EmptyState } from '../components/EmptyState';
import { PageHeader } from '../components/PageHeader';
import { StatusBanner } from '../components/StatusBanner';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import {
  createAgent,
  deleteAgent,
  getStatus,
  listAgents,
  listQueries,
  revealAgentKey,
  rotateAgentKey,
  updateAgent,
} from '../services/api';
import type { AgentFormValues, AgentRecord, ApprovedQueryScope } from '../types/agent';
import type { RuntimeStatus } from '../types/datasource';
import type { SavedQuery } from '../types/query';
import { cn } from '../utils/cn';

const EMPTY_FORM: AgentFormValues = {
  name: '',
  canQuery: true,
  canExecute: false,
  approvedQueryScope: 'none',
  approvedQueryIds: [],
};

const SCOPE_OPTIONS: ReadonlyArray<{ value: ApprovedQueryScope; label: string; description: string }> = [
  { value: 'none', label: 'No approved queries', description: 'Hide approved-query tools for this agent.' },
  { value: 'all', label: 'All approved queries', description: 'Expose every current approved query.' },
  { value: 'selected', label: 'Selected approved queries', description: 'Only expose the queries checked below.' },
];

function endpointUrl(runtime: RuntimeStatus | null): string {
  return runtime?.mcpUrl ?? `http://127.0.0.1:${runtime?.port ?? 18790}/mcp`;
}

function buildMcpConfig(runtime: RuntimeStatus | null, agent: AgentRecord): string {
  return JSON.stringify(
    {
      mcpServers: {
        [agent.installAlias || 'dataclaw-agent']: {
          url: endpointUrl(runtime),
          transport: 'streamable-http',
          headers: {
            Authorization: 'Bearer ${DATACLAW_API_KEY}',
          },
        },
      },
    },
    null,
    2,
  );
}

function summarizeScope(agent: AgentRecord): string {
  switch (agent.approvedQueryScope) {
    case 'all':
      return 'All approved queries';
    case 'selected':
      return `${agent.approvedQueryIds.length} selected approved quer${agent.approvedQueryIds.length === 1 ? 'y' : 'ies'}`;
    default:
      return 'No approved queries';
  }
}

function formatTimestamp(value?: string): string {
  if (!value) return 'Never';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

export default function AgentsPage(): JSX.Element {
  const { refresh } = useOutletContext<AppOutletContext>();
  const [runtime, setRuntime] = useState<RuntimeStatus | null>(null);
  const [agents, setAgents] = useState<AgentRecord[]>([]);
  const [queries, setQueries] = useState<SavedQuery[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [editingAgentId, setEditingAgentId] = useState<string | null>(null);
  const [formValues, setFormValues] = useState<AgentFormValues>(EMPTY_FORM);
  const [revealedKeys, setRevealedKeys] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<string | null>('loading');
  const [feedback, setFeedback] = useState<{ tone: 'info' | 'success' | 'danger'; message: string } | null>(null);

  const load = useCallback(async (): Promise<void> => {
    setBusy((current) => current ?? 'loading');
    try {
      const [nextRuntime, nextAgents, nextQueries] = await Promise.all([getStatus(), listAgents(), listQueries().catch(() => [])]);
      setRuntime(nextRuntime);
      setAgents(nextAgents);
      setQueries(nextQueries);
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to load agent details.' });
    } finally {
      setBusy(null);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (agents.length === 0) {
      setSelectedAgentId(null);
      if (!editingAgentId) {
        setFormValues(EMPTY_FORM);
      }
      return;
    }
    if (!selectedAgentId || !agents.some((agent) => agent.id === selectedAgentId)) {
      setSelectedAgentId(agents[0]?.id ?? null);
    }
  }, [agents, editingAgentId, selectedAgentId]);

  const selectedAgent = useMemo(
    () => agents.find((agent) => agent.id === selectedAgentId) ?? agents[0] ?? null,
    [agents, selectedAgentId],
  );

  const selectedKey = selectedAgent ? revealedKeys[selectedAgent.id] : undefined;
  const selectedConfig = selectedAgent ? buildMcpConfig(runtime, selectedAgent) : '';

  const resetForm = (): void => {
    setEditingAgentId(null);
    setFormValues(EMPTY_FORM);
  };

  const copy = async (value: string, label: string): Promise<void> => {
    try {
      await navigator.clipboard.writeText(value);
      setFeedback({ tone: 'success', message: `${label} copied to clipboard.` });
    } catch {
      setFeedback({ tone: 'danger', message: `Failed to copy ${label.toLowerCase()}.` });
    }
  };

  const setScope = (scope: ApprovedQueryScope): void => {
    setFormValues((current) => ({
      ...current,
      approvedQueryScope: scope,
      approvedQueryIds: scope === 'selected' ? current.approvedQueryIds : [],
    }));
  };

  const toggleApprovedQuery = (queryID: string): void => {
    setFormValues((current) => {
      const hasQuery = current.approvedQueryIds.includes(queryID);
      return {
        ...current,
        approvedQueryIds: hasQuery ? current.approvedQueryIds.filter((id) => id !== queryID) : [...current.approvedQueryIds, queryID],
      };
    });
  };

  const beginEdit = (agent: AgentRecord): void => {
    setEditingAgentId(agent.id);
    setSelectedAgentId(agent.id);
    setFormValues({
      name: agent.name,
      canQuery: agent.canQuery,
      canExecute: agent.canExecute,
      approvedQueryScope: agent.approvedQueryScope,
      approvedQueryIds: [...agent.approvedQueryIds],
    });
    setFeedback(null);
  };

  const saveAgent = async (): Promise<void> => {
    setBusy(editingAgentId ? 'saving' : 'creating');
    setFeedback(null);
    try {
      const saved = editingAgentId ? await updateAgent(editingAgentId, formValues) : await createAgent(formValues);
      if (saved.apiKey) {
        setRevealedKeys((current) => ({ ...current, [saved.id]: saved.apiKey ?? '' }));
      }
      resetForm();
      setSelectedAgentId(saved.id);
      await Promise.all([load(), refresh()]);
      setFeedback({ tone: 'success', message: editingAgentId ? 'Agent updated.' : 'Agent created. Copy the key now if you need it.' });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to save agent.' });
    } finally {
      setBusy(null);
    }
  };

  const toggleReveal = async (agent: AgentRecord): Promise<void> => {
    if (revealedKeys[agent.id]) {
      setRevealedKeys((current) => {
        const next = { ...current };
        delete next[agent.id];
        return next;
      });
      return;
    }
    setBusy(`reveal:${agent.id}`);
    try {
      const revealed = await revealAgentKey(agent.id);
      if (revealed.apiKey) {
        setRevealedKeys((current) => ({ ...current, [agent.id]: revealed.apiKey ?? '' }));
      }
      setFeedback({ tone: 'success', message: `${agent.name} key revealed.` });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to reveal agent key.' });
    } finally {
      setBusy(null);
    }
  };

  const rotateKey = async (agent: AgentRecord): Promise<void> => {
    setBusy(`rotate:${agent.id}`);
    try {
      const rotated = await rotateAgentKey(agent.id);
      setRevealedKeys((current) => ({ ...current, [agent.id]: rotated.apiKey ?? '' }));
      await Promise.all([load(), refresh()]);
      setFeedback({ tone: 'success', message: `${agent.name} key rotated.` });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to rotate agent key.' });
    } finally {
      setBusy(null);
    }
  };

  const removeAgent = async (agent: AgentRecord): Promise<void> => {
    if (!window.confirm(`Delete ${agent.name}? This cannot be undone.`)) return;
    setBusy(`delete:${agent.id}`);
    try {
      await deleteAgent(agent.id);
      setRevealedKeys((current) => {
        const next = { ...current };
        delete next[agent.id];
        return next;
      });
      if (editingAgentId === agent.id) {
        resetForm();
      }
      if (selectedAgentId === agent.id) {
        setSelectedAgentId(null);
      }
      await Promise.all([load(), refresh()]);
      setFeedback({ tone: 'success', message: `${agent.name} deleted.` });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to delete agent.' });
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Agents"
        description="Create named MCP agents with explicit raw-tool permissions, scoped approved-query access, and per-agent API keys."
        actions={
          editingAgentId ? (
            <Button type="button" variant="outline" onClick={resetForm}>
              Cancel edit
            </Button>
          ) : (
            <Button type="button" variant="outline" onClick={resetForm}>
              <Plus className="h-4 w-4" />
              New agent
            </Button>
          )
        }
      />
      {feedback ? <StatusBanner tone={feedback.tone} message={feedback.message} /> : null}
      {!runtime?.datasourceConfigured ? (
        <StatusBanner
          tone="info"
          message="Connect a datasource before any raw query, raw execute, or approved-query MCP tools become available to agents."
        />
      ) : null}
      <div className="grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
        <Card>
          <CardHeader>
            <CardTitle>{editingAgentId ? 'Edit agent' : 'Create an agent'}</CardTitle>
            <CardDescription>
              Install aliases are generated once, remain stable, and are used as the client-facing MCP entry name.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="space-y-2">
              <Label htmlFor="agent-name">Display name</Label>
              <Input
                id="agent-name"
                value={formValues.name}
                onChange={(event) => setFormValues((current) => ({ ...current, name: event.target.value }))}
                placeholder="Finance analyst"
              />
            </div>

            <div className="space-y-3">
              <Label>Raw tools</Label>
              <label className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-secondary p-4 text-sm text-text-secondary">
                <input
                  type="checkbox"
                  className="mt-1 h-4 w-4 rounded border-border-medium"
                  checked={formValues.canQuery}
                  onChange={(event) => setFormValues((current) => ({ ...current, canQuery: event.target.checked }))}
                />
                <div>
                  <div className="font-medium text-text-primary">Allow raw query</div>
                  <p className="mt-1">Expose the <code>query</code> tool for ad-hoc read-only SQL.</p>
                </div>
              </label>
              <label className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-secondary p-4 text-sm text-text-secondary">
                <input
                  type="checkbox"
                  className="mt-1 h-4 w-4 rounded border-border-medium"
                  checked={formValues.canExecute}
                  onChange={(event) => setFormValues((current) => ({ ...current, canExecute: event.target.checked }))}
                />
                <div>
                  <div className="font-medium text-text-primary">Allow raw execute</div>
                  <p className="mt-1">Expose the dangerous <code>execute</code> tool for ad-hoc DDL/DML.</p>
                </div>
              </label>
              {formValues.canExecute ? (
                <StatusBanner
                  tone="danger"
                  message="Raw execute gives this agent direct write/DDL power against the datasource. Only enable it for keys you would trust with production-impacting SQL."
                />
              ) : null}
            </div>

            <div className="space-y-3">
              <Label>Approved query access</Label>
              <div className="grid gap-3 md:grid-cols-3">
                {SCOPE_OPTIONS.map((option) => {
                  const disabled = option.value === 'selected' && queries.length === 0;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      className={cn(
                        'rounded-xl border p-4 text-left transition-colors',
                        formValues.approvedQueryScope === option.value
                          ? 'border-slate-950 bg-slate-950 text-white'
                          : 'border-border-light bg-surface-secondary text-text-primary hover:bg-surface-hover',
                        disabled ? 'cursor-not-allowed opacity-50' : '',
                      )}
                      disabled={disabled}
                      onClick={() => setScope(option.value)}
                    >
                      <div className="font-medium">{option.label}</div>
                      <div className={cn('mt-2 text-sm', formValues.approvedQueryScope === option.value ? 'text-slate-200' : 'text-text-secondary')}>
                        {option.description}
                      </div>
                    </button>
                  );
                })}
              </div>
              {formValues.approvedQueryScope === 'selected' ? (
                <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <div>
                      <div className="text-sm font-medium text-text-primary">Select approved queries</div>
                      <p className="text-sm text-text-secondary">Only these approved queries will be visible through MCP.</p>
                    </div>
                    <div className="text-xs text-text-tertiary">{formValues.approvedQueryIds.length} selected</div>
                  </div>
                  <div className="max-h-64 space-y-2 overflow-y-auto">
                    {queries.map((query) => (
                      <label key={query.id} className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-primary p-3 text-sm text-text-secondary">
                        <input
                          type="checkbox"
                          className="mt-1 h-4 w-4 rounded border-border-medium"
                          checked={formValues.approvedQueryIds.includes(query.id)}
                          onChange={() => toggleApprovedQuery(query.id)}
                        />
                        <div className="min-w-0 flex-1">
                          <div className="truncate font-medium text-text-primary" title={query.naturalLanguagePrompt}>
                            {query.naturalLanguagePrompt}
                          </div>
                          <div className="truncate text-xs text-text-tertiary" title={query.sql}>
                            {query.sql}
                          </div>
                        </div>
                      </label>
                    ))}
                  </div>
                </div>
              ) : null}
            </div>

            <div className="flex flex-wrap gap-3">
              <Button type="button" onClick={() => void saveAgent()} disabled={busy !== null}>
                {editingAgentId ? 'Save changes' : 'Create agent'}
              </Button>
              {editingAgentId ? (
                <Button type="button" variant="outline" onClick={resetForm} disabled={busy !== null}>
                  Cancel
                </Button>
              ) : null}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Bot className="h-5 w-5" />
              Agent inventory
            </CardTitle>
            <CardDescription>
              Each agent keeps its own immutable install alias, API key, and MCP tool scope.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {agents.length === 0 ? (
              <EmptyState
                title="No agents yet"
                body="Create your first agent to mint a dedicated API key and start composing MCP client configs."
              />
            ) : (
              <div className="space-y-3">
                {agents.map((agent) => {
                  const revealed = Boolean(revealedKeys[agent.id]);
                  return (
                    <div
                      key={agent.id}
                      className={cn(
                        'rounded-2xl border p-4 transition-colors',
                        selectedAgentId === agent.id ? 'border-slate-950 bg-slate-950 text-white' : 'border-border-light bg-surface-secondary',
                      )}
                    >
                      <div className="flex flex-wrap items-start justify-between gap-4">
                        <div className="space-y-2">
                          <button type="button" className="text-left" onClick={() => setSelectedAgentId(agent.id)}>
                            <div className="text-base font-semibold">{agent.name}</div>
                            <div className={cn('text-sm', selectedAgentId === agent.id ? 'text-slate-300' : 'text-text-secondary')}>
                              {agent.installAlias}
                            </div>
                          </button>
                          <div className="flex flex-wrap gap-2 text-xs">
                            {agent.canQuery ? <span className="rounded-full bg-emerald-500/15 px-2 py-1 font-medium text-emerald-600">Raw query</span> : null}
                            {agent.canExecute ? <span className="rounded-full bg-red-500/15 px-2 py-1 font-medium text-red-600">Raw execute</span> : null}
                            <span className="rounded-full bg-surface-primary px-2 py-1 font-medium text-text-secondary">{summarizeScope(agent)}</span>
                          </div>
                          <div className={cn('text-sm', selectedAgentId === agent.id ? 'text-slate-300' : 'text-text-secondary')}>
                            Last used: {formatTimestamp(agent.lastUsedAt)}
                          </div>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <Button type="button" size="sm" variant="outline" onClick={() => beginEdit(agent)}>
                            <Pencil className="h-4 w-4" />
                            Edit
                          </Button>
                          <Button type="button" size="sm" variant="outline" onClick={() => void toggleReveal(agent)} disabled={busy === `reveal:${agent.id}`}>
                            {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                            {revealed ? 'Hide key' : 'Reveal key'}
                          </Button>
                          <Button type="button" size="sm" variant="outline" onClick={() => void rotateKey(agent)} disabled={busy === `rotate:${agent.id}`}>
                            <RefreshCw className="h-4 w-4" />
                            Rotate
                          </Button>
                          <Button type="button" size="sm" variant="destructive" onClick={() => void removeAgent(agent)} disabled={busy === `delete:${agent.id}`}>
                            <Trash2 className="h-4 w-4" />
                            Delete
                          </Button>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {selectedAgent ? (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
          <Card>
            <CardHeader>
              <CardTitle>{selectedAgent.name} connection details</CardTitle>
              <CardDescription>
                Use the immutable install alias below when you configure a remote MCP client.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
                <div className="text-sm font-medium text-text-primary">Install alias</div>
                <div className="mt-2 flex flex-wrap items-center gap-3">
                  <code className="rounded-lg bg-surface-primary px-3 py-2 text-sm text-text-primary">{selectedAgent.installAlias}</code>
                  <Button type="button" variant="outline" size="sm" onClick={() => void copy(selectedAgent.installAlias, 'Install alias')}>
                    <Copy className="h-4 w-4" />
                    Copy
                  </Button>
                </div>
              </div>

              <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
                <div className="text-sm font-medium text-text-primary">Endpoint URL</div>
                <div className="mt-2 flex flex-wrap items-center gap-3">
                  <code className="rounded-lg bg-surface-primary px-3 py-2 text-sm text-text-primary">{endpointUrl(runtime)}</code>
                  <Button type="button" variant="outline" size="sm" onClick={() => void copy(endpointUrl(runtime), 'Endpoint URL')}>
                    <Copy className="h-4 w-4" />
                    Copy
                  </Button>
                </div>
              </div>

              <div className="rounded-2xl border border-border-light bg-surface-secondary p-4">
                <div className="text-sm font-medium text-text-primary">Agent API key</div>
                <div className="mt-2 flex flex-wrap items-center gap-3">
                  <code className="rounded-lg bg-surface-primary px-3 py-2 text-sm text-text-primary">
                    {selectedKey ?? selectedAgent.maskedApiKey ?? '••••••••••••••••'}
                  </code>
                  <Button type="button" variant="outline" size="sm" onClick={() => void toggleReveal(selectedAgent)}>
                    {selectedKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    {selectedKey ? 'Hide' : 'Reveal'}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => void copy(selectedKey ?? '', 'Agent API key')}
                    disabled={!selectedKey}
                  >
                    <Copy className="h-4 w-4" />
                    Copy key
                  </Button>
                  <Button type="button" variant="outline" size="sm" onClick={() => void rotateKey(selectedAgent)} disabled={busy === `rotate:${selectedAgent.id}`}>
                    <RefreshCw className="h-4 w-4" />
                    Rotate key
                  </Button>
                </div>
                <p className="mt-3 text-sm text-text-secondary">
                  Normal page loads stay masked. Plaintext is only returned immediately after create, explicit reveal, or key rotation.
                </p>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <ShieldAlert className="h-5 w-5" />
                Generic MCP config snippet
              </CardTitle>
              <CardDescription>
                Paste this into a compatible remote MCP client config, then set <code>DATACLAW_API_KEY</code> to the revealed key above.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <pre className="overflow-x-auto rounded-2xl bg-slate-950 p-4 text-sm text-slate-100">
                <code>{selectedConfig}</code>
              </pre>
              <div className="flex flex-wrap gap-3">
                <Button type="button" onClick={() => void copy(selectedConfig, 'MCP config')}>
                  <Copy className="h-4 w-4" />
                  Copy config
                </Button>
              </div>
              <div className="rounded-2xl border border-dashed border-border-light bg-surface-secondary p-4 text-sm text-text-secondary">
                <ul className="list-disc space-y-2 pl-5">
                  <li>The alias <code>{selectedAgent.installAlias}</code> stays stable across ordinary edits.</li>
                  <li><code>query</code> is {selectedAgent.canQuery ? 'enabled' : 'disabled'} and raw <code>execute</code> is {selectedAgent.canExecute ? 'enabled' : 'disabled'} for this agent.</li>
                  <li>{summarizeScope(selectedAgent)} is currently configured for approved-query access.</li>
                </ul>
              </div>
            </CardContent>
          </Card>
        </div>
      ) : null}
    </div>
  );
}
