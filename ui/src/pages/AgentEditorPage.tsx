import { ArrowLeft } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext, useParams } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import {
  AgentFormFields,
  EMPTY_AGENT_FORM,
  agentFormFromRecord,
  isAgentFormSubmittable,
} from '../components/AgentFormFields';
import { PageHeader } from '../components/PageHeader';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { useToast } from '../components/ui/Toast';
import { createAgent, getAgent, listQueries, updateAgent } from '../services/api';
import type { AgentFormValues } from '../types/agent';
import type { SavedQuery } from '../types/query';

type Mode = 'create' | 'edit';

export default function AgentEditorPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const mode: Mode = id ? 'edit' : 'create';
  const { refresh } = useOutletContext<AppOutletContext>();
  const { toast } = useToast();
  const navigate = useNavigate();

  const [form, setForm] = useState<AgentFormValues>(EMPTY_AGENT_FORM);
  const [queries, setQueries] = useState<SavedQuery[]>([]);
  const [loading, setLoading] = useState(mode === 'edit');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const [loadedQueries, loadedAgent] = await Promise.all([
          listQueries().catch(() => [] as SavedQuery[]),
          mode === 'edit' && id ? getAgent(id) : Promise.resolve(null),
        ]);
        if (cancelled) return;
        setQueries(loadedQueries);
        if (loadedAgent) setForm(agentFormFromRecord(loadedAgent));
      } catch (error) {
        if (cancelled) return;
        toast({
          title: 'Failed to load',
          description: error instanceof Error ? error.message : 'Failed to load access point.',
          variant: 'error',
        });
        navigate('/agents');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id, mode, navigate, toast]);

  const submitLabel = mode === 'create' ? (busy ? 'Creating…' : 'Create access point') : busy ? 'Saving…' : 'Save changes';
  const canSubmit = isAgentFormSubmittable(form) && !busy && !loading;

  const handleSubmit = async (): Promise<void> => {
    if (!canSubmit) return;
    setBusy(true);
    try {
      if (mode === 'create') {
        const created = await createAgent({ ...form, name: form.name.trim() });
        await Promise.all([refresh(), Promise.resolve()]);
        toast({
          title: 'Access point created',
          description: `${created.name} is ready — copy the API key before leaving this page.`,
          variant: 'success',
        });
        navigate(`/agents/${created.id}`, { state: { apiKey: created.apiKey ?? null } });
        return;
      }
      if (!id) return;
      const saved = await updateAgent(id, form);
      await refresh();
      toast({ title: 'Access point updated', description: `${saved.name} access updated.`, variant: 'success' });
      navigate(`/agents/${saved.id}`);
    } catch (error) {
      toast({
        title: mode === 'create' ? 'Create failed' : 'Update failed',
        description: error instanceof Error ? error.message : 'Failed to save access point.',
        variant: 'error',
      });
    } finally {
      setBusy(false);
    }
  };

  const handleCancel = (): void => {
    if (mode === 'edit' && id) {
      navigate(`/agents/${id}`);
    } else {
      navigate('/agents');
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <Button variant="ghost" size="sm" onClick={handleCancel}>
          <ArrowLeft className="h-4 w-4" />
          {mode === 'edit' ? 'Back to access point' : 'Back to Agent Access'}
        </Button>
      </div>

      <PageHeader
        title={mode === 'create' ? 'New Access Point' : 'Edit Access Point'}
        description={
          mode === 'create'
            ? 'Create a named access point, choose its tool permissions, and scope its approved-query access.'
            : 'Update tool permissions and approved-query access for this access point.'
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-xl">Access point details</CardTitle>
          <CardDescription>
            {mode === 'create'
              ? 'The name becomes the MCP server key. Tool permissions and approved-query scope are enforced on every request.'
              : 'Name is immutable. Changing permissions takes effect immediately for this access point but MCP clients will need to be restarted.'}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {loading ? (
            <div className="rounded-xl border border-dashed border-border-light bg-surface-secondary/60 px-4 py-6 text-sm text-text-secondary">
              Loading…
            </div>
          ) : (
            <AgentFormFields form={form} onChange={setForm} queries={queries} nameReadOnly={mode === 'edit'} />
          )}

          <div className="flex flex-wrap items-center justify-end gap-3 border-t border-border-light pt-4">
            <Button type="button" variant="outline" onClick={handleCancel} disabled={busy}>
              Cancel
            </Button>
            <Button type="button" onClick={() => void handleSubmit()} disabled={!canSubmit}>
              {submitLabel}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
