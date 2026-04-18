import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Check, ChevronUp, Sparkles } from 'lucide-react';

import { ApprovedQueryManagerHelp } from './ApprovedQueryManagerHelp';
import { DangerousExecuteDialog } from './DangerousExecuteDialog';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Label } from './ui/Label';
import { toMCPKey } from '../lib/mcpSlug';
import type { AgentFormValues, AgentRecord, ApprovedQueryScope } from '../types/agent';
import type { SavedQuery } from '../types/query';
import { cn } from '../utils/cn';

const APPROVED_QUERY_MANAGER_HELP_PANEL_ID = 'approved-query-manager-help';

export const EMPTY_AGENT_FORM: AgentFormValues = {
  name: '',
  canQuery: true,
  canExecute: false,
  canManageApprovedQueries: false,
  approvedQueryScope: 'none',
  approvedQueryIds: [],
};

export function agentFormFromRecord(agent: AgentRecord): AgentFormValues {
  const canManageApprovedQueries = Boolean(agent.canManageApprovedQueries);
  return {
    name: agent.name,
    canQuery: canManageApprovedQueries ? true : agent.canQuery,
    canExecute: agent.canExecute,
    canManageApprovedQueries,
    approvedQueryScope: canManageApprovedQueries ? 'all' : agent.approvedQueryScope,
    approvedQueryIds: canManageApprovedQueries ? [] : [...agent.approvedQueryIds],
  };
}

const SCOPE_OPTIONS: ReadonlyArray<{ value: ApprovedQueryScope; label: string }> = [
  { value: 'none', label: 'No approved queries' },
  { value: 'all', label: 'All approved queries' },
  { value: 'selected', label: 'Selected approved queries' },
];

interface AgentFormFieldsProps {
  form: AgentFormValues;
  onChange: (values: AgentFormValues) => void;
  queries: SavedQuery[];
  nameReadOnly?: boolean;
}

export function AgentFormFields({ form, onChange, queries, nameReadOnly = false }: AgentFormFieldsProps): JSX.Element {
  const [showManagerHelp, setShowManagerHelp] = useState(false);
  const [panelOpen, setPanelOpen] = useState(true);
  const [executeDialogOpen, setExecuteDialogOpen] = useState(false);
  const managerEnabled = Boolean(form.canManageApprovedQueries);

  const setField = <K extends keyof AgentFormValues>(key: K, value: AgentFormValues[K]): void => {
    onChange({ ...form, [key]: value });
  };

  const toggleApproved = (queryId: string): void => {
    const hasQuery = form.approvedQueryIds.includes(queryId);
    onChange({
      ...form,
      approvedQueryIds: hasQuery
        ? form.approvedQueryIds.filter((id) => id !== queryId)
        : [...form.approvedQueryIds, queryId],
    });
  };

  const scopeHasPanel = (scope: ApprovedQueryScope): boolean => scope === 'selected' || scope === 'all';

  const handleScopeClick = (scope: ApprovedQueryScope): void => {
    if (scopeHasPanel(scope) && form.approvedQueryScope === scope) {
      setPanelOpen((open) => !open);
      return;
    }
    setPanelOpen(true);
    onChange({
      ...form,
      approvedQueryScope: scope,
      approvedQueryIds: scope === 'selected' ? form.approvedQueryIds : [],
    });
  };

  const panelVisible = scopeHasPanel(form.approvedQueryScope) && panelOpen;

  const slugPreview = toMCPKey(form.name || 'agent');

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <Label htmlFor="agent-name">Name</Label>
        <Input
          id="agent-name"
          value={form.name}
          onChange={(event) => setField('name', event.target.value)}
          placeholder="Marketing bot"
          readOnly={nameReadOnly}
        />
        {!nameReadOnly ? (
          <p className="text-xs text-text-tertiary">
            In your MCP config this becomes{' '}
            <code className="rounded bg-surface-secondary px-1 py-0.5 text-text-primary">{slugPreview}</code>.
          </p>
        ) : null}
      </div>

      <div className="space-y-2">
        <Label>Tools</Label>
        <label className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-secondary p-3 text-sm text-text-secondary">
          <input
            type="checkbox"
            className="mt-1 h-4 w-4 rounded border-border-medium"
            checked={managerEnabled ? true : form.canQuery}
            onChange={(event) => setField('canQuery', managerEnabled ? true : event.target.checked)}
            disabled={managerEnabled}
          />
          <div>
            <div className="font-medium text-text-primary">Allow agent to query entire database</div>
            <p className="mt-0.5 text-xs">
              Expose the <code className="rounded bg-surface-primary px-1 py-0.5">query</code> tool for ad-hoc read-only SQL.
              This query will have access to anything the datasource credentials allow including any tables/columns and schema.
              {managerEnabled ? ' Required while approved-query management is enabled.' : ''}
            </p>
          </div>
        </label>
        <label className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-secondary p-3 text-sm text-text-secondary">
          <input
            type="checkbox"
            className="mt-1 h-4 w-4 rounded border-border-medium"
            checked={form.canExecute}
            onChange={(event) => {
              if (event.target.checked) {
                setExecuteDialogOpen(true);
                return;
              }
              setField('canExecute', false);
            }}
          />
          <div>
            <div className="font-medium text-text-primary">Allow agent full write access to the database</div>
            <p className="mt-0.5 text-xs">
              Expose the <code className="rounded bg-surface-primary px-1 py-0.5">execute</code> tool which gives full
              permissions granted by the datasource credentials — potentially including{' '}
              <code className="rounded bg-surface-primary px-1 py-0.5">create</code>,{' '}
              <code className="rounded bg-surface-primary px-1 py-0.5">alter</code>, and{' '}
              <code className="rounded bg-surface-primary px-1 py-0.5">drop database</code> as well as{' '}
              <code className="rounded bg-surface-primary px-1 py-0.5">insert</code>,{' '}
              <code className="rounded bg-surface-primary px-1 py-0.5">update</code>, and{' '}
              <code className="rounded bg-surface-primary px-1 py-0.5">delete</code> rows of data.
            </p>
          </div>
        </label>
        {executeDialogOpen ? (
          <DangerousExecuteDialog
            onCancel={() => setExecuteDialogOpen(false)}
            onConfirm={() => {
              setExecuteDialogOpen(false);
              setField('canExecute', true);
            }}
          />
        ) : null}
        <div className="space-y-3 rounded-xl border border-border-light bg-surface-secondary p-3 text-sm text-text-secondary">
          <div className="flex items-start gap-3">
            <input
              id="agent-manage-approved-queries"
              type="checkbox"
              className="mt-1 h-4 w-4 rounded border-border-medium"
              checked={managerEnabled}
              onChange={(event) => {
                const enabling = event.target.checked;
                if (enabling) {
                  setPanelOpen(true);
                }
                onChange({
                  ...form,
                  canManageApprovedQueries: enabling,
                  canQuery: enabling ? true : form.canQuery,
                  approvedQueryScope: enabling ? 'all' : form.approvedQueryScope,
                  approvedQueryIds: enabling ? [] : form.approvedQueryIds,
                });
              }}
            />
            <div className="min-w-0 flex-1">
              <label htmlFor="agent-manage-approved-queries" className="font-medium text-text-primary">
                Allow agent to manage approved queries
              </label>
              <p className="mt-0.5 text-xs">
                Expose tools that let this agent curate approved queries — schema discovery, prototyping, and full
                metadata — so other agents can call them via{' '}
                <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-[11px] text-text-primary">execute_query</code>.
              </p>
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              aria-expanded={showManagerHelp}
              aria-controls={APPROVED_QUERY_MANAGER_HELP_PANEL_ID}
              onClick={() => setShowManagerHelp((current) => !current)}
              className="border-violet-300 bg-violet-50 text-violet-700 hover:bg-violet-100 hover:text-violet-800"
            >
              {showManagerHelp ? (
                <ChevronUp className="h-4 w-4" />
              ) : (
                <Sparkles className="h-4 w-4 text-violet-500" />
              )}
              Learn more
            </Button>
          </div>
          {showManagerHelp ? <ApprovedQueryManagerHelp panelId={APPROVED_QUERY_MANAGER_HELP_PANEL_ID} /> : null}
        </div>
      </div>

      <div className="space-y-2">
        <Label>Approved queries</Label>
        <div
          className={cn(
            'flex border border-border-light overflow-hidden',
            panelVisible ? 'rounded-t-xl' : 'rounded-xl',
          )}
        >
          {SCOPE_OPTIONS.map((option, index) => {
            const lockedByManager = managerEnabled && option.value !== 'all';
            const disabled = lockedByManager || (option.value === 'selected' && queries.length === 0);
            const active = form.approvedQueryScope === option.value;
            const expanded = scopeHasPanel(option.value) ? active && panelOpen : undefined;
            return (
              <button
                key={option.value}
                type="button"
                role="radio"
                aria-checked={active}
                aria-expanded={expanded}
                onClick={() => handleScopeClick(option.value)}
                disabled={disabled}
                className={cn(
                  'flex flex-1 items-center justify-center gap-2 px-4 py-3 text-sm transition-colors',
                  active
                    ? 'bg-surface-secondary text-text-primary'
                    : 'bg-surface-primary text-text-primary hover:bg-surface-secondary/50',
                  index > 0 ? 'border-l border-border-light' : '',
                  disabled ? 'cursor-not-allowed opacity-50' : '',
                )}
              >
                {active ? <Check className="h-4 w-4 text-emerald-600" aria-hidden /> : null}
                <span className="font-medium">{option.label}</span>
              </button>
            );
          })}
        </div>

        {panelVisible && form.approvedQueryScope === 'selected' ? (
          <div className="rounded-b-xl border border-t-0 border-border-light bg-surface-secondary p-4">
            <p className="mb-3 text-sm text-text-secondary">
              This agent will only have access to the queries you check below. Manage the catalog on the{' '}
              <Link to="/queries" className="font-medium text-text-primary underline underline-offset-2">
                Approved queries
              </Link>{' '}
              page.
            </p>
            <div className="mb-2 flex items-center justify-between">
              <div className="text-xs text-text-secondary">Select at least one query for this agent.</div>
              <div className="flex gap-2">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => onChange({ ...form, approvedQueryIds: queries.map((query) => query.id) })}
                  disabled={queries.length === 0}
                >
                  Select all
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => onChange({ ...form, approvedQueryIds: [] })}
                  disabled={form.approvedQueryIds.length === 0}
                >
                  Clear
                </Button>
              </div>
            </div>
            {queries.length === 0 ? (
              <p className="text-sm text-text-secondary">No approved queries available yet.</p>
            ) : (
              <div className="max-h-60 space-y-1.5 overflow-y-auto">
                {queries.map((query) => (
                  <label
                    key={query.id}
                    className="flex items-center gap-3 rounded-lg border border-border-light bg-surface-primary p-2 text-sm"
                  >
                    <input
                      type="checkbox"
                      className="h-4 w-4 rounded border-border-medium"
                      checked={form.approvedQueryIds.includes(query.id)}
                      onChange={() => toggleApproved(query.id)}
                    />
                    <span
                      className="min-w-0 flex-1 truncate font-medium text-text-primary"
                      title={query.naturalLanguagePrompt}
                    >
                      {query.naturalLanguagePrompt || 'Untitled query'}
                    </span>
                  </label>
                ))}
              </div>
            )}
          </div>
        ) : null}

        {panelVisible && form.approvedQueryScope === 'all' ? (
          <div className="rounded-b-xl border border-t-0 border-border-light bg-surface-secondary p-4">
            <p className="mb-3 text-sm text-text-secondary">
              This agent will have access to all approved queries (even ones added in the future).
              {managerEnabled ? ' Required while approved-query management is enabled.' : ''} Manage the catalog on the{' '}
              <Link to="/queries" className="font-medium text-text-primary underline underline-offset-2">
                Approved queries
              </Link>{' '}
              page.
            </p>
            {queries.length === 0 ? (
              <p className="text-sm text-text-secondary">No approved queries available yet.</p>
            ) : (
              <div className="max-h-60 space-y-1.5 overflow-y-auto">
                {queries.map((query) => (
                  <div
                    key={query.id}
                    className="flex items-center gap-3 rounded-lg border border-border-light bg-surface-primary p-2 text-sm"
                  >
                    <input
                      type="checkbox"
                      className="h-4 w-4 rounded border-border-medium"
                      checked
                      disabled
                      readOnly
                      aria-label={`${query.naturalLanguagePrompt || 'Untitled query'} (always included)`}
                    />
                    <span
                      className="min-w-0 flex-1 truncate font-medium text-text-primary"
                      title={query.naturalLanguagePrompt}
                    >
                      {query.naturalLanguagePrompt || 'Untitled query'}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export function isAgentFormSubmittable(form: AgentFormValues): boolean {
  if (form.name.trim().length === 0) return false;
  if (form.approvedQueryScope === 'selected' && form.approvedQueryIds.length === 0) return false;
  return true;
}
