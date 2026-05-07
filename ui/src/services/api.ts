import { QUERY_TEMPLATE } from '../constants';
import type { ApiEnvelope } from '../types/api';
import type { AgentBundleInstallLink, AgentFormValues, AgentRecord } from '../types/agent';
import type {
  DatasourceAdapterInfo,
  DatasourceFormValues,
  DatasourceRecord,
  RuntimeStatus,
  TemplateSyntaxHints,
  TestConnectionResult,
} from '../types/datasource';
import type { OutputColumn, QueryExecutionResult, QueryParameter, QueryValidationResult, SavedQuery } from '../types/query';
import type { MCPToolEventDetails, MCPToolEventFilters, MCPToolEventPage, MCPToolEventRecord, MCPToolEventType } from '../types/mcpEvent';

const JSON_HEADERS = {
  'Content-Type': 'application/json',
};

export const AUTH_UNAUTHORIZED_EVENT = 'dataclaw:auth-unauthorized';
const CSRF_HEADER = 'X-CSRF-Token';

let currentCSRFToken: string | undefined;

export class UnauthorizedError extends Error {
  constructor(message = 'unauthorized') {
    super(message);
    this.name = 'UnauthorizedError';
  }
}

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

function isUnsafeMethod(method: string | undefined): boolean {
  const normalized = (method ?? 'GET').toUpperCase();
  return normalized !== 'GET' && normalized !== 'HEAD' && normalized !== 'OPTIONS';
}

function requestMethod(input: RequestInfo | URL, init: RequestInit): string {
  if (init.method) return init.method;
  if (typeof Request !== 'undefined' && input instanceof Request) return input.method;
  return 'GET';
}

function requestURL(input: RequestInfo | URL): URL | null {
  const base =
    typeof window !== 'undefined' && window.location?.origin
      ? window.location.origin
      : 'http://localhost';
  const raw = typeof Request !== 'undefined' && input instanceof Request ? input.url : String(input);
  try {
    return new URL(raw, base);
  } catch {
    return null;
  }
}

function isSameOriginAdminAPIRequest(input: RequestInfo | URL): boolean {
  const url = requestURL(input);
  if (!url) return false;
  const origin =
    typeof window !== 'undefined' && window.location?.origin
      ? window.location.origin
      : url.origin;
  return url.origin === origin && url.pathname.startsWith('/api/');
}

function rememberAuthSession(session: AuthSession): AuthSession {
  if (session.authenticated && session.csrfToken) {
    currentCSRFToken = session.csrfToken;
  } else if (!session.authenticated) {
    currentCSRFToken = undefined;
  }
  return session;
}

function forgetAuthSession(): void {
  currentCSRFToken = undefined;
}

function pick(record: JsonRecord | null, ...keys: string[]): unknown {
  if (!record) return undefined;
  for (const key of keys) {
    if (key in record) return record[key];
  }
  return undefined;
}

async function parseResponse<T>(response: Response): Promise<T> {
  const isJson = response.headers.get('content-type')?.includes('application/json');
  const text = isJson ? await response.text() : '';
  const payload: ApiEnvelope<T> | T | null = text.trim() ? JSON.parse(text) as ApiEnvelope<T> | T : null;
  const payloadRecord = asRecord(payload);

  if (!response.ok) {
    const message = asString(pick(payloadRecord, 'message', 'error')) ?? response.statusText ?? 'Request failed';
    if (response.status === 401) {
      forgetAuthSession();
      if (typeof window !== 'undefined') {
        window.dispatchEvent(new Event(AUTH_UNAUTHORIZED_EVENT));
      }
      throw new UnauthorizedError(message);
    }
    throw new Error(message);
  }

  if (payloadRecord && 'data' in payloadRecord) {
    return (payloadRecord.data ?? payload) as T;
  }

  return payload as T;
}

async function apiFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const requestInit: RequestInit = {
    ...init,
    credentials: init.credentials ?? 'same-origin',
  };
  if (currentCSRFToken && isUnsafeMethod(requestMethod(input, init)) && isSameOriginAdminAPIRequest(input)) {
    const headers = new Headers(init.headers);
    if (!headers.has(CSRF_HEADER)) {
      headers.set(CSRF_HEADER, currentCSRFToken);
    }
    requestInit.headers = headers;
  }
  return fetch(input, {
    ...requestInit,
  });
}

function toTemplateSyntaxHints(raw: unknown): TemplateSyntaxHints | undefined {
  const record = asRecord(raw);
  if (!record) return undefined;
  const placeholders = pick(record, 'placeholderAntiExamples', 'placeholder_anti_examples');
  const pagination = pick(record, 'paginationAntiExamples', 'pagination_anti_examples');
  const placeholderList = Array.isArray(placeholders)
    ? placeholders.filter((value): value is string => typeof value === 'string')
    : [];
  const paginationList = Array.isArray(pagination)
    ? pagination.filter((value): value is string => typeof value === 'string')
    : [];
  const notes = asString(pick(record, 'notes'));
  if (placeholderList.length === 0 && paginationList.length === 0 && !notes) return undefined;
  return {
    placeholderAntiExamples: placeholderList,
    paginationAntiExamples: paginationList,
    notes,
  };
}

function toDatasourceAdapterInfo(raw: unknown): DatasourceAdapterInfo | null {
  const record = asRecord(raw);
  const type = asString(pick(record, 'type'));
  if (!record || !type) return null;
  const capabilities = asRecord(record.capabilities);
  return {
    type,
    displayName: asString(pick(record, 'displayName', 'display_name')) ?? type,
    description: asString(pick(record, 'description')),
    icon: asString(pick(record, 'icon')),
    sqlDialect: asString(pick(record, 'sqlDialect', 'sql_dialect')),
    capabilities: capabilities
      ? {
          supportsArrayParameters: asBoolean(pick(capabilities, 'supportsArrayParameters', 'supports_array_parameters')),
        }
      : undefined,
    templateSyntaxHints: toTemplateSyntaxHints(pick(record, 'templateSyntaxHints', 'template_syntax_hints')),
  };
}

function toDatasourceRecord(raw: unknown): DatasourceRecord | null {
  const record = asRecord(raw);
  const type = asString(pick(record, 'type'));
  if (!record || !type) return null;
  const host = asString(pick(record, 'host')) ?? '';
  const database = asString(pick(record, 'database')) ?? '';
  const username = asString(pick(record, 'username'));
  const password = asString(pick(record, 'password'));
  const sslMode = asString(pick(record, 'sslMode', 'ssl_mode')) as DatasourceRecord['sslMode'];
  const options = asRecord(pick(record, 'options'));
  const provider = asString(pick(record, 'provider')) ?? type;
  return {
    id: asString(pick(record, 'id')) ?? 'default',
    type,
    provider,
    sqlDialect: asString(pick(record, 'sqlDialect', 'sql_dialect')),
    displayName: (asString(pick(record, 'displayName', 'display_name')) ?? database) || provider,
    database,
    host,
    port: asNumber(pick(record, 'port')) ?? 0,
    username,
    password,
    sslMode,
    options: options ?? undefined,
    createdAt: asString(pick(record, 'createdAt', 'created_at')),
    updatedAt: asString(pick(record, 'updatedAt', 'updated_at')),
    templateSyntaxHints: toTemplateSyntaxHints(pick(record, 'templateSyntaxHints', 'template_syntax_hints')),
  };
}

function toQuery(raw: unknown): SavedQuery {
  const record = asRecord(raw);
  const parameterValues = Array.isArray(record?.parameters) ? record.parameters : [];
  const parameters = parameterValues
        .map((parameter) => {
          const parameterRecord = asRecord(parameter);
          const name = asString(pick(parameterRecord, 'name'));
          const type = asString(pick(parameterRecord, 'type')) as QueryParameter['type'] | undefined;
          if (!name || !type) return null;
          const next: QueryParameter = {
            name,
            type,
            description: asString(pick(parameterRecord, 'description')) ?? '',
            required: asBoolean(pick(parameterRecord, 'required')) ?? true,
          };
          const defaultValue = pick(parameterRecord, 'default');
          if (defaultValue !== undefined) {
            next.default = defaultValue;
          }
          return next;
        })
        .filter((parameter): parameter is QueryParameter => parameter !== null);
  const outputColumnValues = Array.isArray(record?.output_columns) ? record.output_columns : Array.isArray(record?.outputColumns) ? record.outputColumns : [];
  const outputColumns = outputColumnValues
        .map((column) => {
          const columnRecord = asRecord(column);
          const name = asString(pick(columnRecord, 'name'));
          if (!name) return null;
          return {
            name,
            type: asString(pick(columnRecord, 'type')) ?? 'text',
            description: asString(pick(columnRecord, 'description')) ?? '',
          };
        })
        .filter((column): column is OutputColumn => column !== null);

  return {
    id: asString(pick(record, 'id', 'query_id')) ?? 'unknown',
    datasourceId: asString(pick(record, 'datasourceId', 'datasource_id')),
    naturalLanguagePrompt: asString(pick(record, 'naturalLanguagePrompt', 'natural_language_prompt')) ?? '',
    additionalContext: asString(pick(record, 'additionalContext', 'additional_context')) ?? '',
    sql: asString(pick(record, 'sql', 'sql_query')) ?? QUERY_TEMPLATE,
    allowsModification: asBoolean(pick(record, 'allowsModification', 'allows_modification')) ?? false,
    parameters,
    outputColumns,
    constraints: asString(pick(record, 'constraints')) ?? '',
    createdAt: asString(pick(record, 'createdAt', 'created_at')),
    updatedAt: asString(pick(record, 'updatedAt', 'updated_at')),
  };
}

function toAgentRecord(raw: unknown): AgentRecord {
  const record = asRecord(raw);
  return {
    id: asString(pick(record, 'id')) ?? 'unknown',
    name: asString(pick(record, 'name')) ?? '',
    maskedApiKey: asString(pick(record, 'maskedApiKey', 'masked_api_key')) ?? '',
    apiKey: asString(pick(record, 'apiKey', 'api_key')),
    canQuery: asBoolean(pick(record, 'canQuery', 'can_query')) ?? false,
    canExecute: asBoolean(pick(record, 'canExecute', 'can_execute')) ?? false,
    canManageApprovedQueries:
      asBoolean(pick(record, 'canManageApprovedQueries', 'can_manage_approved_queries')) ?? false,
    approvedQueryScope: (asString(pick(record, 'approvedQueryScope', 'approved_query_scope')) ?? 'none') as AgentRecord['approvedQueryScope'],
    approvedQueryIds: Array.isArray(pick(record, 'approvedQueryIds', 'approved_query_ids'))
      ? (pick(record, 'approvedQueryIds', 'approved_query_ids') as unknown[]).filter((value): value is string => typeof value === 'string')
      : [],
    createdAt: asString(pick(record, 'createdAt', 'created_at')),
    updatedAt: asString(pick(record, 'updatedAt', 'updated_at')),
    lastUsedAt: asString(pick(record, 'lastUsedAt', 'last_used_at')),
  };
}

function toAgentBundleInstallLink(raw: unknown): AgentBundleInstallLink {
  const record = asRecord(raw);
  return {
    slug: asString(pick(record, 'slug')) ?? 'agent',
    code: asString(pick(record, 'code')) ?? '',
    bundleUrl: asString(pick(record, 'bundleUrl', 'bundle_url')) ?? '',
    expiresAt: asString(pick(record, 'expiresAt', 'expires_at')),
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

function toMCPToolEvent(raw: unknown): MCPToolEventRecord {
  const record = asRecord(raw);
  return {
    id: asString(pick(record, 'id')) ?? 'unknown',
    agentId: asString(pick(record, 'agentId', 'agent_id')),
    agentName: asString(pick(record, 'agentName', 'agent_name')) ?? '',
    toolName: asString(pick(record, 'toolName', 'tool_name')) ?? '',
    eventType: (asString(pick(record, 'eventType', 'event_type')) ?? 'tool_call') as MCPToolEventType,
    wasSuccessful: asBoolean(pick(record, 'wasSuccessful', 'was_successful')) ?? false,
    durationMs: asNumber(pick(record, 'durationMs', 'duration_ms')) ?? 0,
    hasDetails: asBoolean(pick(record, 'hasDetails', 'has_details')) ?? false,
    createdAt: asString(pick(record, 'createdAt', 'created_at')) ?? '',
  };
}

function toMCPToolEventDetails(raw: unknown): MCPToolEventDetails {
  const record = asRecord(raw);
  return {
    id: asString(pick(record, 'id')) ?? 'unknown',
    requestParams: asRecord(pick(record, 'requestParams', 'request_params')) ?? {},
    resultSummary: asRecord(pick(record, 'resultSummary', 'result_summary')) ?? {},
    errorMessage: asString(pick(record, 'errorMessage', 'error_message')) ?? '',
    queryName: asString(pick(record, 'queryName', 'query_name')) ?? '',
    sqlText: asString(pick(record, 'sqlText', 'sql_text')) ?? '',
  };
}

function datasourcePayload(values: DatasourceFormValues): Record<string, unknown> {
  return {
    type: values.type,
    provider: values.provider,
    display_name: values.displayName,
    config: {
      host: values.host,
      port: Number(values.port),
      database: values.database,
      username: values.username,
      password: values.password,
      ssl_mode: values.sslMode,
      ...(values.type === 'mssql'
        ? {
            encrypt: values.encrypt,
            trust_server_certificate: values.trustServerCertificate,
          }
        : {}),
    },
  };
}

function agentPayload(values: AgentFormValues): Record<string, unknown> {
  const canManageApprovedQueries = Boolean(values.canManageApprovedQueries);
  return {
    name: values.name,
    can_query: canManageApprovedQueries ? true : values.canQuery,
    can_execute: values.canExecute,
    can_manage_approved_queries: canManageApprovedQueries,
    approved_query_scope: values.approvedQueryScope,
    approved_query_ids: values.approvedQueryScope === 'selected' ? values.approvedQueryIds : [],
  };
}

export interface AuthSession {
  authenticated: boolean;
  expiresAt?: string | undefined;
  csrfToken?: string | undefined;
}

function toAuthSession(raw: unknown): AuthSession {
  const record = asRecord(raw);
  const session: AuthSession = {
    authenticated: asBoolean(pick(record, 'authenticated', 'isAuthenticated', 'is_authenticated')) ?? false,
    expiresAt: asString(pick(record, 'expiresAt', 'expires_at')),
  };
  const csrfToken = asString(pick(record, 'csrfToken', 'csrf_token'));
  if (csrfToken) {
    session.csrfToken = csrfToken;
  }
  return session;
}

export async function listMCPEvents(filters: MCPToolEventFilters = {}): Promise<MCPToolEventPage> {
  const params = new URLSearchParams();
  if (filters.range) params.set('range', filters.range);
  if (filters.eventType) params.set('event_type', filters.eventType);
  if (filters.toolName?.trim()) params.set('tool_name', filters.toolName.trim());
  if (filters.agentName?.trim()) params.set('agent_name', filters.agentName.trim());
  if (filters.limit !== undefined) params.set('limit', String(filters.limit));
  if (filters.offset !== undefined) params.set('offset', String(filters.offset));

  const query = params.toString();
  const data = await parseResponse<unknown>(await apiFetch(query ? `/api/mcp-events?${query}` : '/api/mcp-events'));
  const record = asRecord(data);
  const items = Array.isArray(record?.items) ? record.items : [];
  return {
    items: items.map(toMCPToolEvent),
    total: asNumber(pick(record, 'total')) ?? 0,
    limit: asNumber(pick(record, 'limit')) ?? (filters.limit ?? 50),
    offset: asNumber(pick(record, 'offset')) ?? (filters.offset ?? 0),
  };
}

export function mcpEventsDownloadURL(): string {
  return '/api/mcp-events.csv';
}

export async function getMCPEvent(id: string): Promise<MCPToolEventDetails> {
  const data = await parseResponse<unknown>(await apiFetch(`/api/mcp-events/${encodeURIComponent(id)}`));
  const record = asRecord(data);
  return toMCPToolEventDetails(record?.event);
}

export async function deleteMCPEvents(): Promise<void> {
  await parseResponse<void>(await apiFetch('/api/mcp-events', { method: 'DELETE' }));
}

export async function getAuthSession(): Promise<AuthSession> {
  const data = await parseResponse<unknown>(await apiFetch('/api/auth/session'));
  return rememberAuthSession(toAuthSession(data));
}

export async function signin(password: string, remember: boolean): Promise<AuthSession> {
  const data = await parseResponse<unknown>(
    await apiFetch('/api/auth/signin', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({ password, remember }),
    }),
  );
  return rememberAuthSession(toAuthSession(data));
}

export async function logout(): Promise<void> {
  await parseResponse<void>(
    await apiFetch('/api/auth/logout', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({}),
    }),
  );
  forgetAuthSession();
}

export async function getStatus(): Promise<RuntimeStatus | null> {
  const data = await parseResponse<unknown>(await apiFetch('/api/status'));
  const record = asRecord(data);
  return {
    version: asString(pick(record, 'version')),
    adminBaseUrl: asString(pick(record, 'adminBaseUrl', 'admin_base_url')),
    mcpBaseUrl: asString(pick(record, 'mcpBaseUrl', 'mcp_base_url')),
    mcpUrl: asString(pick(record, 'mcpUrl', 'mcp_url')),
    adminPort: asNumber(pick(record, 'adminPort', 'admin_port')),
    mcpPort: asNumber(pick(record, 'mcpPort', 'mcp_port')),
    listenerSplit: asBoolean(pick(record, 'listenerSplit', 'listener_split')),
    datasourceConfigured: asBoolean(pick(record, 'datasourceConfigured', 'datasource_configured')),
    agentCount: asNumber(pick(record, 'agentCount', 'agent_count')),
  };
}

export async function getDatasource(): Promise<DatasourceRecord | null> {
  const data = await parseResponse<unknown>(await apiFetch('/api/datasource'));
  const record = asRecord(data);
  const datasource = record && 'datasource' in record ? record.datasource : data;
  return toDatasourceRecord(datasource);
}

export async function getDatasourceTypes(): Promise<DatasourceAdapterInfo[]> {
  const data = await parseResponse<unknown>(await apiFetch('/api/datasource/types'));
  const record = asRecord(data);
  const types = Array.isArray(record?.types) ? record.types : [];
  return types.map(toDatasourceAdapterInfo).filter((type): type is DatasourceAdapterInfo => type !== null);
}

export async function saveDatasource(values: DatasourceFormValues): Promise<DatasourceRecord> {
  const data = await parseResponse<unknown>(
    await apiFetch('/api/datasource', {
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
    await apiFetch('/api/datasource/test', {
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
  await parseResponse<void>(await apiFetch('/api/datasource', { method: 'DELETE' }));
}

export async function listQueries(): Promise<SavedQuery[]> {
  const data = await parseResponse<unknown>(await apiFetch('/api/queries'));
  const record = asRecord(data);
  const queries = Array.isArray(record?.queries) ? record.queries : Array.isArray(data) ? data : [];
  return queries.map(toQuery);
}

export async function getQuery(id: string): Promise<SavedQuery> {
  const data = await parseResponse<unknown>(await apiFetch(`/api/queries/${id}`));
  const record = asRecord(data);
  return toQuery(record && 'query' in record ? record.query : data);
}

function isBlankOutputColumnPlaceholder(column: OutputColumn): boolean {
  return !column.name.trim() && !column.type.trim() && !column.description.trim();
}

function outputColumnsPayload(columns: OutputColumn[]): OutputColumn[] {
  return columns.filter((column) => !isBlankOutputColumnPlaceholder(column));
}

function approvedQueryPayload(query: Omit<SavedQuery, 'id'>): Record<string, unknown> {
  return {
    natural_language_prompt: query.naturalLanguagePrompt,
    additional_context: query.additionalContext,
    sql_query: query.sql,
    allows_modification: query.allowsModification,
    parameters: query.parameters,
    output_columns: outputColumnsPayload(query.outputColumns),
    constraints: query.constraints,
  };
}

export async function createQuery(query: Omit<SavedQuery, 'id'>): Promise<SavedQuery> {
  const data = await parseResponse<unknown>(
    await apiFetch('/api/queries', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify(approvedQueryPayload(query)),
    }),
  );
  const record = asRecord(data);
  return toQuery(record && 'query' in record ? record.query : data);
}

export async function updateQuery(id: string, query: Omit<SavedQuery, 'id'>): Promise<SavedQuery> {
  const data = await parseResponse<unknown>(
    await apiFetch(`/api/queries/${id}`, {
      method: 'PUT',
      headers: JSON_HEADERS,
      body: JSON.stringify(approvedQueryPayload(query)),
    }),
  );
  const record = asRecord(data);
  return toQuery(record && 'query' in record ? record.query : data);
}

export async function deleteQuery(id: string): Promise<void> {
  await parseResponse<void>(await apiFetch(`/api/queries/${id}`, { method: 'DELETE' }));
}

export async function validateQuery(sql: string, parameters: QueryParameter[], allowsModification: boolean): Promise<QueryValidationResult> {
  const data = await parseResponse<unknown>(
    await apiFetch('/api/queries/validate', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({ sql_query: sql, parameters, allows_modification: allowsModification }),
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

export async function testQuery(
  sql: string,
  parameters: QueryParameter[],
  allowsModification: boolean,
  values?: Record<string, unknown>,
): Promise<QueryExecutionResult> {
  const body: Record<string, unknown> = { sql_query: sql, parameters, allows_modification: allowsModification };
  if (values && Object.keys(values).length > 0) {
    body.parameter_values = values;
  }
  const data = await parseResponse<unknown>(
    await apiFetch('/api/queries/test', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify(body),
    }),
  );
  const record = asRecord(data);
  return toExecutionResult(record && 'result' in record ? record.result : data);
}

export async function executeSavedQuery(id: string, parameters?: Record<string, unknown>, limit = 100): Promise<QueryExecutionResult> {
  const data = await parseResponse<unknown>(
    await apiFetch(`/api/queries/${id}/execute`, {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({
        ...(parameters && Object.keys(parameters).length > 0 ? { parameters } : {}),
        limit,
      }),
    }),
  );
  const record = asRecord(data);
  return toExecutionResult(record && 'result' in record ? record.result : data);
}

export async function listAgents(): Promise<AgentRecord[]> {
  const data = await parseResponse<unknown>(await apiFetch('/api/agents'));
  const record = asRecord(data);
  const agents = Array.isArray(record?.agents) ? record.agents : [];
  return agents.map(toAgentRecord);
}

export async function getAgent(id: string): Promise<AgentRecord> {
  const data = await parseResponse<unknown>(await apiFetch(`/api/agents/${id}`));
  const record = asRecord(data);
  return toAgentRecord(record && 'agent' in record ? record.agent : data);
}

export async function createAgent(values: AgentFormValues): Promise<AgentRecord> {
  const data = await parseResponse<unknown>(
    await apiFetch('/api/agents', {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify(agentPayload(values)),
    }),
  );
  const record = asRecord(data);
  return toAgentRecord(record && 'agent' in record ? record.agent : data);
}

export async function updateAgent(id: string, values: AgentFormValues): Promise<AgentRecord> {
  const data = await parseResponse<unknown>(
    await apiFetch(`/api/agents/${id}`, {
      method: 'PUT',
      headers: JSON_HEADERS,
      body: JSON.stringify(agentPayload(values)),
    }),
  );
  const record = asRecord(data);
  return toAgentRecord(record && 'agent' in record ? record.agent : data);
}

export async function deleteAgent(id: string): Promise<void> {
  await parseResponse<void>(await apiFetch(`/api/agents/${id}`, { method: 'DELETE' }));
}

export async function revealAgentKey(id: string): Promise<AgentRecord> {
  const data = await parseResponse<unknown>(
    await apiFetch(`/api/agents/${id}/reveal-key`, {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({}),
    }),
  );
  const record = asRecord(data);
  return toAgentRecord(record && 'agent' in record ? record.agent : data);
}

export async function rotateAgentKey(id: string): Promise<AgentRecord> {
  const data = await parseResponse<unknown>(
    await apiFetch(`/api/agents/${id}/rotate-key`, {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({}),
    }),
  );
  const record = asRecord(data);
  return toAgentRecord(record && 'agent' in record ? record.agent : data);
}

export async function createAgentBundleInstallLink(id: string): Promise<AgentBundleInstallLink> {
  const data = await parseResponse<unknown>(
    await apiFetch(`/api/agents/${id}/bundle-code`, {
      method: 'POST',
      headers: JSON_HEADERS,
      body: JSON.stringify({}),
    }),
  );
  const record = asRecord(data);
  return toAgentBundleInstallLink(record && 'bundle_install' in record ? record.bundle_install : data);
}
