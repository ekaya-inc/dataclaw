import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import AgentsPage from './AgentsPage';

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof ReactRouterDom>('react-router-dom');
  return {
    ...actual,
    useOutletContext: () => ({ refresh: vi.fn(async () => undefined) }),
  };
});

function response(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('AgentsPage', () => {
  it('renders agents, config guidance, and reveals keys on demand', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') {
        return response({ port: 18791, base_url: 'http://127.0.0.1:18791', mcp_url: 'http://127.0.0.1:18791/mcp', datasource_configured: true, agent_count: 1 });
      }
      if (url === '/api/agents' && !init?.method) {
        return response({
          agents: [
            {
              id: 'agent_1',
              name: 'Warehouse analyst',
              install_alias: 'warehouse-analyst-123456',
              masked_api_key: 'dclw-••••',
              can_query: true,
              can_execute: false,
              approved_query_scope: 'selected',
              approved_query_ids: ['query_1'],
              last_used_at: null,
            },
          ],
        });
      }
      if (url === '/api/queries') {
        return response({
          queries: [
            {
              query_id: 'query_1',
              datasource_id: 'ds_1',
              natural_language_prompt: 'List accounts',
              sql_query: 'SELECT * FROM accounts',
              allows_modification: false,
              parameters: [],
              output_columns: [],
              constraints: '',
            },
          ],
        });
      }
      if (url === '/api/agents/agent_1/reveal-key' && init?.method === 'POST') {
        return response({
          agent: {
            id: 'agent_1',
            name: 'Warehouse analyst',
            install_alias: 'warehouse-analyst-123456',
            masked_api_key: 'dclw-••••',
            api_key: 'dclw-live-secret',
            can_query: true,
            can_execute: false,
            approved_query_scope: 'selected',
            approved_query_ids: ['query_1'],
          },
        });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<AgentsPage />);

    await waitFor(() => expect(screen.getByRole('button', { name: /warehouse analyst/i })).toBeInTheDocument());
    expect(screen.getByText(/generic mcp config snippet/i)).toBeInTheDocument();
    expect(screen.getAllByText(/warehouse-analyst-123456/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/\$\{DATACLAW_API_KEY\}/)).toBeInTheDocument();
    expect(screen.queryByText(/dclw-live-secret/i)).not.toBeInTheDocument();

    const revealButtons = screen.getAllByRole('button', { name: /reveal key/i });
    expect(revealButtons.length).toBeGreaterThan(0);
    await userEvent.click(revealButtons[0]!);

    await waitFor(() => expect(screen.getByText('dclw-live-secret')).toBeInTheDocument());
  });
});
