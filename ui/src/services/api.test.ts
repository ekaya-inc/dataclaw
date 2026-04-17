import { describe, expect, it, vi } from 'vitest';

import { createAgent, executeSavedQuery, getDatasource, getDatasourceTypes, testQuery, validateQuery } from './api';
import type { QueryParameter } from '../types/query';

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('api service contracts', () => {
  it('sends sql_query and parameters during validation', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({ success: true, data: { valid: true } }),
    );
    const parameters: QueryParameter[] = [
      { name: 'account_id', type: 'uuid', description: 'Account id', required: true, default: null },
    ];

    await validateQuery('SELECT * FROM accounts WHERE id = {{account_id}}', parameters, false);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(init?.method).toBe('POST');
    expect(JSON.parse(String(init?.body))).toEqual({
      sql_query: 'SELECT * FROM accounts WHERE id = {{account_id}}',
      parameters,
      allows_modification: false,
    });
  });

  it('sends sql_query to the draft test endpoint', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({ success: true, data: { columns: [], rows: [], row_count: 0 } }),
    );

    await testQuery('SELECT 1', [], false);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0] ?? [];
    expect(input).toBe('/api/queries/test');
    expect(JSON.parse(String(init?.body))).toEqual({
      sql_query: 'SELECT 1',
      parameters: [],
      allows_modification: false,
    });
  });

  it('sends parameters and limit to the saved-query execute endpoint', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({ success: true, data: { columns: [], rows: [], row_count: 0 } }),
    );

    await executeSavedQuery('query_1', { account_id: '550e8400-e29b-41d4-a716-446655440000' }, 25);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0] ?? [];
    expect(input).toBe('/api/queries/query_1/execute');
    expect(JSON.parse(String(init?.body))).toEqual({
      parameters: { account_id: '550e8400-e29b-41d4-a716-446655440000' },
      limit: 25,
    });
  });

  it('creates agents with the selected approved-query scope payload', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({
        success: true,
        data: {
          agent: {
            id: 'agent_1',
            name: 'Warehouse analyst',
            masked_api_key: 'dclw-••••',
            api_key: 'dclw-live-secret',
            can_query: true,
            can_execute: false,
            approved_query_scope: 'selected',
            approved_query_ids: ['query_1'],
          },
        },
      }),
    );

    const created = await createAgent({
      name: 'Warehouse analyst',
      canQuery: true,
      canExecute: false,
      approvedQueryScope: 'selected',
      approvedQueryIds: ['query_1'],
    });

    expect(created.apiKey).toBe('dclw-live-secret');
    const [, init] = vi.mocked(global.fetch).mock.calls[0] ?? [];
    expect(init?.method).toBe('POST');
    expect(JSON.parse(String(init?.body))).toEqual({
      name: 'Warehouse analyst',
      can_query: true,
      can_execute: false,
      approved_query_scope: 'selected',
      approved_query_ids: ['query_1'],
    });
  });

  it('preserves datasource types from the server without coercing unknown adapters', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({
        success: true,
        data: {
          datasource: {
            id: 'ds_1',
            type: 'snowflake',
            sql_dialect: 'PostgreSQL',
            display_name: 'Warehouse',
          },
        },
      }),
    );

    const datasource = await getDatasource();

    expect(datasource?.type).toBe('snowflake');
    expect(datasource?.sqlDialect).toBe('PostgreSQL');
  });

  it('loads datasource type metadata from the server', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({
        success: true,
        data: {
          types: [
            {
              type: 'postgres',
              display_name: 'PostgreSQL',
              sql_dialect: 'PostgreSQL',
              capabilities: { supports_array_parameters: true },
            },
          ],
        },
      }),
    );

    const types = await getDatasourceTypes();

    expect(types).toEqual([
      {
        type: 'postgres',
        displayName: 'PostgreSQL',
        description: undefined,
        icon: undefined,
        sqlDialect: 'PostgreSQL',
        capabilities: { supportsArrayParameters: true },
      },
    ]);
  });
});
