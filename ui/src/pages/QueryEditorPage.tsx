import { AlertTriangle, ArrowLeft, FlaskConical, Save } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext, useParams } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { OutputColumnEditor } from '../components/OutputColumnEditor';
import { ParameterEditor } from '../components/ParameterEditor';
import { SqlEditor } from '../components/SqlEditor';
import { PageHeader } from '../components/PageHeader';
import { QueryResultsTable } from '../components/QueryResultsTable';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { useToast } from '../components/ui/Toast';
import { QUERY_TEMPLATE } from '../constants';
import { useSqlValidation } from '../hooks/useSqlValidation';
import { createQuery, getDatasource, getQuery, testQuery, updateQuery } from '../services/api';
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

type Mode = 'create' | 'edit';

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

export default function QueryEditorPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const mode: Mode = id ? 'edit' : 'create';
  const navigate = useNavigate();
  const { refresh } = useOutletContext<AppOutletContext>();
  const { toast } = useToast();

  const [datasource, setDatasource] = useState<DatasourceRecord | null>(null);
  const [draft, setDraft] = useState<DraftState>(emptyDraft);
  const [loading, setLoading] = useState(mode === 'edit');
  const [busy, setBusy] = useState<'loading' | 'saving' | 'testing' | null>(mode === 'edit' ? 'loading' : null);
  const [results, setResults] = useState<QueryExecutionResult | null>(null);

  const validation = useSqlValidation({
    sql: draft.sql,
    parameters: draft.parameters,
    allowsModification: draft.allowsModification,
  });

  useEffect(() => {
    let cancelled = false;

    void (async () => {
      setResults(null);
      if (mode === 'edit') {
        setLoading(true);
        setBusy('loading');
      }

      try {
        const [currentDatasource, query] = await Promise.all([
          getDatasource(),
          mode === 'edit' && id ? getQuery(id) : Promise.resolve(null),
        ]);
        if (cancelled) return;

        setDatasource(currentDatasource);
        setDraft(query ? draftFromQuery(query) : emptyDraft());
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
          setBusy(null);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [id, mode, navigate, toast]);

  const dialect = datasource?.sqlDialect ?? DEFAULT_SQL_DIALECT;

  const updateDraft = <K extends keyof DraftState>(key: K, value: DraftState[K]): void => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const handleSubmit = async (): Promise<void> => {
    if (!draft.naturalLanguagePrompt.trim() || !draft.sql.trim()) {
      toast({ variant: 'error', title: 'Add a prompt and SQL before saving' });
      return;
    }

    setBusy('saving');
    try {
      const payload = { ...draft, datasourceId: datasource?.id };
      const saved = mode === 'edit' && id ? await updateQuery(id, payload) : await createQuery(payload);
      toast({
        variant: 'success',
        title: mode === 'edit' ? 'Approved query updated' : 'Approved query created',
      });
      await refresh();
      navigate(`/queries/${saved.id}`, { replace: mode === 'create' });
    } catch (error) {
      toast({
        variant: 'error',
        title: mode === 'edit' ? 'Failed to update query' : 'Failed to create query',
        description: error instanceof Error ? error.message : undefined,
      });
    } finally {
      setBusy(null);
    }
  };

  const runDraftTest = async (): Promise<void> => {
    if (!draft.sql.trim()) {
      toast({ variant: 'error', title: 'Add SQL before testing' });
      return;
    }

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

  const handleCancel = (): void => {
    if (mode === 'edit' && id) {
      navigate(`/queries/${id}`);
      return;
    }
    navigate('/queries');
  };

  const submitLabel = mode === 'create' ? (busy === 'saving' ? 'Creating…' : 'Create query') : busy === 'saving' ? 'Saving…' : 'Save changes';
  const canSubmit = !loading && busy === null && draft.naturalLanguagePrompt.trim() !== '' && draft.sql.trim() !== '';

  return (
    <div className="space-y-6">
      <div>
        <Button variant="ghost" size="sm" onClick={handleCancel}>
          <ArrowLeft className="h-4 w-4" />
          {mode === 'edit' ? 'Back to query' : 'Back to approved queries'}
        </Button>
      </div>

      <PageHeader
        title={mode === 'create' ? 'Create approved query' : 'Edit approved query'}
        description={
          mode === 'create'
            ? 'Author SQL once, validate it, and add it to the approved catalog Agents can use.'
            : 'Update the saved SQL, metadata, and output shape for this approved query.'
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">Query details</CardTitle>
          <CardDescription>
            Draft testing and saving happen here. Saved-query execution and deletion stay on the read-only detail page.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {loading ? (
            <div className="rounded-xl border border-dashed border-border-light bg-surface-secondary/60 px-4 py-6 text-sm text-text-secondary">
              Loading approved query…
            </div>
          ) : (
            <>
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

              <div className="flex flex-wrap items-center justify-end gap-3 border-t border-border-light pt-4">
                <Button type="button" variant="outline" onClick={handleCancel} disabled={busy !== null}>
                  Cancel
                </Button>
                <Button type="button" variant="outline" onClick={() => void runDraftTest()} disabled={loading || busy !== null || !draft.sql.trim()}>
                  <FlaskConical className="h-4 w-4" />
                  Test draft query
                </Button>
                <Button type="button" onClick={() => void handleSubmit()} disabled={!canSubmit}>
                  <Save className="h-4 w-4" />
                  {submitLabel}
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {results ? <QueryResultsTable columns={results.columns} rows={results.rows} rowCount={results.rowCount} /> : null}
    </div>
  );
}
