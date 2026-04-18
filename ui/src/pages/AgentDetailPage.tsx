import { ArrowLeft, Bot, Check, Copy, Eye, EyeOff, Pencil, RefreshCw, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { useLocation, useNavigate, useOutletContext, useParams } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { PageHeader } from '../components/PageHeader';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/Dialog';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { useToast } from '../components/ui/Toast';
import { toMCPKey } from '../lib/mcpSlug';
import { deleteAgent, getAgent, getStatus, revealAgentKey, rotateAgentKey } from '../services/api';
import type { AgentRecord } from '../types/agent';
import type { RuntimeStatus } from '../types/datasource';
import { cn } from '../utils/cn';

const DELETE_CONFIRM_TEXT = 'delete access point';

function endpointUrl(runtime: RuntimeStatus | null): string {
  return runtime?.mcpUrl ?? `http://127.0.0.1:${runtime?.port ?? 18790}/mcp`;
}

function buildMcpConfig(runtime: RuntimeStatus | null, agent: AgentRecord, apiKey: string | undefined): string {
  return JSON.stringify(
    {
      mcpServers: {
        [toMCPKey(agent.name)]: {
          type: 'http',
          url: endpointUrl(runtime),
          headers: {
            Authorization: `Bearer ${apiKey ?? '<your-api-key>'}`,
          },
        },
      },
    },
    null,
    2,
  );
}

function formatDate(value?: string): string {
  if (!value) return '—';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleDateString();
}

function formatTimestamp(value?: string): string {
  if (!value) return '—';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

function scopeSummary(agent: AgentRecord): string {
  switch (agent.approvedQueryScope) {
    case 'all':
      return 'all queries';
    case 'selected':
      return `${agent.approvedQueryIds.length} ${agent.approvedQueryIds.length === 1 ? 'query' : 'queries'}`;
    default:
      return 'no approved queries';
  }
}

function ToolsPills({ agent }: { agent: AgentRecord }): JSX.Element {
  return (
    <div className="flex flex-wrap items-center gap-1.5 text-xs font-medium">
      {agent.canQuery ? (
        <span className="rounded-full bg-emerald-500/15 px-2 py-0.5 text-emerald-700">query</span>
      ) : null}
      {agent.canExecute ? (
        <span className="rounded-full bg-red-500/15 px-2 py-0.5 text-red-700">execute</span>
      ) : null}
      {agent.canManageApprovedQueries ? (
        <span className="rounded-full bg-indigo-500/15 px-2 py-0.5 text-indigo-700">Manage Approved Queries</span>
      ) : null}
      <span
        className={cn(
          'rounded-full px-2 py-0.5',
          agent.approvedQueryScope === 'none'
            ? 'bg-surface-secondary text-text-tertiary'
            : 'bg-slate-500/15 text-slate-700',
        )}
      >
        {scopeSummary(agent)}
      </span>
    </div>
  );
}

interface DetailLocationState {
  apiKey?: string | null;
}

export default function AgentDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const { refresh } = useOutletContext<AppOutletContext>();
  const { toast } = useToast();

  const [agent, setAgent] = useState<AgentRecord | null>(null);
  const [runtime, setRuntime] = useState<RuntimeStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [revealedKey, setRevealedKey] = useState<string | null>(null);
  const [revealBusy, setRevealBusy] = useState(false);
  const [rotateBusy, setRotateBusy] = useState(false);
  const [copyFlash, setCopyFlash] = useState<string | null>(null);

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState('');
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    const state = location.state as DetailLocationState | null;
    if (state?.apiKey) {
      setRevealedKey(state.apiKey);
      navigate(location.pathname, { replace: true, state: null });
    }
  }, [location.pathname, location.state, navigate]);

  const load = useCallback(async (): Promise<void> => {
    if (!id) return;
    try {
      const [loadedAgent, loadedRuntime] = await Promise.all([getAgent(id), getStatus()]);
      setAgent(loadedAgent);
      setRuntime(loadedRuntime);
    } catch (error) {
      toast({
        title: 'Failed to load',
        description: error instanceof Error ? error.message : 'Failed to load access point.',
        variant: 'error',
      });
      navigate('/agents');
    } finally {
      setLoading(false);
    }
  }, [id, navigate, toast]);

  useEffect(() => {
    void load();
  }, [load]);

  const copy = async (value: string, label: string): Promise<void> => {
    try {
      await navigator.clipboard.writeText(value);
      setCopyFlash(label);
      window.setTimeout(() => setCopyFlash((current) => (current === label ? null : current)), 1800);
    } catch {
      toast({
        title: 'Copy failed',
        description: `Could not copy ${label.toLowerCase()}.`,
        variant: 'error',
      });
    }
  };

  const reveal = async (): Promise<void> => {
    if (!agent) return;
    setRevealBusy(true);
    try {
      const revealed = await revealAgentKey(agent.id);
      if (revealed.apiKey) setRevealedKey(revealed.apiKey);
    } catch (error) {
      toast({
        title: 'Reveal failed',
        description: error instanceof Error ? error.message : 'Failed to reveal key.',
        variant: 'error',
      });
    } finally {
      setRevealBusy(false);
    }
  };

  const rotate = async (): Promise<void> => {
    if (!agent) return;
    setRotateBusy(true);
    try {
      const rotated = await rotateAgentKey(agent.id);
      if (rotated.apiKey) setRevealedKey(rotated.apiKey);
      toast({ title: 'Key rotated', description: `${rotated.name} now has a new API key.`, variant: 'success' });
      await load();
    } catch (error) {
      toast({
        title: 'Rotate failed',
        description: error instanceof Error ? error.message : 'Failed to rotate key.',
        variant: 'error',
      });
    } finally {
      setRotateBusy(false);
    }
  };

  const openDelete = (): void => {
    setDeleteConfirm('');
    setDeleteOpen(true);
  };

  const closeDelete = (): void => {
    setDeleteOpen(false);
    setDeleteConfirm('');
  };

  const handleDelete = async (): Promise<void> => {
    if (!agent || deleteConfirm !== DELETE_CONFIRM_TEXT) return;
    setDeleting(true);
    try {
      await deleteAgent(agent.id);
      await refresh();
      toast({ title: 'Access point deleted', description: `${agent.name} removed.`, variant: 'success' });
      navigate('/agents');
    } catch (error) {
      toast({
        title: 'Delete failed',
        description: error instanceof Error ? error.message : 'Failed to delete access point.',
        variant: 'error',
      });
      setDeleting(false);
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div>
          <Button variant="ghost" size="sm" onClick={() => navigate('/agents')}>
            <ArrowLeft className="h-4 w-4" />
            Back to Agent Access
          </Button>
        </div>
        <div className="rounded-xl border border-dashed border-border-light bg-surface-secondary/60 px-4 py-6 text-sm text-text-secondary">
          Loading access point…
        </div>
      </div>
    );
  }

  if (!agent) return <></>;

  const displayKey = revealedKey ?? (agent.maskedApiKey || '••••••••••••••••');
  const mcpConfig = buildMcpConfig(runtime, agent, revealedKey ?? undefined);
  const keyLabel = copyFlash === 'API key' ? 'Copied' : 'Copy';
  const configLabel = copyFlash === 'MCP config' ? 'Copied' : 'Copy';

  return (
    <div className="space-y-6">
      <div>
        <Button variant="ghost" size="sm" onClick={() => navigate('/agents')}>
          <ArrowLeft className="h-4 w-4" />
          Back to Agent Access
        </Button>
      </div>

      <PageHeader
        title={agent.name}
        description="Connection details and API key for this access point."
        actions={
          <>
            <Button
              type="button"
              variant="outline"
              onClick={openDelete}
              className="border-red-200 text-red-600 hover:bg-red-50 hover:text-red-700"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </Button>
            <Button type="button" variant="outline" onClick={() => navigate(`/agents/${agent.id}/edit`)}>
              <Pencil className="h-4 w-4" />
              Edit
            </Button>
          </>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-xl">
            <Bot className="h-5 w-5 text-text-secondary" />
            Overview
          </CardTitle>
          <CardDescription>
            Created {formatDate(agent.createdAt)} · Last used {formatTimestamp(agent.lastUsedAt)}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-1.5">
            <Label>Name</Label>
            <Input value={agent.name} readOnly />
          </div>

          <div className="space-y-1.5">
            <Label>Permissions</Label>
            <div className="rounded-xl border border-border-light bg-surface-secondary/60 px-3 py-2.5">
              <ToolsPills agent={agent} />
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">Connection</CardTitle>
          <CardDescription>
            Distribute keys carefully and rotate them periodically. Reveal fetches the plaintext on demand.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-1.5">
            <Label>API key</Label>
            <div className="flex items-center gap-2">
              <Input value={displayKey} readOnly className="font-mono" />
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={revealedKey ? () => setRevealedKey(null) : () => void reveal()}
                disabled={revealBusy}
                title={revealedKey ? 'Hide key' : 'Reveal key'}
                aria-label={revealedKey ? 'Hide key' : 'Reveal key'}
              >
                {revealedKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void copy(revealedKey ?? displayKey, 'API key')}
                disabled={!revealedKey}
                title="Copy key"
                aria-label="Copy API key"
              >
                {copyFlash === 'API key' ? (
                  <Check className="h-4 w-4 text-emerald-600" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
                <span className="sr-only">{keyLabel}</span>
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void rotate()}
                disabled={rotateBusy}
                title="Rotate key"
                aria-label="Rotate API key"
              >
                <RefreshCw className={cn('h-4 w-4', rotateBusy ? 'animate-spin' : '')} />
              </Button>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>MCP server configuration</Label>
            <div className="relative">
              <pre className="overflow-x-auto rounded-xl border border-border-light bg-slate-950 p-4 pr-16 font-mono text-xs leading-relaxed text-slate-100">
                {mcpConfig}
              </pre>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="absolute right-2 top-2"
                onClick={() => void copy(mcpConfig, 'MCP config')}
                aria-label="Copy MCP config"
              >
                {copyFlash === 'MCP config' ? (
                  <Check className="h-4 w-4 text-emerald-600" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
                <span className="sr-only">{configLabel}</span>
              </Button>
            </div>
            <p className="text-xs text-text-tertiary">Add this to your MCP client configuration to connect this agent.</p>
          </div>
        </CardContent>
      </Card>

      <DeleteAgentDialog
        open={deleteOpen}
        agentName={agent.name}
        confirmText={deleteConfirm}
        onConfirmTextChange={setDeleteConfirm}
        deleting={deleting}
        onCancel={closeDelete}
        onConfirm={() => void handleDelete()}
      />
    </div>
  );
}

function DeleteAgentDialog({
  open,
  agentName,
  confirmText,
  onConfirmTextChange,
  deleting,
  onCancel,
  onConfirm,
}: {
  open: boolean;
  agentName: string;
  confirmText: string;
  onConfirmTextChange: (value: string) => void;
  deleting: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}): JSX.Element {
  const canDelete = confirmText === DELETE_CONFIRM_TEXT && !deleting;
  return (
    <Dialog open={open} onOpenChange={(next) => (next ? null : onCancel())}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Delete access point</DialogTitle>
          <DialogDescription>
            This permanently removes <span className="font-medium text-text-primary">{agentName}</span> and revokes its
            API key.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="delete-confirm">
            Type <code className="rounded bg-surface-secondary px-1 py-0.5 text-text-primary">{DELETE_CONFIRM_TEXT}</code>{' '}
            to confirm
          </Label>
          <Input
            id="delete-confirm"
            value={confirmText}
            onChange={(event) => onConfirmTextChange(event.target.value)}
            placeholder={DELETE_CONFIRM_TEXT}
            disabled={deleting}
            autoComplete="off"
          />
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onCancel} disabled={deleting}>
            Cancel
          </Button>
          <Button type="button" variant="destructive" onClick={onConfirm} disabled={!canDelete}>
            {deleting ? 'Deleting…' : 'Delete access point'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
