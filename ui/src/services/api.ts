import { QUERY_TEMPLATE } from '../constants';
import type { ApiEnvelope } from '../types/api';
import type { DatasourceFormValues, DatasourceRecord, RuntimeStatus, TestConnectionResult } from '../types/datasource';
import type { OpenClawConfig } from '../types/openclaw';
import type { QueryExecutionResult, QueryParameter, QueryValidationResult, SavedQuery } from '../types/query';

const JSON_HEADERS = {
  'Content-Type': 'application/json',
};

type JsonRecord = Record<string, unknown>;

function asRecord(value: unknown): JsonRecord | null {
  return typeof value === 'object' && value !== null ? (value as JsonRecord) : null;
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

function asBoolean(value: unknown): boolean | undefined {
  return typeof value === 'boolean' ? value : undefined;
}

function asNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

function pick(record: JsonRecord | null, ...keys: string[]): unknown {
  if (!record) return undefined;
  for (const key of keys) {
    if (key in record) return record[key];
  }
  return undefined;
}

function recordField(record: JsonRecord | null, key: string): JsonRecord | null {
  return asRecord(record?.[key]);
}

async function parseResponse<T>(response: Response): Promise<T> {
  const isJson = response.headers.get('content-type')?.includes('application/json');
  const payload: ApiEnvelope<T> | T | null = isJson ? (await response.json()) as ApiEnvelope<T> | T : null;
  const payloadRecord = asRecord(payload);

  if (!response.ok) {
    const message = asString(pick(payloadRecord, 'message', 'error')) ?? response.statusText ?? 'Request failed';
    throw new Error(message);
  }

  if (payloadRecord && 'data' in payloadRecord) {
    return (payloadRecord.data ?? payload) as T;
  }

  return payload as T;
}

function toDatasourceRecord(raw: unknown): DatasourceRecord | null {
  const record = asRecord(raw);
  if (!record) return null;

  const typeValue = pick(record, 'type');
  const type = typeValue === 'mssql' ? 'mssql' : 'postgres';

  return {
    id: asString(pick(record, 'id', 'datasource_id')) ?? 'default',
    type,
    provider: asString(pick(record, 'provider')),
    displayName: asString(pick(record, 'displayName', 'display_name', 'name')) ?? 'Datasource',
    database: asString(pick(record, 'database', 'name')) ?? '',
    host: asString(pick(record, 'host')) ?? '',
    port: asNumber(pick(record, 'port')) ?? 0,
    username: asString(pick(record, 'username', 'user')),
    password: asString(pick(record, 'password')),
    sslMode: asString(pick(record, 'sslMode', 'ssl_mode')) as DatasourceRecord['sslMode'],
    options: recordField(record, 'options') ?? recordField(record, 'extra') ?? undefined,
    createdAt: asString(pick(record, 'createdAt', 'created_at')),
    updatedAt: asString(pick(record, 'updatedAt', 'updated_at')),
  };
}

function toQueryParameter(raw: unknown): QueryParameter | null {
  const record = asRecord(raw);
  if (!record) return null;
  const type = asString(pick(record, 'type')) as QueryParameter['type'] | undefined;
  if (!type) return null;
  return {
    name: asString(pick(record, 'name')) ?? '',
    type,
    description: asString(pick(record, 'description')) ?? '',
    required: asBoolean(pick(record, 'required')) ?? true,
    default: asString(pick(record, 'default')) ?? null,
  };
}

function toQuery(raw: unknown): SavedQuery {
  const record = asRecord(raw) ?? {};
  const parameters = Array.isArray(record.parameters)
    ? record.parameters.map(toQueryParameter).filter((parameter): parameter is QueryParameter => parameter !== null)
    : [];

  return {
    id: asString(pick(record, 'id', 'query_id')) ?? crypto.randomUUID(),
    datasourceId: asString(pick(record, 'datasourceId', 'datasource_id')),
    name: asString(pick(record, 'name', 'natural_language_prompt')) ?? 'Untitled query',
    description: asString(pick(record, 'description', 'additional_context')),
    sql: asString(pick(record, 'sql', 'sql_query')) ?? QUERY_TEMPLATE,
    isEnabled: asBoolean(pick(record, 'isEnabled', 'is_enabled')) ?? true,
    parameters,
    createdAt: asString(pick(record, 'createdAt', 'created_at')),
    updatedAt: asString(pick(record, 'updatedAt', 'updated_at')),
  };
}

function toOpenClawConfig(raw: unknown, runtime: RuntimeStatus | null): OpenClawConfig {
  const record = asRecord(raw);
  return {
    apiKey: asString(pick(record, 'apiKey', 'api_key')) ?? '',
    maskedApiKey: asString(pick(record, 'maskedApiKey', 'masked_api_key')),
    endpointUrl:
      asString(pick(record, 'endpointUrl', 'endpoint_url', 'mcp_url')) ?? (runtime?.baseUrl ? `${runtime.baseUrl}/mcp` : undefined),
    transport: asString(pick(record, 'transport')) ?? 'streamable-http',
    installCommand: asString(pick(record, 'installCommand', 'install_command', 'openclaw_cli')),
    generatedAt: asString(pick(record, 'generatedAt', 'generated_at')),
  };
}

function toExecutionResult(raw: unknown): QueryExecutionResult {
  const record = asRecord(raw);
  const columns = Array.isArray(record?.columns)
    ? record.columns
        .map((column) => {
          const columnRecord = asRecord(column);
          const name = asString(pick(columnRecord, 'name'));
          const type = asString(pick(columnRecord, 'type')) ?? 'unknown';
          return name ? { name, type } : null;
        })
        .filter((column): column is { name: string; type: string } => column !== null)
    : [];
  const rows = Array.isArray(record?.rows)
    ? record.rows.map((row) => asRecord(row) ?? {})
    : [];

  return {
    columns,
    rows,
    rowCount: asNumber(pick(record, 'rowCount', 'row_count')) ?? rows.length,
  };
}

function datasourcePayload(values: DatasourceFormValues): Record<string, unknown> {
  return {
    type: values.type,
    provider: values.provider,
    display_name: values.displayName,
    host: values.host,
    port: Number(values.port),
    name: values.database,
    user: values.username,
    password: values.password,
    ssl_mode: values.sslMode,
    options:
      values.type === 'mssql'
        ? {
            encrypt: values.encrypt,
            trust_server_certificate: values.trustServerCertificate,
          }
        : undefined,
  };
}

export async function getStatus(): Promise<RuntimeStatus | null> {
  const data = await parseResponse<unknown>(await fetch('/api/status'));
  const record = asRecord(data);
  return {
    version: asString(pick(record, 'version')),
    baseUrl: asString(pick(record, 'baseUrl', 'base_url', 'serverUrl', 'server_url')),
    port: asNumber(pick(record, 'port')),
    datasourceConfigured: asBoolean(pick(record, 'datasourceConfigured', 'datasource_configured')),
  };
}

export async function getDatasource(): Promise<DatasourceRecord | null> {
  const data = await parseResponse<unknown>(await fetch('/api/datasource'));
  const record = asRecord(data);
  const datasource = record && 'datasource' in record ? record.datasource : data;
  return toDatasourceRecord(datasource);
}

export async function saveDatasource(values: DatasourceFormValues): Promise<DatasourceRecord> {
  const data = await parseResponse<unknown>(
    await fetch('/api/datasource', {
      method: 'PUT',
      headers: JSON_HEADERS,
      body: JSON.stringify(datasourcePayload(values)),
    }),
  );
  const record = asRecord(data);
  return (
    toDatasourceRecord(record && 'datasource' in record ? record.datasource : data) ?? {
      id: 'default',
      type: values.type,
      provider: values.provider,
      displayName: values.displayName,
      database: values.database,
      host: values.host,
      port: Number(values.port),
      username: values.username,
      password: values.password,
      sslMode: values.sslMode,
    }
  );
}

export async function testDatasource(values: DatasourceFormValues): Promise<TestConnectionResult> {
  const data = await parseResponse<unknown>(
    await fetch('/api/datasource/test', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify(datasourcePayload(values)),
    }),
  );
  const record = asRecord(data);
  return {
    success: asBoolean(pick(record, 'success')) ?? true,
    message: asString(pick(record, 'message')) ?? 'Connection succeeded.',
  };
}

export async function deleteDatasource(): Promise<void> {
  await parseResponse<void>(await fetch('/api/datasource', { method: 'DELETE' }));
}

export async function listQueries(): Promise<SavedQuery[]> {
  const data = await parseResponse<unknown>(await fetch('/api/queries'));
  const record = asRecord(data);
  const queries = Array.isArray(record?.queries) ? record.queries : Array.isArray(data) ? data : [];
  return queries.map(toQuery);
}

export async function createQuery(query: Omit<SavedQuery, 'id'>): Promise<SavedQuery> {
  const data = await parseResponse<unknown>(
    await fetch('/api/queries', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({
        name: query.name,
        description: query.description,
        sql: query.sql,
        is_enabled: query.isEnabled,
        parameters: query.parameters,
      }),
    }),
  );
  const record = asRecord(data);
  return toQuery(record && 'query' in record ? record.query : data);
}

export async function updateQuery(id: string, query: Omit<SavedQuery, 'id'>): Promise<SavedQuery> {
  const data = await parseResponse<unknown>(
    await fetch(`/api/queries/${id}`, {
      method: 'PUT',
      headers: JSON_HEADERS,
      body: JSON.stringify({
        name: query.name,
        description: query.description,
        sql: query.sql,
        is_enabled: query.isEnabled,
        parameters: query.parameters,
      }),
    }),
  );
  const record = asRecord(data);
  return toQuery(record && 'query' in record ? record.query : data);
}

export async function deleteQuery(id: string): Promise<void> {
  await parseResponse<void>(await fetch(`/api/queries/${id}`, { method: 'DELETE' }));
}

export async function validateQuery(sql: string, parameters: QueryParameter[]): Promise<QueryValidationResult> {
  const data = await parseResponse<unknown>(
    await fetch('/api/queries/validate', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({ sql_query: sql, parameters }),
    }),
  );
  const record = asRecord(data);
  const warnings = Array.isArray(record?.warnings)
    ? record.warnings.filter((warning): warning is string => typeof warning === 'string')
    : undefined;
  return {
    valid: asBoolean(pick(record, 'valid')) ?? true,
    message: asString(pick(record, 'message')),
    warnings,
  };
}

export async function testQuery(sql: string, parameters: QueryParameter[]): Promise<QueryExecutionResult> {
  const data = await parseResponse<unknown>(
    await fetch('/api/queries/test', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({ sql_query: sql, parameters }),
    }),
  );
  const record = asRecord(data);
  return toExecutionResult(record && 'result' in record ? record.result : data);
}

export async function executeSavedQuery(id: string): Promise<QueryExecutionResult> {
  const data = await parseResponse<unknown>(
    await fetch(`/api/queries/${id}/execute`, {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({}),
    }),
  );
  const record = asRecord(data);
  return toExecutionResult(record && 'result' in record ? record.result : data);
}

export async function getOpenClaw(runtime: RuntimeStatus | null): Promise<OpenClawConfig> {
  const data = await parseResponse<unknown>(await fetch('/api/openclaw'));
  const record = asRecord(data);
  return toOpenClawConfig(record && 'openclaw' in record ? record.openclaw : data, runtime);
}

export async function rotateOpenClawKey(runtime: RuntimeStatus | null): Promise<OpenClawConfig> {
  const data = await parseResponse<unknown>(
    await fetch('/api/openclaw/rotate-key', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({}),
    }),
  );
  const record = asRecord(data);
  return toOpenClawConfig(record && 'openclaw' in record ? record.openclaw : data, runtime);
}
