import { Pencil, Plus, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { EmptyState } from '../components/EmptyState';
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
import { deleteAgent, getStatus, listAgents } from '../services/api';
import type { AgentRecord } from '../types/agent';
import type { RuntimeStatus } from '../types/datasource';
import { cn } from '../utils/cn';

const DELETE_CONFIRM_TEXT = 'delete access point';

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

export default function AgentsPage(): JSX.Element {
  const { refresh } = useOutletContext<AppOutletContext>();
  const { toast } = useToast();
  const navigate = useNavigate();

  const [runtime, setRuntime] = useState<RuntimeStatus | null>(null);
  const [agents, setAgents] = useState<AgentRecord[]>([]);
  const [loading, setLoading] = useState(true);

  const [deleteTarget, setDeleteTarget] = useState<AgentRecord | null>(null);
  const [deleteConfirmText, setDeleteConfirmText] = useState('');
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async (): Promise<void> => {
    try {
      const [nextRuntime, nextAgents] = await Promise.all([getStatus(), listAgents()]);
      setRuntime(nextRuntime);
      setAgents(nextAgents);
    } catch (error) {
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to load access points.',
        variant: 'error',
      });
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    void load();
  }, [load]);

  const openDeleteDialog = (agent: AgentRecord): void => {
    setDeleteTarget(agent);
    setDeleteConfirmText('');
  };

  const closeDeleteDialog = (): void => {
    setDeleteTarget(null);
    setDeleteConfirmText('');
  };

  const handleDelete = async (): Promise<void> => {
    if (!deleteTarget || deleteConfirmText !== DELETE_CONFIRM_TEXT) return;
    setDeleting(true);
    try {
      await deleteAgent(deleteTarget.id);
      closeDeleteDialog();
      await Promise.all([load(), refresh()]);
      toast({ title: 'Access point deleted', description: `${deleteTarget.name} removed.`, variant: 'success' });
    } catch (error) {
      toast({
        title: 'Delete failed',
        description: error instanceof Error ? error.message : 'Failed to delete access point.',
        variant: 'error',
      });
    } finally {
      setDeleting(false);
    }
  };

  if (!loading && !runtime?.datasourceConfigured) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Agent Access"
          description="Create access points that give agents scoped raw-tool permissions, approved-query access, and their own API keys."
        />
        <EmptyState
          title="Start by adding a datasource"
          body="DataClaw needs a datasource before agents can run queries or expose approved-query tools. Once connected, you can create access points and scope what each agent can do."
          actions={<Button onClick={() => navigate('/datasource')}>Configure datasource</Button>}
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Agent Access"
        description="Create access points that give agents scoped raw-tool permissions, approved-query access, and their own API keys."
        actions={
          <Button type="button" onClick={() => navigate('/agents/new')}>
            <Plus className="h-4 w-4" />
            New Access Point
          </Button>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">Access Points</CardTitle>
          <CardDescription>
            {loading
              ? 'Loading access points…'
              : agents.length === 0
                ? 'No access points yet.'
                : `${agents.length} ${agents.length === 1 ? 'access point' : 'access points'}.`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? null : agents.length === 0 ? (
            <EmptyState
              title="No access points yet"
              body="Create an access point to mint an API key and scope the raw tools and approved queries an agent can use."
              actions={
                <Button type="button" onClick={() => navigate('/agents/new')}>
                  <Plus className="h-4 w-4" />
                  New Access Point
                </Button>
              }
            />
          ) : (
            <AgentTable
              agents={agents}
              onRowClick={(agent) => navigate(`/agents/${agent.id}`)}
              onEdit={(agent) => navigate(`/agents/${agent.id}/edit`)}
              onDelete={(agent) => openDeleteDialog(agent)}
            />
          )}
        </CardContent>
      </Card>

      <DeleteAgentDialog
        open={Boolean(deleteTarget)}
        agent={deleteTarget}
        confirmText={deleteConfirmText}
        onConfirmTextChange={setDeleteConfirmText}
        deleting={deleting}
        onCancel={closeDeleteDialog}
        onConfirm={() => void handleDelete()}
      />
    </div>
  );
}

function AgentTable({
  agents,
  onRowClick,
  onEdit,
  onDelete,
}: {
  agents: AgentRecord[];
  onRowClick: (agent: AgentRecord) => void;
  onEdit: (agent: AgentRecord) => void;
  onDelete: (agent: AgentRecord) => void;
}): JSX.Element {
  return (
    <div className="overflow-hidden rounded-xl border border-border-light">
      <div className="grid grid-cols-[minmax(180px,2fr)_minmax(180px,1.3fr)_minmax(110px,1fr)_minmax(140px,1.2fr)_auto] items-center border-b border-border-light bg-surface-secondary/60 px-4 py-2 text-xs font-medium uppercase tracking-wider text-text-tertiary">
        <span>Agent</span>
        <span>Tools</span>
        <span>Created</span>
        <span>Last used</span>
        <span className="w-16" aria-hidden />
      </div>
      <div className="divide-y divide-border-light">
        {agents.map((agent) => (
          <div
            key={agent.id}
            role="button"
            tabIndex={0}
            className="group grid cursor-pointer grid-cols-[minmax(180px,2fr)_minmax(180px,1.3fr)_minmax(110px,1fr)_minmax(140px,1.2fr)_auto] items-center px-4 py-3 transition-colors hover:bg-surface-hover focus-visible:bg-surface-hover focus-visible:outline-none"
            onClick={() => onRowClick(agent)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                onRowClick(agent);
              }
            }}
          >
            <div className="min-w-0">
              <div className="truncate font-medium text-text-primary">{agent.name}</div>
            </div>
            <ToolsPills agent={agent} />
            <div className="text-sm tabular-nums text-text-secondary">{formatDate(agent.createdAt)}</div>
            <div className="text-sm tabular-nums text-text-secondary">{formatTimestamp(agent.lastUsedAt)}</div>
            <div className="flex w-16 items-center justify-end">
              <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100">
                <button
                  type="button"
                  onClick={(event) => {
                    event.stopPropagation();
                    onEdit(agent);
                  }}
                  className="rounded p-1.5 text-text-tertiary transition-colors hover:bg-surface-secondary hover:text-text-primary"
                  title={`Edit ${agent.name}`}
                  aria-label={`Edit ${agent.name}`}
                >
                  <Pencil className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  onClick={(event) => {
                    event.stopPropagation();
                    onDelete(agent);
                  }}
                  className="rounded p-1.5 text-text-tertiary transition-colors hover:bg-red-50 hover:text-red-600"
                  title={`Delete ${agent.name}`}
                  aria-label={`Delete ${agent.name}`}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function DeleteAgentDialog({
  open,
  agent,
  confirmText,
  onConfirmTextChange,
  deleting,
  onCancel,
  onConfirm,
}: {
  open: boolean;
  agent: AgentRecord | null;
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
            This permanently removes{' '}
            {agent ? <span className="font-medium text-text-primary">{agent.name}</span> : 'the access point'} and revokes its
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
