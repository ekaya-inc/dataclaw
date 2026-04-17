import { AlertTriangle, ArrowLeft, FlaskConical, Play, Save, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useOutletContext, useParams } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { DeleteQueryDialog } from '../components/DeleteQueryDialog';
import { OutputColumnEditor } from '../components/OutputColumnEditor';
import { ParameterEditor } from '../components/ParameterEditor';
import { ParameterInputForm } from '../components/ParameterInputForm';
import { QueryResultsTable } from '../components/QueryResultsTable';
import { SqlEditor } from '../components/SqlEditor';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { useToast } from '../components/ui/Toast';
import { QUERY_TEMPLATE } from '../constants';
import { useSqlValidation } from '../hooks/useSqlValidation';
import { createQuery, deleteQuery, executeSavedQuery, getDatasource, listQueries, testQuery, updateQuery } from '../services/api';
import type { DatasourceRecord } from '../types/datasource';
import { DEFAULT_SQL_DIALECT } from '../types/query';
import type { OutputColumn, QueryExecutionResult, QueryParameter, SavedQuery } from '../types/query';

interface DraftState {
  naturalLanguagePrompt: string;
  additionalContext: string;
  sql: string;
  allowsModification: boolean;
  parameters: QueryParameter[];
  outputColumns: OutputColumn[];
  constraints: string;
}

function emptyDraft(): DraftState {
  return {
    naturalLanguagePrompt: '',
    additionalContext: '',
    sql: QUERY_TEMPLATE,
    allowsModification: false,
    parameters: [],
    outputColumns: [],
    constraints: '',
  };
}

function draftFromQuery(query: SavedQuery): DraftState {
  return {
    naturalLanguagePrompt: query.naturalLanguagePrompt,
    additionalContext: query.additionalContext,
    sql: query.sql,
    allowsModification: query.allowsModification,
    parameters: query.parameters,
    outputColumns: query.outputColumns,
    constraints: query.constraints,
  };
}

function hasRequiredExecutionValues(parameters: QueryParameter[], values: Record<string, unknown>): boolean {
  return parameters.every((parameter) => {
    if (!parameter.required) {
      return true;
    }
    const provided = Object.prototype.hasOwnProperty.call(values, parameter.name);
    if (provided) {
      const value = values[parameter.name];
      if (value === null || value === undefined) return false;
      if (typeof value === 'string') return value.trim() !== '';
      if (Array.isArray(value)) return value.length > 0;
      return true;
    }
    return parameter.default !== null && parameter.default !== undefined;
  });
}

export default function QueryEditorPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { refresh } = useOutletContext<AppOutletContext>();
  const { toast } = useToast();

  const [datasource, setDatasource] = useState<DatasourceRecord | null>(null);
  const [savedQuery, setSavedQuery] = useState<SavedQuery | null>(null);
  const [draft, setDraft] = useState<DraftState>(emptyDraft);
  const [executeParameterValues, setExecuteParameterValues] = useState<Record<string, unknown>>({});
  const [busy, setBusy] = useState<'loading' | 'saving' | 'testing' | 'executing' | 'deleting' | null>('loading');
  const [results, setResults] = useState<QueryExecutionResult | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const validation = useSqlValidation({
    sql: draft.sql,
    parameters: draft.parameters,
    allowsModification: draft.allowsModification,
  });

  useEffect(() => {
    void (async () => {
      setBusy('loading');
      try {
        const [currentDatasource, queries] = await Promise.all([getDatasource(), listQueries()]);
        setDatasource(currentDatasource);
        if (id) {
          const match = queries.find((query) => query.id === id);
          if (!match) {
            toast({ variant: 'error', title: 'Query not found' });
            navigate('/queries');
            return;
          }
          setSavedQuery(match);
          setDraft(draftFromQuery(match));
        } else {
          setSavedQuery(null);
          setDraft(emptyDraft());
        }
      } catch (error) {
        toast({
          variant: 'error',
          title: 'Failed to load approved query',
          description: error instanceof Error ? error.message : undefined,
        });
      } finally {
        setBusy(null);
      }
    })();
  }, [id, navigate, toast]);

  const dialect = datasource?.sqlDialect ?? DEFAULT_SQL_DIALECT;

  const canExecuteSavedQuery = useMemo(() => {
    if (!savedQuery) return false;
    return hasRequiredExecutionValues(savedQuery.parameters, executeParameterValues);
  }, [savedQuery, executeParameterValues]);

  const updateDraft = <K extends keyof DraftState>(key: K, value: DraftState[K]): void => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const persistDraft = async (): Promise<void> => {
    if (!draft.naturalLanguagePrompt.trim() || !draft.sql.trim()) {
      toast({ variant: 'error', title: 'Add a prompt and SQL before saving' });
      return;
    }
    setBusy('saving');
    try {
      const payload = { ...draft, datasourceId: datasource?.id };
      const saved = savedQuery ? await updateQuery(savedQuery.id, payload) : await createQuery(payload);
      setSavedQuery(saved);
      setDraft(draftFromQuery(saved));
      toast({
        variant: 'success',
        title: savedQuery ? 'Approved query updated' : 'Approved query created',
      });
      void refresh();
      if (!savedQuery) {
        navigate(`/queries/${saved.id}`, { replace: true });
      }
    } catch (error) {
      toast({
        variant: 'error',
        title: savedQuery ? 'Failed to update query' : 'Failed to create query',
        description: error instanceof Error ? error.message : undefined,
      });
    } finally {
      setBusy(null);
    }
  };

  const runDraftTest = async (): Promise<void> => {
    setBusy('testing');
    try {
      const execution = await testQuery(draft.sql, draft.parameters, draft.allowsModification);
      setResults(execution);
      toast({ variant: 'success', title: 'Draft query executed' });
    } catch (error) {
      toast({
        variant: 'error',
        title: 'Draft test failed',
        description: error instanceof Error ? error.message : undefined,
      });
    } finally {
      setBusy(null);
    }
  };

  const runSavedQuery = async (): Promise<void> => {
    if (!savedQuery || !canExecuteSavedQuery) return;
    setBusy('executing');
    try {
      const execution = await executeSavedQuery(savedQuery.id, executeParameterValues, 100);
      setResults(execution);
      toast({ variant: 'success', title: 'Approved query executed' });
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

  const removeSelectedQuery = async (): Promise<void> => {
    if (!savedQuery) return;
    setBusy('deleting');
    try {
      await deleteQuery(savedQuery.id);
      toast({ variant: 'success', title: 'Approved query deleted' });
      void refresh();
      navigate('/queries');
    } catch (error) {
      toast({
        variant: 'error',
        title: 'Failed to delete query',
        description: error instanceof Error ? error.message : undefined,
      });
      setBusy(null);
      setDeleteDialogOpen(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <Button variant="ghost" size="sm" onClick={() => navigate('/queries')}>
          <ArrowLeft className="h-4 w-4" />
          Back to approved queries
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">{savedQuery ? 'Edit approved query' : 'Create approved query'}</CardTitle>
          <CardDescription>
            Author SQL once, validate it, and keep the approved set lean. Agents will only be able to run what lives
            here.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="grid gap-4 lg:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="query-prompt">Natural language prompt *</Label>
              <Input
                id="query-prompt"
                value={draft.naturalLanguagePrompt}
                onChange={(event) => updateDraft('naturalLanguagePrompt', event.target.value)}
                placeholder="e.g. Recent orders for a given customer"
              />
              <p className="text-xs text-text-tertiary">Plain-language trigger Agents will match against.</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="query-additional-context">Additional context</Label>
              <Input
                id="query-additional-context"
                value={draft.additionalContext}
                onChange={(event) => updateDraft('additionalContext', event.target.value)}
                placeholder="Hints for the LLM: when to use this query, edge cases, etc."
              />
            </div>
          </div>

          <div className="flex items-start gap-3 rounded-xl border border-border-light bg-surface-secondary p-4">
            <input
              id="query-allows-modification"
              type="checkbox"
              className="mt-1"
              checked={draft.allowsModification}
              onChange={(event) => updateDraft('allowsModification', event.target.checked)}
            />
            <div>
              <label htmlFor="query-allows-modification" className="text-sm font-medium text-text-primary">
                Allow this query to modify data
              </label>
              <p className="text-xs text-text-secondary">
                Permits <code>INSERT</code>, <code>UPDATE</code>, and <code>DELETE</code>. DDL is always blocked. Use
                <code className="mx-1">RETURNING</code> (PostgreSQL) or <code className="mx-1">OUTPUT</code> (SQL
                Server) if you need rows back.
              </p>
              {draft.allowsModification ? (
                <p className="mt-2 inline-flex items-center gap-2 rounded-md bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800">
                  <AlertTriangle className="h-3 w-3" />
                  This query can mutate rows.
                </p>
              ) : null}
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="query-sql">SQL *</Label>
            <SqlEditor
              value={draft.sql}
              onChange={(value) => updateDraft('sql', value)}
              dialect={dialect}
              validationStatus={validation.status}
              validationError={validation.error}
            />
          </div>

          <ParameterEditor
            parameters={draft.parameters}
            onChange={(parameters) => updateDraft('parameters', parameters)}
          />

          <OutputColumnEditor
            outputColumns={draft.outputColumns}
            onChange={(outputColumns) => updateDraft('outputColumns', outputColumns)}
          />

          <div className="space-y-2">
            <Label htmlFor="query-constraints">Constraints</Label>
            <textarea
              id="query-constraints"
              className="min-h-[100px] w-full rounded-xl border border-border-medium bg-surface-primary px-3 py-2 text-sm text-text-primary"
              value={draft.constraints}
              onChange={(event) => updateDraft('constraints', event.target.value)}
              placeholder="Rules Agents must respect when using this query. For example: never request more than 30 days of data."
            />
          </div>

          {savedQuery && savedQuery.parameters.length > 0 ? (
            <ParameterInputForm
              parameters={savedQuery.parameters}
              values={executeParameterValues}
              onChange={setExecuteParameterValues}
            />
          ) : null}

          <div className="flex flex-wrap gap-3">
            <Button
              type="button"
              onClick={() => void persistDraft()}
              disabled={busy !== null || !draft.naturalLanguagePrompt.trim() || !draft.sql.trim()}
            >
              <Save className="h-4 w-4" />
              {savedQuery ? 'Save changes' : 'Create query'}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => void runDraftTest()}
              disabled={busy !== null || !draft.sql.trim()}
            >
              <FlaskConical className="h-4 w-4" />
              Test draft query
            </Button>
            {savedQuery ? (
              <>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void runSavedQuery()}
                  disabled={busy !== null || !canExecuteSavedQuery}
                >
                  <Play className="h-4 w-4" />
                  Execute saved query
                </Button>
                <Button
                  type="button"
                  variant="destructive"
                  onClick={() => setDeleteDialogOpen(true)}
                  disabled={busy !== null}
                >
                  <Trash2 className="h-4 w-4" />
                  Delete query
                </Button>
              </>
            ) : null}
          </div>

          {savedQuery && !canExecuteSavedQuery && savedQuery.parameters.length > 0 ? (
            <p className="text-sm text-text-secondary">
              Provide values for the required execution parameters before running this approved query.
            </p>
          ) : null}
        </CardContent>
      </Card>

      {results ? <QueryResultsTable columns={results.columns} rows={results.rows} rowCount={results.rowCount} /> : null}

      <DeleteQueryDialog
        open={deleteDialogOpen}
        queryPrompt={savedQuery?.naturalLanguagePrompt ?? ''}
        deleting={busy === 'deleting'}
        onCancel={() => setDeleteDialogOpen(false)}
        onConfirm={() => void removeSelectedQuery()}
      />
    </div>
  );
}
