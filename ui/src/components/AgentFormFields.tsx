import { useState } from 'react';

import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Label } from './ui/Label';
import { toMCPKey } from '../lib/mcpSlug';
import type { AgentFormValues, AgentRecord, ApprovedQueryScope } from '../types/agent';
import type { SavedQuery } from '../types/query';
import { cn } from '../utils/cn';

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
    approvedQueryScope: agent.approvedQueryScope,
    approvedQueryIds: [...agent.approvedQueryIds],
  };
}

const SCOPE_OPTIONS: ReadonlyArray<{ value: ApprovedQueryScope; label: string; description: string }> = [
  { value: 'none', label: 'No approved queries', description: 'Hide approved-query tools for this agent.' },
  { value: 'all', label: 'All approved queries', description: 'Expose every current approved query.' },
  { value: 'selected', label: 'Selected approved queries', description: 'Only expose the queries checked below.' },
];

interface AgentFormFieldsProps {
  form: AgentFormValues;
  onChange: (values: AgentFormValues) => void;
  queries: SavedQuery[];
  nameReadOnly?: boolean;
}

export function AgentFormFields({ form, onChange, queries, nameReadOnly = false }: AgentFormFieldsProps): JSX.Element {
  const [showManagerHelp, setShowManagerHelp] = useState(false);
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

  const setScope = (scope: ApprovedQueryScope): void => {
    onChange({
      ...form,
      approvedQueryScope: scope,
      approvedQueryIds: scope === 'selected' ? form.approvedQueryIds : [],
    });
  };

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
            <div className="font-medium text-text-primary">Allow raw query</div>
            <p className="mt-0.5 text-xs">
              Expose the <code className="rounded bg-surface-primary px-1 py-0.5">query</code> tool for ad-hoc read-only SQL.
              {managerEnabled ? ' Required while approved-query management is enabled.' : ''}
            </p>
          </div>
        </label>
        <label className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-secondary p-3 text-sm text-text-secondary">
          <input
            type="checkbox"
            className="mt-1 h-4 w-4 rounded border-border-medium"
            checked={form.canExecute}
            onChange={(event) => setField('canExecute', event.target.checked)}
          />
          <div>
            <div className="font-medium text-text-primary">Allow raw execute</div>
            <p className="mt-0.5 text-xs">
              Expose the dangerous <code className="rounded bg-surface-primary px-1 py-0.5">execute</code> tool for ad-hoc DDL/DML.
            </p>
          </div>
        </label>
        <div className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-secondary p-3 text-sm text-text-secondary">
          <input
            id="agent-manage-approved-queries"
            type="checkbox"
            className="mt-1 h-4 w-4 rounded border-border-medium"
            checked={managerEnabled}
            onChange={(event) =>
              onChange({
                ...form,
                canManageApprovedQueries: event.target.checked,
                canQuery: event.target.checked ? true : form.canQuery,
              })
            }
          />
          <div className="min-w-0 flex-1">
            <label htmlFor="agent-manage-approved-queries" className="font-medium text-text-primary">
              Allow agent to manage approved queries
            </label>
            <p className="mt-0.5 text-xs">
              Expose tools that allow this agent to manage approved queries for other agents to use{' '}
              <button
                type="button"
                className="font-medium text-text-primary underline underline-offset-2"
                aria-expanded={showManagerHelp}
                onClick={() => setShowManagerHelp((current) => !current)}
              >
                learn more
              </button>
              .
            </p>
            {showManagerHelp ? (
              <p className="mt-2 rounded-lg border border-border-light bg-surface-primary px-3 py-2 text-xs text-text-secondary">
                This lets the agent do the hard work of maintaining approved queries based on agents’ needs.
              </p>
            ) : null}
          </div>
        </div>
      </div>

      <div className="space-y-2">
        <Label>Approved queries</Label>
        <div className="grid gap-2">
          {SCOPE_OPTIONS.map((option) => {
            const disabled = option.value === 'selected' && queries.length === 0;
            const active = form.approvedQueryScope === option.value;
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => setScope(option.value)}
                disabled={disabled}
                className={cn(
                  'flex items-start gap-3 rounded-xl border p-3 text-left text-sm transition-colors',
                  active
                    ? 'border-slate-950 bg-slate-950 text-white'
                    : 'border-border-light bg-surface-secondary text-text-primary hover:bg-surface-hover',
                  disabled ? 'cursor-not-allowed opacity-50' : '',
                )}
              >
                <span
                  className={cn(
                    'mt-0.5 flex h-4 w-4 items-center justify-center rounded-full border',
                    active ? 'border-white bg-white' : 'border-border-medium bg-surface-primary',
                  )}
                  aria-hidden
                >
                  {active ? <span className="h-2 w-2 rounded-full bg-slate-950" /> : null}
                </span>
                <span>
                  <span className="block font-medium">{option.label}</span>
                  <span className={cn('mt-0.5 block text-xs', active ? 'text-slate-200' : 'text-text-secondary')}>
                    {option.description}
                  </span>
                </span>
              </button>
            );
          })}
        </div>

        {form.approvedQueryScope === 'selected' ? (
          <div className="rounded-xl border border-border-light bg-surface-secondary p-3">
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
                    className="flex items-start gap-3 rounded-lg border border-border-light bg-surface-primary p-2 text-sm"
                  >
                    <input
                      type="checkbox"
                      className="mt-1 h-4 w-4 rounded border-border-medium"
                      checked={form.approvedQueryIds.includes(query.id)}
                      onChange={() => toggleApproved(query.id)}
                    />
                    <span className="min-w-0 flex-1">
                      <span
                        className="block truncate font-medium text-text-primary"
                        title={query.naturalLanguagePrompt}
                      >
                        {query.naturalLanguagePrompt || 'Untitled query'}
                      </span>
                      <span className="block truncate text-xs text-text-tertiary" title={query.sql}>
                        {query.sql}
                      </span>
                    </span>
                  </label>
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
