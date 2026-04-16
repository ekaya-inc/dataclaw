import { describe, expect, it, vi } from 'vitest';

import { executeSavedQuery, getOpenClaw, testQuery, validateQuery } from './api';
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

    await validateQuery('SELECT * FROM accounts WHERE id = {{account_id}}', parameters);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(init?.method).toBe('POST');
    expect(JSON.parse(String(init?.body))).toEqual({
      sql_query: 'SELECT * FROM accounts WHERE id = {{account_id}}',
      parameters,
    });
  });

  it('sends sql_query to the draft test endpoint', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({ success: true, data: { columns: [], rows: [], row_count: 0 } }),
    );

    await testQuery('SELECT 1', []);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0] ?? [];
    expect(input).toBe('/api/queries/test');
    expect(JSON.parse(String(init?.body))).toEqual({
      sql_query: 'SELECT 1',
      parameters: [],
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

  it('uses the server-provided OpenClaw install command', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({
        success: true,
        data: {
          api_key: 'dclw-live-secret',
          mcp_url: 'http://127.0.0.1:18790/mcp',
          openclaw_cli:
            'openclaw mcp set dataclaw \'{"url":"http://127.0.0.1:18790/mcp","headers":{"Authorization":"Bearer ${DATACLAW_API_KEY}"}}\'',
        },
      }),
    );

    const config = await getOpenClaw({ baseUrl: 'http://127.0.0.1:18790', port: 18790, datasourceConfigured: true });

    expect(config.installCommand).toContain('${DATACLAW_API_KEY}');
    expect(config.installCommand).not.toContain('dclw-live-secret');
    expect(config.endpointUrl).toBe('http://127.0.0.1:18790/mcp');
  });
});
