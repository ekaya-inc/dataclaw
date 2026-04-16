import { CheckCircle2, FlaskConical, Pencil, Play, Plus, Save, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useOutletContext } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { QUERY_TEMPLATE } from '../constants';
import { EmptyState } from '../components/EmptyState';
import { PageHeader } from '../components/PageHeader';
import { ParameterEditor } from '../components/ParameterEditor';
import { ParameterInputForm } from '../components/ParameterInputForm';
import { QueryResultsTable } from '../components/QueryResultsTable';
import { SqlEditor } from '../components/SqlEditor';
import { StatusBanner } from '../components/StatusBanner';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { createQuery, deleteQuery, executeSavedQuery, getDatasource, listQueries, testQuery, updateQuery, validateQuery } from '../services/api';
import type { DatasourceRecord } from '../types/datasource';
import { datasourceTypeToDialect } from '../types/query';
import type { QueryExecutionResult, QueryValidationResult, SavedQuery } from '../types/query';

const EMPTY_QUERY: Omit<SavedQuery, 'id'> = {
  datasourceId: undefined,
  name: '',
  description: '',
  sql: QUERY_TEMPLATE,
  isEnabled: true,
  parameters: [],
};

function queryParameterHasDefault(value: unknown): boolean {
  return value !== null && value !== undefined;
}

function hasProvidedQueryParameterValue(value: unknown): boolean {
  if (value === null || value === undefined) {
    return false;
  }
  if (typeof value === 'string') {
    return value.trim() !== '';
  }
  if (Array.isArray(value)) {
    return value.length > 0;
  }
  return true;
}

function canExecuteSavedQuery(query: SavedQuery, values: Record<string, unknown>): boolean {
  if (!query.isEnabled) {
    return false;
  }

  return query.parameters.every((parameter) => {
    if (!parameter.required) {
      return true;
    }

    const hasExplicitValue = Object.prototype.hasOwnProperty.call(values, parameter.name);
    if (hasExplicitValue) {
      return hasProvidedQueryParameterValue(values[parameter.name]);
    }

    return queryParameterHasDefault(parameter.default);
  });
}

export default function ApprovedQueriesPage(): JSX.Element {
  const { refresh } = useOutletContext<AppOutletContext>();
  const [datasource, setDatasource] = useState<DatasourceRecord | null>(null);
  const [queries, setQueries] = useState<SavedQuery[]>([]);
  const [selectedQueryId, setSelectedQueryId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Omit<SavedQuery, 'id'>>(EMPTY_QUERY);
  const [executeParameterValues, setExecuteParameterValues] = useState<Record<string, unknown>>({});
  const [busy, setBusy] = useState<'loading' | 'saving' | 'validating' | 'testing' | 'executing' | 'deleting' | null>('loading');
  const [feedback, setFeedback] = useState<{ tone: 'info' | 'success' | 'danger'; message: string } | null>(null);
  const [validation, setValidation] = useState<QueryValidationResult | null>(null);
  const [results, setResults] = useState<QueryExecutionResult | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        const [currentDatasource, currentQueries] = await Promise.all([getDatasource(), listQueries()]);
        setDatasource(currentDatasource);
        setQueries(currentQueries);
        if (currentQueries[0]) {
          setSelectedQueryId(currentQueries[0].id);
          setDraft({ ...currentQueries[0], datasourceId: currentQueries[0].datasourceId });
        }
      } catch (error) {
        setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to load approved queries.' });
      } finally {
        setBusy(null);
      }
    })();
  }, []);

  const selectedQuery = useMemo(
    () => queries.find((query) => query.id === selectedQueryId) ?? null,
    [queries, selectedQueryId],
  );
  const canExecuteSelectedQuery = selectedQuery ? canExecuteSavedQuery(selectedQuery, executeParameterValues) : false;

  const dialect = datasource ? datasourceTypeToDialect[datasource.type] : 'PostgreSQL';

  const resetDraft = (nextQuery?: SavedQuery | null): void => {
    setExecuteParameterValues({});
    if (!nextQuery) {
      setSelectedQueryId(null);
      setDraft({ ...EMPTY_QUERY, datasourceId: datasource?.id });
      setValidation(null);
      setResults(null);
      return;
    }

    setSelectedQueryId(nextQuery.id);
    setDraft({
      datasourceId: nextQuery.datasourceId,
      name: nextQuery.name,
      description: nextQuery.description ?? '',
      sql: nextQuery.sql,
      isEnabled: nextQuery.isEnabled,
      parameters: nextQuery.parameters,
    });
    setValidation(null);
    setResults(null);
  };

  const persistDraft = async (): Promise<void> => {
    setBusy('saving');
    setFeedback(null);
    try {
      const payload = { ...draft, datasourceId: datasource?.id };
      const saved = selectedQueryId ? await updateQuery(selectedQueryId, payload) : await createQuery(payload);
      const nextQueries = selectedQueryId
        ? queries.map((query) => (query.id === saved.id ? saved : query))
        : [saved, ...queries];
      setQueries(nextQueries);
      resetDraft(saved);
      setFeedback({ tone: 'success', message: selectedQueryId ? 'Approved query updated.' : 'Approved query created.' });
      void refresh();
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to save query.' });
    } finally {
      setBusy(null);
    }
  };

  const removeSelectedQuery = async (): Promise<void> => {
    if (!selectedQueryId) return;
    setBusy('deleting');
    setFeedback(null);
    try {
      await deleteQuery(selectedQueryId);
      const remainingQueries = queries.filter((query) => query.id !== selectedQueryId);
      setQueries(remainingQueries);
      resetDraft(remainingQueries[0] ?? null);
      setFeedback({ tone: 'success', message: 'Approved query deleted.' });
      void refresh();
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to delete query.' });
    } finally {
      setBusy(null);
    }
  };

  const runValidation = async (): Promise<void> => {
    setBusy('validating');
    setFeedback(null);
    try {
      const nextValidation = await validateQuery(draft.sql, draft.parameters);
      setValidation(nextValidation);
      setFeedback({
        tone: nextValidation.valid ? 'success' : 'danger',
        message: nextValidation.message ?? (nextValidation.valid ? 'SQL validated.' : 'SQL validation failed.'),
      });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Validation failed.' });
      setValidation({ valid: false, message: error instanceof Error ? error.message : 'Validation failed.' });
    } finally {
      setBusy(null);
    }
  };

  const runDraftTest = async (): Promise<void> => {
    setBusy('testing');
    setFeedback(null);
    try {
      const execution = await testQuery(draft.sql, draft.parameters);
      setResults(execution);
      setFeedback({ tone: 'success', message: 'Draft query executed successfully.' });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Test query failed.' });
    } finally {
      setBusy(null);
    }
  };

  const runSavedQuery = async (queryId: string): Promise<void> => {
    if (!selectedQuery || !canExecuteSavedQuery(selectedQuery, executeParameterValues)) {
      return;
    }

    setBusy('executing');
    setFeedback(null);
    try {
      const execution = await executeSavedQuery(queryId, executeParameterValues, 100);
      setResults(execution);
      setFeedback({ tone: 'success', message: 'Approved query executed.' });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Saved query execution failed.' });
    } finally {
      setBusy(null);
    }
  };

  if (!datasource && busy !== 'loading') {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Approved Queries"
          description="Create the small catalog of SQL prompts OpenClaw should be allowed to use after your datasource is configured."
        />
        <EmptyState
          title="Start by adding a datasource"
          body="DataClaw needs a datasource before it can validate or execute approved queries. Once connected, you can seed SELECT true AS connected or build a richer catalog."
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Approved Queries"
        description="Manage the exact SQL that OpenClaw is allowed to create, inspect, and run through the local MCP server. There are no pending-review queues in DataClaw v1—everything here is the approved set."
        actions={
          <Button type="button" variant="outline" onClick={() => resetDraft(null)}>
            <Plus className="h-4 w-4" />
            New query
          </Button>
        }
      />
      {feedback ? <StatusBanner tone={feedback.tone} message={feedback.message} /> : null}
      <div className="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <Card>
          <CardHeader>
            <CardTitle className="text-xl">Approved catalog</CardTitle>
            <CardDescription>
              {queries.length === 0 ? 'No approved queries yet.' : `${queries.length} approved ${queries.length === 1 ? 'query' : 'queries'}.`}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {queries.length === 0 ? (
              <EmptyState
                title="No approved queries yet"
                body="Seed a simple connectivity check now, or create a custom SQL query below."
                actions={
                  <Button
                    type="button"
                    onClick={() => {
                      setDraft({ ...EMPTY_QUERY, datasourceId: datasource?.id, name: 'Connectivity check', sql: QUERY_TEMPLATE });
                      setSelectedQueryId(null);
                    }}
                  >
                    <CheckCircle2 className="h-4 w-4" />
                    Use SELECT true AS connected
                  </Button>
                }
              />
            ) : (
              queries.map((query) => (
                <button
                  key={query.id}
                  className={`w-full rounded-2xl border px-4 py-4 text-left transition ${
                    selectedQueryId === query.id
                      ? 'border-slate-950 bg-slate-950 text-white'
                      : 'border-border-light bg-surface-primary hover:border-slate-400'
                  }`}
                  onClick={() => resetDraft(query)}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="font-medium">{query.name}</div>
                      <p className={`mt-1 text-sm ${selectedQueryId === query.id ? 'text-slate-300' : 'text-text-secondary'}`}>
                        {query.description || 'No description yet.'}
                      </p>
                    </div>
                    <span className={`rounded-full px-3 py-1 text-xs font-medium ${query.isEnabled ? 'bg-emerald-100 text-emerald-800' : 'bg-amber-100 text-amber-800'}`}>
                      {query.isEnabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </div>
                  <code className={`mt-3 block truncate rounded-lg px-3 py-2 text-xs ${selectedQueryId === query.id ? 'bg-slate-900 text-slate-200' : 'bg-surface-secondary text-text-secondary'}`}>
                    {query.sql}
                  </code>
                  <div className="mt-3 flex flex-wrap gap-2 text-xs">
                    <span className={selectedQueryId === query.id ? 'text-slate-300' : 'text-text-tertiary'}>
                      {query.parameters.length} parameters
                    </span>
                    <span className={selectedQueryId === query.id ? 'text-slate-300' : 'text-text-tertiary'}>
                      Updated {query.updatedAt ?? 'recently'}
                    </span>
                  </div>
                </button>
              ))
            )}
          </CardContent>
        </Card>
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-xl">{selectedQuery ? 'Edit approved query' : 'Create approved query'}</CardTitle>
              <CardDescription>
                Author SQL once, validate it, and keep the approved set lean. Schema autocomplete and pending-review flows are intentionally removed in DataClaw.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid gap-4 md:grid-cols-2">
                <div>
                  <Label htmlFor="query-name">Name</Label>
                  <Input id="query-name" className="mt-2" value={draft.name} onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))} placeholder="Connectivity check" />
                </div>
                <div>
                  <Label htmlFor="query-description">Description</Label>
                  <Input id="query-description" className="mt-2" value={draft.description ?? ''} onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))} placeholder="Explain when OpenClaw should use this query" />
                </div>
              </div>
              <label className="flex items-center gap-2 text-sm text-text-secondary">
                <input type="checkbox" checked={draft.isEnabled} onChange={(event) => setDraft((current) => ({ ...current, isEnabled: event.target.checked }))} />
                Enabled for OpenClaw
              </label>
              <div>
                <Label htmlFor="query-sql">SQL</Label>
                <div className="mt-2">
                  <SqlEditor
                    value={draft.sql}
                    onChange={(value) => setDraft((current) => ({ ...current, sql: value }))}
                    dialect={dialect}
                    validationStatus={busy === 'validating' ? 'validating' : validation ? (validation.valid ? 'valid' : 'invalid') : 'idle'}
                    validationError={validation?.valid === false ? validation.message : undefined}
                  />
                </div>
              </div>
              <ParameterEditor parameters={draft.parameters} onChange={(parameters) => setDraft((current) => ({ ...current, parameters }))} />
              {selectedQuery && selectedQuery.parameters.length > 0 ? (
                <ParameterInputForm parameters={selectedQuery.parameters} values={executeParameterValues} onChange={setExecuteParameterValues} />
              ) : null}
              <div className="flex flex-wrap gap-3">
                <Button type="button" onClick={() => void persistDraft()} disabled={busy !== null || !draft.name.trim() || !draft.sql.trim()}>
                  <Save className="h-4 w-4" />
                  {selectedQuery ? 'Save changes' : 'Create query'}
                </Button>
                <Button type="button" variant="outline" onClick={() => void runValidation()} disabled={busy !== null || !draft.sql.trim()}>
                  <Pencil className="h-4 w-4" />
                  Validate SQL
                </Button>
                <Button type="button" variant="outline" onClick={() => void runDraftTest()} disabled={busy !== null || !draft.sql.trim()}>
                  <FlaskConical className="h-4 w-4" />
                  Test draft query
                </Button>
                {selectedQuery ? (
                  <>
                    <Button type="button" variant="outline" onClick={() => void runSavedQuery(selectedQuery.id)} disabled={busy !== null || !canExecuteSelectedQuery}>
                      <Play className="h-4 w-4" />
                      Execute saved query
                    </Button>
                    <Button type="button" variant="destructive" onClick={() => void removeSelectedQuery()} disabled={busy !== null}>
                      <Trash2 className="h-4 w-4" />
                      Delete query
                    </Button>
                  </>
                ) : null}
              </div>
              {selectedQuery && !selectedQuery.isEnabled ? (
                <p className="text-sm text-text-secondary">Execute saved query is disabled because this approved query is currently disabled.</p>
              ) : null}
              {selectedQuery && selectedQuery.isEnabled && !canExecuteSelectedQuery ? (
                <p className="text-sm text-text-secondary">Provide values for the required execution parameters before running this approved query.</p>
              ) : null}
            </CardContent>
          </Card>
          {results ? <QueryResultsTable columns={results.columns} rows={results.rows} rowCount={results.rowCount} /> : null}
        </div>
      </div>
    </div>
  );
}
