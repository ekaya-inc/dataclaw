import { Plus, Search, Sparkles } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { EmptyState } from '../components/EmptyState';
import { PageHeader } from '../components/PageHeader';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { useToast } from '../components/ui/Toast';
import { QUERY_TEMPLATE } from '../constants';
import { createQuery, getDatasource, listQueries } from '../services/api';
import type { DatasourceRecord } from '../types/datasource';
import type { SavedQuery } from '../types/query';

export default function ApprovedQueriesPage(): JSX.Element {
  const navigate = useNavigate();
  const { refresh } = useOutletContext<AppOutletContext>();
  const { toast } = useToast();
  const [datasource, setDatasource] = useState<DatasourceRecord | null>(null);
  const [queries, setQueries] = useState<SavedQuery[]>([]);
  const [loading, setLoading] = useState(true);
  const [seeding, setSeeding] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');

  useEffect(() => {
    void (async () => {
      try {
        const [currentDatasource, currentQueries] = await Promise.all([getDatasource(), listQueries()]);
        setDatasource(currentDatasource);
        setQueries(currentQueries);
      } catch (error) {
        toast({
          variant: 'error',
          title: 'Failed to load approved queries',
          description: error instanceof Error ? error.message : undefined,
        });
      } finally {
        setLoading(false);
      }
    })();
  }, [toast]);

  const filteredQueries = useMemo(() => {
    const needle = searchTerm.trim().toLowerCase();
    if (!needle) return queries;
    return queries.filter(
      (query) =>
        query.naturalLanguagePrompt.toLowerCase().includes(needle) ||
        query.additionalContext.toLowerCase().includes(needle) ||
        query.sql.toLowerCase().includes(needle),
    );
  }, [queries, searchTerm]);

  const handleSeedQuery = async (): Promise<void> => {
    setSeeding(true);
    try {
      await createQuery({
        datasourceId: datasource?.id,
        naturalLanguagePrompt: 'Connectivity check',
        additionalContext: 'Verify the datasource is reachable by returning a simple boolean.',
        sql: QUERY_TEMPLATE,
        allowsModification: false,
        parameters: [],
        outputColumns: [
          { name: 'connected', type: 'boolean', description: 'True when the datasource responds.' },
        ],
        constraints: '',
      });
      toast({ variant: 'success', title: 'Seeded example query' });
      const [latest] = await Promise.all([listQueries(), refresh()]);
      setQueries(latest);
    } catch (error) {
      toast({
        variant: 'error',
        title: 'Failed to seed example query',
        description: error instanceof Error ? error.message : undefined,
      });
    } finally {
      setSeeding(false);
    }
  };

  if (!datasource && !loading) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Approved Queries"
          description="Create the catalog of SQL that Agents are allowed to run after your datasource is configured."
        />
        <EmptyState
          title="Start by adding a datasource"
          body="DataClaw needs a datasource before it can validate or execute approved queries. Once connected, you can seed a connectivity check or build a richer catalog."
          actions={<Button onClick={() => navigate('/datasource')}>Configure datasource</Button>}
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Approved Queries"
        description="Manage the SQL Queries that Agents are allowed to run."
        actions={
          <Button type="button" onClick={() => navigate('/queries/new')}>
            <Plus className="h-4 w-4" />
            New query
          </Button>
        }
      />

      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-tertiary" />
        <Input
          className="pl-10"
          placeholder="Search prompts, context, or SQL…"
          value={searchTerm}
          onChange={(event) => setSearchTerm(event.target.value)}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">Approved catalog</CardTitle>
          <CardDescription>
            {loading
              ? 'Loading approved queries…'
              : queries.length === 0
                ? 'No approved queries yet.'
                : `${queries.length} approved ${queries.length === 1 ? 'query' : 'queries'}.`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? null : queries.length === 0 ? (
            <EmptyState
              title="No approved queries yet"
              body="Create your own query by hand or (RECOMMENDED) have an agent create your queries for you by enabling 'Manage Approved Queries' in the Agents screen."
              actions={
                <Button type="button" onClick={() => void handleSeedQuery()} disabled={seeding}>
                  <Sparkles className="h-4 w-4" />
                  {seeding ? 'Seeding…' : 'Seed with an Example Query'}
                </Button>
              }
            />
          ) : filteredQueries.length === 0 ? (
            <EmptyState title="No matches" body="Try a different search term to find an approved query." />
          ) : (
            <ul className="grid gap-3">
              {filteredQueries.map((query) => (
                <li key={query.id}>
                  <button
                    type="button"
                    onClick={() => navigate(`/queries/${query.id}`)}
                    className="flex h-full w-full flex-col items-start gap-3 rounded-2xl border border-border-light bg-surface-primary p-4 text-left transition hover:border-slate-400 hover:bg-surface-hover"
                  >
                    <div className="flex w-full items-start justify-between gap-4">
                      <div className="min-w-0">
                        <div className="truncate text-base font-semibold text-text-primary">
                          {query.naturalLanguagePrompt || 'Untitled query'}
                        </div>
                        {query.additionalContext ? (
                          <p className="mt-1 line-clamp-2 text-sm text-text-secondary">{query.additionalContext}</p>
                        ) : null}
                      </div>
                      <div className="flex shrink-0 flex-col items-end gap-1.5 text-xs text-text-tertiary">
                        {query.allowsModification ? (
                          <span className="rounded-full bg-amber-100 px-2 py-1 text-xs font-medium text-amber-800">
                            Mutating
                          </span>
                        ) : null}
                        <div className="flex flex-wrap justify-end gap-x-3 gap-y-1">
                          <span>
                            {query.parameters.length} {query.parameters.length === 1 ? 'parameter' : 'parameters'}
                          </span>
                          <span>
                            {query.outputColumns.length} {query.outputColumns.length === 1 ? 'output column' : 'output columns'}
                          </span>
                          <span>Updated {query.updatedAt ?? 'recently'}</span>
                        </div>
                      </div>
                    </div>
                    <code className="block w-full truncate rounded-lg bg-surface-secondary px-3 py-2 text-xs text-text-secondary">
                      {query.sql}
                    </code>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
