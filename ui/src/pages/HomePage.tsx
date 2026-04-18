import { CheckCircle2, ChevronDown, ChevronRight, Loader2, RefreshCw, XCircle } from 'lucide-react';
import { Fragment, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { EmptyState } from '../components/EmptyState';
import { PageHeader } from '../components/PageHeader';
import { Button } from '../components/ui/Button';
import { Card, CardContent } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { getMCPEvent, listMCPEvents } from '../services/api';
import type { MCPToolEventDetails, MCPToolEventPage, MCPToolEventRange, MCPToolEventType } from '../types/mcpEvent';
import { cn } from '../utils/cn';

type DetailsState =
  | { status: 'loading' }
  | { status: 'loaded'; data: MCPToolEventDetails }
  | { status: 'error'; error: string };

interface HomePageProps {
  datasourceConfigured: boolean | undefined;
  statusLoaded: boolean;
}

const DEFAULT_LIMIT = 50;
const TIME_RANGES: ReadonlyArray<{ value: MCPToolEventRange; label: string }> = [
  { value: '24h', label: 'Last 24h' },
  { value: '7d', label: 'Last 7d' },
  { value: '30d', label: 'Last 30d' },
  { value: 'all', label: 'All' },
];

export default function HomePage({ datasourceConfigured, statusLoaded }: HomePageProps): JSX.Element | null {
  const navigate = useNavigate();
  const [page, setPage] = useState<MCPToolEventPage>({ items: [], total: 0, limit: DEFAULT_LIMIT, offset: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [timeRange, setTimeRange] = useState<MCPToolEventRange>('7d');
  const [eventTypeFilter, setEventTypeFilter] = useState<MCPToolEventType | ''>('');
  const [toolNameInput, setToolNameInput] = useState('');
  const [toolNameFilter, setToolNameFilter] = useState('');
  const [agentNameInput, setAgentNameInput] = useState('');
  const [agentNameFilter, setAgentNameFilter] = useState('');
  const [offset, setOffset] = useState(0);
  const [expandedRow, setExpandedRow] = useState<string | null>(null);
  const [detailsById, setDetailsById] = useState<Record<string, DetailsState>>({});
  const [refreshKey, setRefreshKey] = useState(0);

  const beginReload = (resetPagination: boolean): void => {
    setLoading(true);
    setError(null);
    setExpandedRow(null);
    setDetailsById({});
    if (resetPagination) {
      setOffset(0);
    }
  };

  const toggleExpansion = (rowId: string): void => {
    setExpandedRow((current) => (current === rowId ? null : rowId));
    setDetailsById((current) => {
      if (current[rowId]) {
        return current;
      }
      const next = { ...current, [rowId]: { status: 'loading' } as DetailsState };
      void getMCPEvent(rowId)
        .then((data) => {
          setDetailsById((state) => ({ ...state, [rowId]: { status: 'loaded', data } }));
        })
        .catch((cause: unknown) => {
          const message = cause instanceof Error ? cause.message : 'Failed to load event details.';
          setDetailsById((state) => ({ ...state, [rowId]: { status: 'error', error: message } }));
        });
      return next;
    });
  };

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const nextFilter = toolNameInput.trim();
      setToolNameFilter((current) => {
        if (current === nextFilter) {
          return current;
        }
        beginReload(true);
        return nextFilter;
      });
    }, 300);
    return () => window.clearTimeout(timer);
  }, [toolNameInput]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const nextFilter = agentNameInput.trim();
      setAgentNameFilter((current) => {
        if (current === nextFilter) {
          return current;
        }
        beginReload(true);
        return nextFilter;
      });
    }, 300);
    return () => window.clearTimeout(timer);
  }, [agentNameInput]);

  useEffect(() => {
    if (!statusLoaded || !datasourceConfigured) {
      return;
    }

    let cancelled = false;

    void listMCPEvents({
      range: timeRange,
      eventType: eventTypeFilter,
      toolName: toolNameFilter,
      agentName: agentNameFilter,
      limit: DEFAULT_LIMIT,
      offset,
    })
      .then((nextPage) => {
        if (!cancelled) {
          setPage(nextPage);
        }
      })
      .catch((cause: unknown) => {
        if (!cancelled) {
          setError(cause instanceof Error ? cause.message : 'Failed to load MCP tool activity.');
          setPage({ items: [], total: 0, limit: DEFAULT_LIMIT, offset });
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [agentNameFilter, datasourceConfigured, eventTypeFilter, offset, refreshKey, statusLoaded, timeRange, toolNameFilter]);

  if (!statusLoaded) {
    return null;
  }

  if (!datasourceConfigured) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Dashboard"
          description="Monitor MCP tool activity by agent, review recent successes and failures, and inspect request details."
        />
        <EmptyState
          title="Start by adding a datasource"
          body="DataClaw needs a datasource before it can track MCP tool activity. Once connected, agent traffic will appear here."
          actions={<Button onClick={() => navigate('/datasource')}>Configure datasource</Button>}
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Dashboard"
        description="Monitor MCP tool activity by agent, review recent successes and failures, and inspect request details."
        actions={
          <Button type="button" variant="outline" onClick={() => { beginReload(false); setRefreshKey((current) => current + 1); }} disabled={loading}>
            <RefreshCw className={cn('h-4 w-4', loading ? 'animate-spin' : undefined)} />
            Refresh
          </Button>
        }
      />

      <Card>
        <CardContent className="space-y-4 p-6">
          <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
            <div className="flex flex-wrap items-center gap-2">
              {TIME_RANGES.map((range) => (
                <button
                  key={range.value}
                  type="button"
                  onClick={() => {
                    if (timeRange === range.value) return;
                    beginReload(true);
                    setTimeRange(range.value);
                  }}
                  className={cn(
                    'rounded-full px-3 py-1.5 text-xs font-medium transition-colors',
                    timeRange === range.value
                      ? 'bg-surface-submit text-white'
                      : 'bg-surface-secondary text-text-secondary hover:bg-surface-hover hover:text-text-primary',
                  )}
                >
                  {range.label}
                </button>
              ))}
              <select
                aria-label="Event type"
                value={eventTypeFilter}
                onChange={(event) => {
                  const nextFilter = event.target.value as MCPToolEventType | '';
                  if (nextFilter === eventTypeFilter) return;
                  beginReload(true);
                  setEventTypeFilter(nextFilter);
                }}
                className="h-10 rounded-lg border border-border-light bg-surface-primary px-3 text-sm text-text-primary"
              >
                <option value="">All events</option>
                <option value="tool_call">Tool call</option>
                <option value="tool_error">Tool error</option>
              </select>
            </div>
            <div className="grid gap-3 sm:grid-cols-2 xl:min-w-[28rem]">
              <Input
                aria-label="Filter by tool"
                placeholder="Filter by tool..."
                value={toolNameInput}
                onChange={(event) => setToolNameInput(event.target.value)}
              />
              <Input
                aria-label="Filter by agent"
                placeholder="Filter by agent..."
                value={agentNameInput}
                onChange={(event) => setAgentNameInput(event.target.value)}
              />
            </div>
          </div>

          {error ? (
            <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">
              {error}
            </div>
          ) : null}

          {loading ? (
            <div className="flex justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-text-secondary" />
            </div>
          ) : page.items.length === 0 ? (
            <EmptyState
              title="No MCP activity yet"
              body="Tracked MCP tool calls will appear here once agents start using query, execute, list_queries, or execute_query."
            />
          ) : (
            <>
              <div className="overflow-x-auto rounded-xl border border-border-light">
                <table className="w-full min-w-[760px] table-fixed text-sm">
                  <thead>
                    <tr className="border-b border-border-light bg-surface-secondary/60 text-left text-xs font-medium uppercase tracking-wider text-text-tertiary">
                      <th className="w-10 px-3 py-3" aria-hidden />
                      <th className="px-3 py-3">Time</th>
                      <th className="px-3 py-3">Agent</th>
                      <th className="px-3 py-3">Tool</th>
                      <th className="px-3 py-3">Event</th>
                      <th className="px-3 py-3 text-right">Duration</th>
                      <th className="px-3 py-3">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.items.map((row) => {
                      const detailsVisible = expandedRow === row.id;
                      return (
                        <Fragment key={row.id}>
                          <tr className={cn('border-b border-border-light/70 align-top', row.wasSuccessful ? 'bg-surface-primary' : 'bg-red-50/40')}>
                            <td className="px-3 py-3">
                              {row.hasDetails ? (
                                <button
                                  type="button"
                                  onClick={() => toggleExpansion(row.id)}
                                  className="rounded-md p-1 text-text-tertiary transition-colors hover:bg-surface-secondary hover:text-text-primary"
                                  aria-label={detailsVisible ? `Collapse details for ${row.toolName}` : `Expand details for ${row.toolName}`}
                                >
                                  {detailsVisible ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                                </button>
                              ) : null}
                            </td>
                            <td className="px-3 py-3 text-text-secondary">{formatTimestamp(row.createdAt)}</td>
                            <td className="px-3 py-3 font-medium text-text-primary">{row.agentName || 'Unknown agent'}</td>
                            <td className="px-3 py-3 text-text-primary">{row.toolName}</td>
                            <td className="px-3 py-3 text-text-secondary">{formatEventType(row.eventType)}</td>
                            <td className="px-3 py-3 text-right tabular-nums text-text-secondary">{formatDuration(row.durationMs)}</td>
                            <td className="px-3 py-3">
                              <StatusBadge wasSuccessful={row.wasSuccessful} />
                            </td>
                          </tr>
                          {detailsVisible ? (
                            <tr className="border-b border-border-light/70 bg-surface-secondary/40">
                              <td colSpan={7} className="px-4 py-4">
                                <DetailsPanel state={detailsById[row.id]} />
                              </td>
                            </tr>
                          ) : null}
                        </Fragment>
                      );
                    })}
                  </tbody>
                </table>
              </div>
              <Pagination
                total={page.total}
                limit={page.limit}
                offset={page.offset}
                onPageChange={(nextOffset) => {
                  if (nextOffset === offset) return;
                  beginReload(false);
                  setOffset(nextOffset);
                }}
              />
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatusBadge({ wasSuccessful }: { wasSuccessful: boolean }): JSX.Element {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium',
        wasSuccessful ? 'bg-emerald-500/15 text-emerald-700' : 'bg-red-500/15 text-red-700',
      )}
    >
      {wasSuccessful ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
      {wasSuccessful ? 'Success' : 'Error'}
    </span>
  );
}

function DetailCard({ title, body, children }: { title: string; body?: string; children?: React.ReactNode }): JSX.Element {
  return (
    <div className="rounded-xl border border-border-light bg-surface-primary p-4">
      <h3 className="text-sm font-semibold text-text-primary">{title}</h3>
      {body !== undefined ? (
        <pre className="mt-2 overflow-x-auto whitespace-pre-wrap break-words text-xs leading-5 text-text-secondary">{body}</pre>
      ) : null}
      {children ? <div className="mt-2 space-y-3">{children}</div> : null}
    </div>
  );
}

function DetailsPanel({ state }: { state: DetailsState | undefined }): JSX.Element {
  if (!state || state.status === 'loading') {
    return (
      <div className="flex items-center gap-2 text-xs text-text-secondary">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
        Loading details...
      </div>
    );
  }
  if (state.status === 'error') {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700" role="alert">
        {state.error}
      </div>
    );
  }
  const { data } = state;
  const errorMessage = data.errorMessage.trim();
  return (
    <div className="space-y-4">
      <DetailCard title="Request summary">
        {data.queryName ? (
          <div>
            <div className="text-xs font-medium uppercase tracking-wider text-text-tertiary">Query</div>
            <div className="mt-1 text-sm text-text-primary">{data.queryName}</div>
          </div>
        ) : null}
        {data.sqlText ? (
          <div>
            <div className="text-xs font-medium uppercase tracking-wider text-text-tertiary">SQL</div>
            <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words rounded-md bg-surface-secondary/60 p-2 text-xs leading-5 text-text-primary">{data.sqlText}</pre>
          </div>
        ) : null}
        <div>
          <div className="text-xs font-medium uppercase tracking-wider text-text-tertiary">Parameters</div>
          <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words text-xs leading-5 text-text-secondary">{formatJSON(data.requestParams)}</pre>
        </div>
      </DetailCard>
      {Object.keys(data.resultSummary).length > 0 ? (
        <DetailCard title="Result summary" body={formatJSON(data.resultSummary)} />
      ) : null}
      {errorMessage ? (
        <div className="rounded-xl border border-red-200 bg-red-50 p-4" role="alert">
          <h3 className="flex items-center gap-1.5 text-sm font-semibold text-red-700">
            <XCircle className="h-4 w-4" aria-hidden />
            Error
          </h3>
          <pre className="mt-2 overflow-x-auto whitespace-pre-wrap break-words text-xs leading-5 text-red-800">{errorMessage}</pre>
        </div>
      ) : null}
    </div>
  );
}

function Pagination({ total, limit, offset, onPageChange }: { total: number; limit: number; offset: number; onPageChange: (offset: number) => void }): JSX.Element | null {
  if (total <= limit) {
    return null;
  }
  const currentPage = Math.floor(offset / limit) + 1;
  const totalPages = Math.ceil(total / limit);
  return (
    <div className="flex flex-col gap-3 text-sm text-text-secondary sm:flex-row sm:items-center sm:justify-between">
      <span>
        Showing {offset + 1}–{Math.min(offset + limit, total)} of {total}
      </span>
      <div className="flex items-center gap-2">
        <Button type="button" variant="outline" size="sm" disabled={currentPage <= 1} onClick={() => onPageChange(Math.max(0, offset - limit))}>
          Previous
        </Button>
        <span>
          Page {currentPage} of {totalPages}
        </span>
        <Button type="button" variant="outline" size="sm" disabled={currentPage >= totalPages} onClick={() => onPageChange(offset + limit)}>
          Next
        </Button>
      </div>
    </div>
  );
}

function formatTimestamp(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function formatDuration(durationMs: number): string {
  if (durationMs < 1000) {
    return `${durationMs}ms`;
  }
  return `${(durationMs / 1000).toFixed(1)}s`;
}

function formatEventType(value: MCPToolEventType): string {
  return value === 'tool_error' ? 'Tool error' : 'Tool call';
}

function formatJSON(value: Record<string, unknown>): string {
  if (Object.keys(value).length === 0) {
    return '{}';
  }
  return JSON.stringify(value, null, 2);
}
