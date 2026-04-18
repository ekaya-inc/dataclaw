import { AlertTriangle, ArrowLeft, Pencil, Play, Trash2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext, useParams } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { DeleteQueryDialog } from '../components/DeleteQueryDialog';
import { PageHeader } from '../components/PageHeader';
import { ParameterInputDialog } from '../components/ParameterInputDialog';
import { QueryResultsTable } from '../components/QueryResultsTable';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Label } from '../components/ui/Label';
import { useToast } from '../components/ui/Toast';
import { useStoredParameterValues } from '../hooks/useStoredParameterValues';
import { deleteQuery, executeSavedQuery, getQuery } from '../services/api';
import type { OutputColumn, QueryExecutionResult, QueryParameter, SavedQuery } from '../types/query';

function formatTimestamp(value?: string): string {
  if (!value) return 'recently';
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString();
}

function ReadonlyBlock({ children, emptyLabel }: { children?: string; emptyLabel: string }): JSX.Element {
  return (
    <div className="min-h-10 rounded-xl border border-border-light bg-surface-secondary/60 px-3 py-2 text-sm text-text-primary whitespace-pre-wrap">
      {children && children.trim() ? children : <span className="text-text-tertiary">{emptyLabel}</span>}
    </div>
  );
}

function QueryParameterSummary({ parameters }: { parameters: QueryParameter[] }): JSX.Element {
  return (
    <div className="space-y-3">
      <div>
        <h3 className="text-sm font-semibold text-text-primary">Parameters</h3>
        <p className="text-sm text-text-secondary">Required parameters must be provided by callers.</p>
      </div>
      {parameters.length === 0 ? (
        <ReadonlyBlock emptyLabel="No parameters defined." />
      ) : (
        <ul className="space-y-3">
          {parameters.map((parameter) => (
            <li key={parameter.name} className="rounded-xl border border-border-light bg-surface-secondary/60 p-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-text-primary">{parameter.name}</span>
                <span className="text-xs text-text-tertiary">({parameter.type})</span>
                <span className="rounded-full bg-surface-primary px-2 py-0.5 text-xs text-text-secondary">
                  {parameter.required ? 'Required' : 'Optional'}
                </span>
              </div>
              {parameter.description ? <p className="mt-2 text-sm text-text-secondary">{parameter.description}</p> : null}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function OutputColumnSummary({ outputColumns }: { outputColumns: OutputColumn[] }): JSX.Element {
  return (
    <div className="space-y-3">
      <div>
        <h3 className="text-sm font-semibold text-text-primary">Output columns</h3>
        <p className="text-sm text-text-secondary">Documented shape for agents.</p>
      </div>
      {outputColumns.length === 0 ? (
        <ReadonlyBlock emptyLabel="No output columns documented." />
      ) : (
        <ul className="space-y-3">
          {outputColumns.map((column) => (
            <li key={column.name} className="rounded-xl border border-border-light bg-surface-secondary/60 p-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-text-primary">{column.name}</span>
                <span className="text-xs text-text-tertiary">({column.type})</span>
              </div>
              {column.description ? <p className="mt-2 text-sm text-text-secondary">{column.description}</p> : null}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export default function QueryDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { refresh } = useOutletContext<AppOutletContext>();
  const { toast } = useToast();

  const [query, setQuery] = useState<SavedQuery | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<'executing' | 'deleting' | null>(null);
  const [results, setResults] = useState<QueryExecutionResult | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [parameterDialogOpen, setParameterDialogOpen] = useState(false);
  const storageKey = `dataclaw.queryParams.detail.${id ?? 'unknown'}`;
  const [storedValues, setStoredValues] = useStoredParameterValues(storageKey);

  useEffect(() => {
    let cancelled = false;

    if (!id) {
      setQuery(null);
      setLoading(false);
      return () => {
        cancelled = true;
      };
    }

    void (async () => {
      setLoading(true);
      setQuery(null);
      setResults(null);
      setDeleteDialogOpen(false);
      setParameterDialogOpen(false);
      try {
        const savedQuery = await getQuery(id);
        if (cancelled) return;
        setQuery(savedQuery);
      } catch (error) {
        if (cancelled) return;
        toast({
          variant: 'error',
          title: 'Failed to load approved query',
          description: error instanceof Error ? error.message : undefined,
        });
        navigate('/queries');
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [id, navigate, toast]);

  const runSavedQuery = async (values: Record<string, unknown>): Promise<void> => {
    if (!query) return;
    setBusy('executing');
    try {
      const execution = await executeSavedQuery(query.id, values, 100);
      setResults(execution);
      toast({ variant: 'success', title: 'Approved query executed' });
      setParameterDialogOpen(false);
    } catch (error) {
      toast({
        variant: 'error',
        title: 'Execution failed',
        description: error instanceof Error ? error.message : undefined,
      });
    } finally {
      setBusy(null);
    }
  };

  const handleExecuteClick = (): void => {
    if (!query) return;
    if (query.parameters.length === 0) {
      void runSavedQuery({});
      return;
    }
    setParameterDialogOpen(true);
  };

  const handleDialogSubmit = (values: Record<string, unknown>): void => {
    setStoredValues(values);
    void runSavedQuery(values);
  };

  const removeSelectedQuery = async (): Promise<void> => {
    if (!query) return;
    setBusy('deleting');
    try {
      await deleteQuery(query.id);
      await refresh();
      toast({ variant: 'success', title: 'Approved query deleted' });
      navigate('/queries');
    } catch (error) {
      toast({
        variant: 'error',
        title: 'Failed to delete query',
        description: error instanceof Error ? error.message : undefined,
      });
      setDeleteDialogOpen(false);
      setBusy(null);
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <div>
          <Button variant="ghost" size="sm" onClick={() => navigate('/queries')}>
            <ArrowLeft className="h-4 w-4" />
            Back to approved queries
          </Button>
        </div>
        <div className="rounded-xl border border-dashed border-border-light bg-surface-secondary/60 px-4 py-6 text-sm text-text-secondary">
          Loading approved query…
        </div>
      </div>
    );
  }

  if (!query) return <></>;

  return (
    <div className="space-y-6">
      <div>
        <Button variant="ghost" size="sm" onClick={() => navigate('/queries')}>
          <ArrowLeft className="h-4 w-4" />
          Back to approved queries
        </Button>
      </div>

      <PageHeader
        title={query.naturalLanguagePrompt || 'Untitled query'}
        description={query.additionalContext || 'Read-only approved query definition. Use Edit to update SQL or metadata.'}
        actions={
          <>
            <Button
              type="button"
              variant="outline"
              onClick={() => setDeleteDialogOpen(true)}
              disabled={busy !== null}
              className="border-red-200 text-red-600 hover:bg-red-50 hover:text-red-700"
            >
              <Trash2 className="h-4 w-4" />
              Delete
            </Button>
            <Button type="button" variant="outline" onClick={() => navigate(`/queries/${query.id}/edit`)}>
              <Pencil className="h-4 w-4" />
              Edit
            </Button>
          </>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">Overview</CardTitle>
          <CardDescription>Saved query definition. Last updated {formatTimestamp(query.updatedAt)}.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="grid gap-4 lg:grid-cols-2">
            <div className="space-y-2">
              <Label>Natural language prompt</Label>
              <ReadonlyBlock emptyLabel="No prompt provided.">{query.naturalLanguagePrompt}</ReadonlyBlock>
            </div>
            <div className="space-y-2">
              <Label>Additional context</Label>
              <ReadonlyBlock emptyLabel="No additional context.">{query.additionalContext}</ReadonlyBlock>
            </div>
          </div>

          <div className="space-y-2">
            <Label>SQL</Label>
            <pre className="overflow-x-auto rounded-xl border border-border-light bg-slate-950 p-4 text-sm text-slate-100">
              <code>{query.sql}</code>
            </pre>
          </div>

          <div className="rounded-xl border border-border-light bg-surface-secondary p-4">
            <div className="flex flex-wrap items-center gap-3">
              <span className="text-sm font-medium text-text-primary">Query type</span>
              {query.allowsModification ? (
                <span className="inline-flex items-center gap-2 rounded-md bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800">
                  <AlertTriangle className="h-3 w-3" />
                  Can modify rows
                </span>
              ) : (
                <span className="rounded-md bg-emerald-100 px-2 py-1 text-xs font-medium text-emerald-800">Read only</span>
              )}
            </div>
            <p className="mt-2 text-sm text-text-secondary">
              Agents can only execute the saved query definition shown here. Edit this query to change its SQL or metadata.
            </p>
          </div>

          <div className="grid gap-6 xl:grid-cols-2">
            <QueryParameterSummary parameters={query.parameters} />
            <OutputColumnSummary outputColumns={query.outputColumns} />
          </div>

          <div className="space-y-2">
            <Label>Constraints</Label>
            <ReadonlyBlock emptyLabel="No constraints documented.">{query.constraints}</ReadonlyBlock>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">Execute saved query</CardTitle>
          <CardDescription>Run the approved query exactly as saved. Draft testing lives on the edit page.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap gap-3">
            <Button type="button" variant="outline" onClick={handleExecuteClick} disabled={busy !== null}>
              <Play className="h-4 w-4" />
              Execute saved query
            </Button>
          </div>

          {query.parameters.length > 0 ? (
            <p className="text-sm text-text-secondary">
              You&apos;ll be prompted for execution parameter values before the query runs.
            </p>
          ) : null}
        </CardContent>
      </Card>

      {results ? <QueryResultsTable columns={results.columns} rows={results.rows} rowCount={results.rowCount} /> : null}

      <ParameterInputDialog
        open={parameterDialogOpen}
        onOpenChange={setParameterDialogOpen}
        parameters={query.parameters}
        initialValues={storedValues}
        title="Execute saved query"
        description="Enter values for this query's parameters. Defaults are used when present."
        submitLabel="Execute"
        submitting={busy === 'executing'}
        onSubmit={handleDialogSubmit}
      />

      <DeleteQueryDialog
        open={deleteDialogOpen}
        queryPrompt={query.naturalLanguagePrompt}
        disabled={busy !== null}
        deleting={busy === 'deleting'}
        onCancel={() => setDeleteDialogOpen(false)}
        onConfirm={() => void removeSelectedQuery()}
      />
    </div>
  );
}
