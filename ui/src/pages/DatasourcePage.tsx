import { Pencil, Save, TestTube2, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useOutletContext } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { PROVIDERS } from '../constants';
import { PageHeader } from '../components/PageHeader';
import { StatusBanner } from '../components/StatusBanner';
import { Button } from '../components/ui/Button';
import { Card, CardContent } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { deleteDatasource, getDatasource, saveDatasource, testDatasource } from '../services/api';
import type { DatasourceFormValues, DatasourceRecord, SSLMode } from '../types/datasource';

const DEFAULT_VALUES: DatasourceFormValues = {
  type: 'postgres',
  provider: 'postgres',
  displayName: 'dataclaw',
  host: '',
  port: '5432',
  database: '',
  username: '',
  password: '',
  sslMode: 'require',
  encrypt: true,
  trustServerCertificate: false,
};

const CONNECTION_FIELDS: ReadonlySet<keyof DatasourceFormValues> = new Set([
  'type',
  'provider',
  'host',
  'port',
  'database',
  'username',
  'password',
  'sslMode',
  'encrypt',
  'trustServerCertificate',
]);

function FieldLabel({ htmlFor, children, locked = false }: { htmlFor: string; children: string; locked?: boolean }): JSX.Element {
  return (
    <div className="flex items-center gap-2">
      <Label htmlFor={htmlFor}>{children}</Label>
      {locked ? <span className="rounded-full bg-surface-secondary px-2 py-0.5 text-[11px] font-medium uppercase tracking-[0.08em] text-text-tertiary">Read-only</span> : null}
    </div>
  );
}

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
  const { refresh } = useOutletContext<AppOutletContext>();
  const [datasource, setDatasource] = useState<DatasourceRecord | null>(null);
  const [formValues, setFormValues] = useState<DatasourceFormValues>(DEFAULT_VALUES);
  const [busy, setBusy] = useState<'loading' | 'saving' | 'testing' | 'deleting' | null>('loading');
  const [feedback, setFeedback] = useState<{ tone: 'info' | 'success' | 'danger'; message: string } | null>(null);
  const [isEditingName, setIsEditingName] = useState(false);
  const [testPassed, setTestPassed] = useState(false);
  const connectionLocked = datasource !== null;
  const nameInputRef = useRef<HTMLInputElement>(null);

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
          setIsEditingName(false);
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

  useEffect(() => {
    if (!isEditingName) return;
    nameInputRef.current?.focus();
    nameInputRef.current?.select();
  }, [isEditingName]);

  const updateField = <K extends keyof DatasourceFormValues>(field: K, value: DatasourceFormValues[K]): void => {
    if (CONNECTION_FIELDS.has(field)) {
      setTestPassed(false);
    }
    setFormValues((current) => ({ ...current, [field]: value }));
  };

  const handleProviderChange = (providerId: string): void => {
    const provider = PROVIDERS.find((item) => item.id === providerId);
    if (!provider) return;
    setTestPassed(false);
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
      setTestPassed(result.success);
      setFeedback({ tone: result.success ? 'success' : 'danger', message: result.message });
    } catch (error) {
      setTestPassed(false);
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
      setIsEditingName(false);
      setFeedback({
        tone: 'success',
        message: 'Datasource saved. DataClaw will use this datasource for raw queries and approved queries.',
      });
      void refresh();
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to save datasource.' });
    } finally {
      setBusy(null);
    }
  };

  const commitDisplayNameIfChanged = async (): Promise<void> => {
    if (!datasource) return;
    if (busy !== null) return;
    if (formValues.displayName === datasource.displayName) return;
    setBusy('saving');
    setFeedback(null);
    try {
      const savedDatasource = await saveDatasource(formValues);
      setDatasource(savedDatasource);
      setFormValues(toFormValues(savedDatasource));
      setFeedback({ tone: 'success', message: 'Display name updated.' });
      void refresh();
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to update display name.' });
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
      setIsEditingName(false);
      setTestPassed(false);
      setFeedback({ tone: 'success', message: 'Datasource removed.' });
      void refresh();
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to delete datasource.' });
    } finally {
      setBusy(null);
    }
  };

  const providerHelper = PROVIDERS.find((provider) => provider.id === formValues.provider)?.helperText;
  const saveGated = !connectionLocked && !testPassed;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Datasource"
        description="Configure the datasource. The agent will be operating with the credentials you supplied below so limit their access appropriately."
      />
      <Card>
        <CardContent className="space-y-6 pt-6">
          <div className="border-b border-border-light pb-6">
            {isEditingName ? (
              <Input
                ref={nameInputRef}
                id="display-name"
                aria-label="Display name"
                className="h-auto max-w-xl border-0 bg-transparent px-0 py-0 text-3xl font-semibold tracking-tight focus-visible:ring-0"
                value={formValues.displayName}
                onBlur={() => {
                  setIsEditingName(false);
                  if (connectionLocked) void commitDisplayNameIfChanged();
                }}
                onChange={(event) => updateField('displayName', event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    setIsEditingName(false);
                    if (connectionLocked) void commitDisplayNameIfChanged();
                  } else if (event.key === 'Escape') {
                    if (datasource) {
                      setFormValues((current) => ({ ...current, displayName: datasource.displayName }));
                    } else {
                      setFormValues((current) => ({ ...current, displayName: DEFAULT_VALUES.displayName }));
                    }
                    setIsEditingName(false);
                  }
                }}
              />
            ) : (
              <button
                type="button"
                onClick={() => setIsEditingName(true)}
                aria-label={`Edit display name (${formValues.displayName})`}
                className="group inline-flex items-center gap-3 text-left"
              >
                <span className="border-b-2 border-dotted border-slate-300 pb-1 text-3xl font-semibold tracking-tight text-text-primary transition-colors group-hover:border-text-primary">
                  {formValues.displayName}
                </span>
                <Pencil
                  className="h-5 w-5 text-text-tertiary transition-colors group-hover:text-text-primary"
                  aria-hidden="true"
                />
              </button>
            )}
          </div>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <div>
              <FieldLabel htmlFor="provider" locked={connectionLocked}>Datasource Type</FieldLabel>
              <select
                id="provider"
                className="mt-2 flex h-10 w-full rounded-lg border border-border-medium bg-surface-primary px-3 py-2 text-sm text-text-primary disabled:cursor-default disabled:bg-surface-secondary disabled:text-text-secondary disabled:opacity-100"
                value={formValues.provider}
                disabled={connectionLocked}
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
              <FieldLabel htmlFor="database" locked={connectionLocked}>Database Name</FieldLabel>
              <Input id="database" className="mt-2" value={formValues.database} readOnly={connectionLocked} onChange={(event) => updateField('database', event.target.value)} />
            </div>
            <div>
              <FieldLabel htmlFor="host" locked={connectionLocked}>Host</FieldLabel>
              <Input id="host" className="mt-2" value={formValues.host} readOnly={connectionLocked} onChange={(event) => updateField('host', event.target.value)} placeholder="db.example.com" />
            </div>
            <div>
              <FieldLabel htmlFor="port" locked={connectionLocked}>Port</FieldLabel>
              <Input id="port" className="mt-2" value={formValues.port} readOnly={connectionLocked} onChange={(event) => updateField('port', event.target.value)} />
            </div>
            <div>
              <FieldLabel htmlFor="username" locked={connectionLocked}>Username</FieldLabel>
              <Input id="username" className="mt-2" value={formValues.username} readOnly={connectionLocked} onChange={(event) => updateField('username', event.target.value)} />
            </div>
            <div>
              <FieldLabel htmlFor="password" locked={connectionLocked}>Password</FieldLabel>
              <Input id="password" className="mt-2" type="password" value={formValues.password} readOnly={connectionLocked} onChange={(event) => updateField('password', event.target.value)} />
            </div>
            <div>
              <FieldLabel htmlFor="ssl-mode" locked={connectionLocked}>SSL mode</FieldLabel>
              <select
                id="ssl-mode"
                className="mt-2 flex h-10 w-full rounded-lg border border-border-medium bg-surface-primary px-3 py-2 text-sm text-text-primary disabled:cursor-default disabled:bg-surface-secondary disabled:text-text-secondary disabled:opacity-100"
                value={formValues.sslMode}
                disabled={connectionLocked}
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
                  <input type="checkbox" checked={formValues.encrypt} disabled={connectionLocked} onChange={(event) => updateField('encrypt', event.target.checked)} />
                  Encrypt connection
                  {connectionLocked ? <span className="rounded-full bg-surface-secondary px-2 py-0.5 text-[11px] font-medium uppercase tracking-[0.08em] text-text-tertiary">Read-only</span> : null}
                </label>
                <label className="mt-8 flex items-center gap-2 text-sm text-text-secondary">
                  <input
                    type="checkbox"
                    checked={formValues.trustServerCertificate}
                    disabled={connectionLocked}
                    onChange={(event) => updateField('trustServerCertificate', event.target.checked)}
                  />
                  Trust server certificate
                  {connectionLocked ? <span className="rounded-full bg-surface-secondary px-2 py-0.5 text-[11px] font-medium uppercase tracking-[0.08em] text-text-tertiary">Read-only</span> : null}
                </label>
              </>
            ) : null}
          </div>
          <div className="space-y-2">
            <div className="flex flex-wrap gap-3">
              <Button type="button" variant="outline" onClick={() => void handleTest()} disabled={busy !== null}>
                <TestTube2 className="h-4 w-4" />
                Test connection
              </Button>
              {!connectionLocked ? (
                <Button type="button" onClick={() => void handleSave()} disabled={busy !== null || saveGated}>
                  <Save className="h-4 w-4" />
                  Save datasource
                </Button>
              ) : null}
              {datasource ? (
                <Button type="button" variant="destructive" onClick={() => void handleDelete()} disabled={busy !== null}>
                  <Trash2 className="h-4 w-4" />
                  Remove datasource
                </Button>
              ) : null}
            </div>
            {saveGated ? (
              <p className="text-xs text-text-tertiary">Run Test connection successfully to enable saving.</p>
            ) : null}
          </div>
          {feedback ? <StatusBanner tone={feedback.tone} message={feedback.message} /> : null}
        </CardContent>
      </Card>
    </div>
  );
}
