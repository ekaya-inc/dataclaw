import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';

import ApprovedQueriesPage from './ApprovedQueriesPage';

function response(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('ApprovedQueriesPage', () => {
  it('creates the seed helper and omits pending-review flows', async () => {
    const fetchMock = vi.spyOn(global, 'fetch');
    fetchMock.mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') {
        return response({ datasource: { id: 'ds_1', type: 'postgres', display_name: 'Primary datasource', host: 'db.example.com', port: 5432, name: 'warehouse', user: 'analyst', ssl_mode: 'require' } });
      }
      if (url === '/api/queries' && !init?.method) return response({ queries: [] });
      if (url === '/api/queries' && init?.method === 'POST') return response({ query: { id: 'query_1', name: 'Connectivity check', description: '', sql: 'SELECT true AS connected', is_enabled: true, parameters: [] } });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<ApprovedQueriesPage />);

    await waitFor(() => expect(screen.getAllByText(/no approved queries yet/i).length).toBeGreaterThan(0));

    await userEvent.click(screen.getByRole('button', { name: /use select true as connected/i }));
    await userEvent.click(screen.getByRole('button', { name: /create query/i }));

    await waitFor(() => expect(screen.getByText(/approved query created/i)).toBeInTheDocument());
    expect(screen.queryByRole('tab', { name: /pending/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: /rejected/i })).not.toBeInTheDocument();
  });
});
