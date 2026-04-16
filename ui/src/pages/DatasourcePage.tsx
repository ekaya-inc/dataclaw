import { DatabaseZap, RotateCcw, Save, TestTube2, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import { PROVIDERS } from '../constants';
import { EmptyState } from '../components/EmptyState';
import { PageHeader } from '../components/PageHeader';
import { StatusBanner } from '../components/StatusBanner';
import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { deleteDatasource, getDatasource, saveDatasource, testDatasource } from '../services/api';
import type { DatasourceFormValues, DatasourceRecord, SSLMode } from '../types/datasource';

const DEFAULT_VALUES: DatasourceFormValues = {
  type: 'postgres',
  provider: 'postgres',
  displayName: 'Primary datasource',
  host: '',
  port: '5432',
  database: '',
  username: '',
  password: '',
  sslMode: 'require',
  encrypt: true,
  trustServerCertificate: false,
};

function toFormValues(datasource: DatasourceRecord): DatasourceFormValues {
  return {
    type: datasource.type,
    provider: datasource.provider ?? datasource.type,
    displayName: datasource.displayName,
    host: datasource.host,
    port: String(datasource.port),
    database: datasource.database,
    username: datasource.username ?? '',
    password: datasource.password ?? '',
    sslMode: datasource.sslMode ?? 'require',
    encrypt: Boolean(datasource.options?.encrypt ?? true),
    trustServerCertificate: Boolean(datasource.options?.trust_server_certificate ?? false),
  };
}

export default function DatasourcePage(): JSX.Element {
  const [datasource, setDatasource] = useState<DatasourceRecord | null>(null);
  const [formValues, setFormValues] = useState<DatasourceFormValues>(DEFAULT_VALUES);
  const [busy, setBusy] = useState<'loading' | 'saving' | 'testing' | 'deleting' | null>('loading');
  const [feedback, setFeedback] = useState<{ tone: 'info' | 'success' | 'danger'; message: string } | null>(null);

  const providerOptions = useMemo(
    () => PROVIDERS.filter((provider) => provider.adapter === formValues.type),
    [formValues.type],
  );

  useEffect(() => {
    void (async () => {
      try {
        const currentDatasource = await getDatasource();
        setDatasource(currentDatasource);
        if (currentDatasource) {
          setFormValues(toFormValues(currentDatasource));
        }
      } catch (error) {
        setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to load datasource.' });
      } finally {
        setBusy(null);
      }
    })();
  }, []);

  useEffect(() => {
    const selectedProvider = providerOptions.find((provider) => provider.id === formValues.provider) ?? providerOptions[0];
    if (!selectedProvider) return;
    setFormValues((current) => ({
      ...current,
      provider: selectedProvider.id,
      port: current.port || selectedProvider.defaultPort,
      sslMode: current.sslMode || selectedProvider.defaultSSL,
    }));
  }, [providerOptions, formValues.provider]);

  const updateField = <K extends keyof DatasourceFormValues>(field: K, value: DatasourceFormValues[K]): void => {
    setFormValues((current) => ({ ...current, [field]: value }));
  };

  const handleProviderChange = (providerId: string): void => {
    const provider = PROVIDERS.find((item) => item.id === providerId);
    if (!provider) return;
    setFormValues((current) => ({
      ...current,
      type: provider.adapter,
      provider: provider.id,
      port: provider.defaultPort,
      sslMode: provider.defaultSSL,
    }));
  };

  const handleTest = async (): Promise<void> => {
    setBusy('testing');
    setFeedback(null);
    try {
      const result = await testDatasource(formValues);
      setFeedback({ tone: result.success ? 'success' : 'danger', message: result.message });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Connection test failed.' });
    } finally {
      setBusy(null);
    }
  };

  const handleSave = async (): Promise<void> => {
    setBusy('saving');
    setFeedback(null);
    try {
      const savedDatasource = await saveDatasource(formValues);
      setDatasource(savedDatasource);
      setFormValues(toFormValues(savedDatasource));
      setFeedback({ tone: 'success', message: 'Datasource saved. DataClaw will use this datasource for raw queries and approved queries.' });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to save datasource.' });
    } finally {
      setBusy(null);
    }
  };

  const handleDelete = async (): Promise<void> => {
    setBusy('deleting');
    setFeedback(null);
    try {
      await deleteDatasource();
      setDatasource(null);
      setFormValues(DEFAULT_VALUES);
      setFeedback({ tone: 'success', message: 'Datasource removed.' });
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to delete datasource.' });
    } finally {
      setBusy(null);
    }
  };

  const providerHelper = PROVIDERS.find((provider) => provider.id === formValues.provider)?.helperText;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Datasource"
        description="Configure the one datasource that OpenClaw will reach through DataClaw. As soon as it is saved, the backend can use it for raw queries and approved-query execution."
        actions={
          datasource ? (
            <Button type="button" variant="outline" onClick={() => setFormValues(toFormValues(datasource))}>
              <RotateCcw className="h-4 w-4" />
              Reset form
            </Button>
          ) : undefined
        }
      />
      {feedback ? <StatusBanner tone={feedback.tone} message={feedback.message} /> : null}
      {!datasource && busy !== 'loading' ? (
        <EmptyState title="No datasource configured yet" body="Choose a PostgreSQL-compatible service or SQL Server, test the connection, then save it. DataClaw keeps exactly one datasource in v1." />
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-xl">
            <DatabaseZap className="h-5 w-5" />
            Datasource details
          </CardTitle>
          <CardDescription>
            {datasource
              ? 'Update or replace the current datasource. Saving again overwrites the single configured datasource.'
              : 'Supported today: PostgreSQL flavors and Microsoft SQL Server.'}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <div>
              <Label htmlFor="provider">Database</Label>
              <select
                id="provider"
                className="mt-2 flex h-10 w-full rounded-lg border border-border-medium bg-surface-primary px-3 py-2 text-sm text-text-primary"
                value={formValues.provider}
                onChange={(event) => handleProviderChange(event.target.value)}
              >
                {PROVIDERS.map((provider) => (
                  <option key={provider.id} value={provider.id}>
                    {provider.label}
                  </option>
                ))}
              </select>
              {providerHelper ? <p className="mt-2 text-xs text-text-tertiary">{providerHelper}</p> : null}
            </div>
            <div>
              <Label htmlFor="display-name">Display name</Label>
              <Input id="display-name" className="mt-2" value={formValues.displayName} onChange={(event) => updateField('displayName', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="database">Database</Label>
              <Input id="database" className="mt-2" value={formValues.database} onChange={(event) => updateField('database', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="host">Host</Label>
              <Input id="host" className="mt-2" value={formValues.host} onChange={(event) => updateField('host', event.target.value)} placeholder="db.example.com" />
            </div>
            <div>
              <Label htmlFor="port">Port</Label>
              <Input id="port" className="mt-2" value={formValues.port} onChange={(event) => updateField('port', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="username">Username</Label>
              <Input id="username" className="mt-2" value={formValues.username} onChange={(event) => updateField('username', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="password">Password</Label>
              <Input id="password" className="mt-2" type="password" value={formValues.password} onChange={(event) => updateField('password', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="ssl-mode">SSL mode</Label>
              <select
                id="ssl-mode"
                className="mt-2 flex h-10 w-full rounded-lg border border-border-medium bg-surface-primary px-3 py-2 text-sm text-text-primary"
                value={formValues.sslMode}
                onChange={(event) => updateField('sslMode', event.target.value as SSLMode)}
              >
                {['disable', 'allow', 'prefer', 'require', 'verify-ca', 'verify-full'].map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </select>
            </div>
            {formValues.type === 'mssql' ? (
              <>
                <label className="mt-8 flex items-center gap-2 text-sm text-text-secondary">
                  <input type="checkbox" checked={formValues.encrypt} onChange={(event) => updateField('encrypt', event.target.checked)} />
                  Encrypt connection
                </label>
                <label className="mt-8 flex items-center gap-2 text-sm text-text-secondary">
                  <input
                    type="checkbox"
                    checked={formValues.trustServerCertificate}
                    onChange={(event) => updateField('trustServerCertificate', event.target.checked)}
                  />
                  Trust server certificate
                </label>
              </>
            ) : null}
          </div>
          <div className="flex flex-wrap gap-3">
            <Button type="button" onClick={() => void handleSave()} disabled={busy !== null}>
              <Save className="h-4 w-4" />
              {datasource ? 'Save changes' : 'Save datasource'}
            </Button>
            <Button type="button" variant="outline" onClick={() => void handleTest()} disabled={busy !== null}>
              <TestTube2 className="h-4 w-4" />
              Test connection
            </Button>
            {datasource ? (
              <Button type="button" variant="destructive" onClick={() => void handleDelete()} disabled={busy !== null}>
                <Trash2 className="h-4 w-4" />
                Remove datasource
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
