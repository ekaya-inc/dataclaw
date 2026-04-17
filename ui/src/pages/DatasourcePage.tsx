import { ArrowLeft, Pencil, Save, TestTube2, Unplug } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useOutletContext } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import { buildProviderOptions } from '../constants';
import { DatasourceTypePicker } from '../components/DatasourceTypePicker';
import { PageHeader } from '../components/PageHeader';
import { StatusBanner } from '../components/StatusBanner';
import { Button } from '../components/ui/Button';
import { Card, CardContent } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { deleteDatasource, getDatasource, getDatasourceTypes, saveDatasource, testDatasource } from '../services/api';
import type { DatasourceAdapterInfo, DatasourceFormValues, DatasourceRecord, SSLMode } from '../types/datasource';
import { parsePostgresUrl } from '../utils/connectionString';

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

const DISCONNECT_CONFIRMATION = 'disconnect datasource';

export default function DatasourcePage(): JSX.Element {
  const { refresh } = useOutletContext<AppOutletContext>();
  const [adapterTypes, setAdapterTypes] = useState<DatasourceAdapterInfo[]>([]);
  const [datasource, setDatasource] = useState<DatasourceRecord | null>(null);
  const [formValues, setFormValues] = useState<DatasourceFormValues>(DEFAULT_VALUES);
  const [busy, setBusy] = useState<'loading' | 'saving' | 'testing' | 'deleting' | null>('loading');
  const [feedback, setFeedback] = useState<{ tone: 'info' | 'success' | 'danger'; message: string } | null>(null);
  const [isEditingName, setIsEditingName] = useState(false);
  const [testPassed, setTestPassed] = useState(false);
  const [showDisconnectDialog, setShowDisconnectDialog] = useState(false);
  const [disconnectConfirmText, setDisconnectConfirmText] = useState('');
  const [typeSelected, setTypeSelected] = useState(false);
  const [connString, setConnString] = useState('');
  const [connStringError, setConnStringError] = useState('');
  const connectionLocked = datasource !== null;
  const providerOptions = useMemo(() => buildProviderOptions(adapterTypes), [adapterTypes]);
  const nameInputRef = useRef<HTMLInputElement>(null);
  const disconnectInputRef = useRef<HTMLInputElement>(null);

  const disconnectConfirmed = disconnectConfirmText.trim().toLowerCase() === DISCONNECT_CONFIRMATION;

  useEffect(() => {
    void (async () => {
      try {
        const [currentDatasource, currentAdapterTypes] = await Promise.all([getDatasource(), getDatasourceTypes()]);
        setAdapterTypes(currentAdapterTypes);
        setDatasource(currentDatasource);
        if (currentDatasource) {
          setFormValues(toFormValues(currentDatasource));
          setIsEditingName(false);
          setTypeSelected(true);
        }
      } catch (error) {
        setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to load datasource.' });
      } finally {
        setBusy(null);
      }
    })();
  }, []);

  useEffect(() => {
    if (!isEditingName) return;
    nameInputRef.current?.focus();
    nameInputRef.current?.select();
  }, [isEditingName]);

  useEffect(() => {
    if (!showDisconnectDialog) return;
    setDisconnectConfirmText('');
    disconnectInputRef.current?.focus();
  }, [showDisconnectDialog]);

  const updateField = <K extends keyof DatasourceFormValues>(field: K, value: DatasourceFormValues[K]): void => {
    if (CONNECTION_FIELDS.has(field)) {
      setTestPassed(false);
    }
    setFormValues((current) => ({ ...current, [field]: value }));
  };

  const handleProviderChange = (providerId: string): void => {
    const provider = providerOptions.find((item) => item.id === providerId);
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

  const handlePickerSelect = (providerId: string): void => {
    handleProviderChange(providerId);
    setFeedback(null);
    setTypeSelected(true);
  };

  const handleCancelSetup = (): void => {
    setFormValues(DEFAULT_VALUES);
    setTestPassed(false);
    setFeedback(null);
    setTypeSelected(false);
    setConnString('');
    setConnStringError('');
  };

  const handleParseConnectionString = (): void => {
    const parsed = parsePostgresUrl(connString.trim());
    if (!parsed) {
      setConnStringError('Invalid connection string. Expected: postgresql://user:password@host:port/database');
      return;
    }
    const detected = parsed.providerId ? providerOptions.find((provider) => provider.id === parsed.providerId) : undefined;
    setTestPassed(false);
    setFormValues((current) => ({
      ...current,
      type: 'postgres',
      provider: detected?.id ?? current.provider,
      host: parsed.host,
      port: String(parsed.port),
      database: parsed.database,
      username: parsed.user,
      password: parsed.password,
      sslMode: parsed.sslMode,
    }));
    setConnString('');
    setConnStringError('');
  };

  const applyConnectionString = (value: string): boolean => {
    const parsed = parsePostgresUrl(value.trim());
    if (!parsed) {
      return false;
    }
    const detected = parsed.providerId ? providerOptions.find((provider) => provider.id === parsed.providerId) : undefined;
    setTestPassed(false);
    setFormValues((current) => ({
      ...current,
      type: 'postgres',
      provider: detected?.id ?? current.provider,
      host: parsed.host,
      port: String(parsed.port),
      database: parsed.database,
      username: parsed.user,
      password: parsed.password,
      sslMode: parsed.sslMode,
    }));
    setConnString('');
    setConnStringError('');
    return true;
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
      await refresh();
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
      await refresh();
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to update display name.' });
    } finally {
      setBusy(null);
    }
  };

  const handleDisconnect = async (): Promise<void> => {
    if (!disconnectConfirmed) return;
    setBusy('deleting');
    setFeedback(null);
    try {
      await deleteDatasource();
      setDatasource(null);
      setFormValues(DEFAULT_VALUES);
      setIsEditingName(false);
      setTestPassed(false);
      setTypeSelected(false);
      setShowDisconnectDialog(false);
      setDisconnectConfirmText('');
      setFeedback({
        tone: 'success',
        message: 'Datasource disconnected. Saved queries were cleared. Agents stay configured, but their MCP tools remain unavailable until you connect a datasource again.',
      });
      await refresh();
    } catch (error) {
      setFeedback({ tone: 'danger', message: error instanceof Error ? error.message : 'Failed to disconnect datasource.' });
    } finally {
      setBusy(null);
    }
  };

  const saveGated = !connectionLocked && !testPassed;

  const showPicker = !datasource && !typeSelected;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Datasource"
        description="Configure the datasource. The agent will be operating with the credentials you supply below so limit their access appropriately."
      />
      {showPicker ? <DatasourceTypePicker onSelect={handlePickerSelect} providers={providerOptions} /> : null}
      {!showPicker ? (
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
          {!connectionLocked && formValues.type === 'postgres' ? (
            <div className="rounded-xl border border-border-light bg-surface-secondary/60 p-4">
              <Label htmlFor="connection-string">Quick setup: paste a connection string</Label>
              <p className="mt-1 text-xs text-text-tertiary">
                Optional. Fills host, port, user, password, database, and SSL mode. Detects Supabase, Neon, Redshift, and CockroachDB from the hostname.
              </p>
              <div className="mt-3 flex flex-wrap items-start gap-2">
                <Input
                  id="connection-string"
                  className="flex-1 min-w-[260px]"
                  placeholder="postgresql://user:password@host:port/database"
                  value={connString}
                  onChange={(event) => {
                    setConnString(event.target.value);
                    if (connStringError) setConnStringError('');
                  }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' && connString.trim()) {
                      event.preventDefault();
                      handleParseConnectionString();
                    }
                  }}
                  onPaste={(event) => {
                    const pastedValue = event.clipboardData.getData('text');
                    if (!pastedValue.trim()) {
                      return;
                    }
                    if (applyConnectionString(pastedValue)) {
                      event.preventDefault();
                    }
                  }}
                  autoComplete="off"
                  spellCheck={false}
                />
                <Button type="button" variant="outline" onClick={handleParseConnectionString} disabled={!connString.trim()}>
                  Parse
                </Button>
              </div>
              {connStringError ? (
                <p className="mt-2 text-xs text-red-600">{connStringError}</p>
              ) : null}
            </div>
          ) : null}
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <div>
              <Label htmlFor="database">Database Name</Label>
              <Input id="database" className="mt-2" value={formValues.database} readOnly={connectionLocked} onChange={(event) => updateField('database', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="host">Host</Label>
              <Input id="host" className="mt-2" value={formValues.host} readOnly={connectionLocked} onChange={(event) => updateField('host', event.target.value)} placeholder="db.example.com" />
            </div>
            <div>
              <Label htmlFor="port">Port</Label>
              <Input id="port" className="mt-2" value={formValues.port} readOnly={connectionLocked} onChange={(event) => updateField('port', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="username">Username</Label>
              <Input id="username" className="mt-2" value={formValues.username} readOnly={connectionLocked} onChange={(event) => updateField('username', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="password">Password</Label>
              <Input id="password" className="mt-2" type="password" value={formValues.password} readOnly={connectionLocked} onChange={(event) => updateField('password', event.target.value)} />
            </div>
            <div>
              <Label htmlFor="ssl-mode">SSL mode</Label>
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
                </label>
                <label className="mt-8 flex items-center gap-2 text-sm text-text-secondary">
                  <input
                    type="checkbox"
                    checked={formValues.trustServerCertificate}
                    disabled={connectionLocked}
                    onChange={(event) => updateField('trustServerCertificate', event.target.checked)}
                  />
                  Trust server certificate
                </label>
              </>
            ) : null}
          </div>
          <div className="space-y-2">
            <div className="flex flex-wrap gap-3">
              {!connectionLocked ? (
                <Button type="button" variant="outline" onClick={handleCancelSetup} disabled={busy !== null}>
                  <ArrowLeft className="h-4 w-4" />
                  Cancel
                </Button>
              ) : null}
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
                <Button type="button" variant="destructive" onClick={() => setShowDisconnectDialog(true)} disabled={busy !== null}>
                  <Unplug className="h-4 w-4" />
                  Disconnect datasource
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
      ) : null}
      {showPicker && feedback ? <StatusBanner tone={feedback.tone} message={feedback.message} /> : null}
      {showDisconnectDialog ? (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="disconnect-title"
          className="fixed inset-0 z-30 flex items-center justify-center bg-slate-950/60 p-4"
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              setShowDisconnectDialog(false);
              setDisconnectConfirmText('');
            }
          }}
        >
          <div className="w-full max-w-md rounded-2xl bg-surface-primary p-6 shadow-xl">
            <h2 id="disconnect-title" className="text-lg font-semibold text-text-primary">
              Disconnect datasource?
            </h2>
            <p className="mt-3 text-sm text-text-secondary">
              This removes the datasource and clears all saved approved queries. The agent API key is also rotated, so anything currently connected with the old key will lose access.
            </p>
            <p className="mt-4 text-sm text-text-secondary">
              Type <code className="rounded bg-surface-secondary px-1.5 py-0.5 font-mono text-[12px] text-text-primary">{DISCONNECT_CONFIRMATION}</code> to confirm.
            </p>
            <Input
              ref={disconnectInputRef}
              id="disconnect-confirm"
              aria-label="Type disconnect datasource to confirm"
              className="mt-3"
              value={disconnectConfirmText}
              onChange={(event) => setDisconnectConfirmText(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && disconnectConfirmed && busy === null) {
                  event.preventDefault();
                  void handleDisconnect();
                }
              }}
              autoComplete="off"
              spellCheck={false}
            />
            <div className="mt-5 flex justify-end gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setShowDisconnectDialog(false);
                  setDisconnectConfirmText('');
                }}
                disabled={busy !== null}
              >
                Cancel
              </Button>
              <Button
                type="button"
                variant="destructive"
                onClick={() => void handleDisconnect()}
                disabled={!disconnectConfirmed || busy !== null}
              >
                <Unplug className="h-4 w-4" />
                Disconnect datasource
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
