import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import ApprovedQueriesPage from './ApprovedQueriesPage';
import { ToastProvider } from '../components/ui/Toast';

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof ReactRouterDom>('react-router-dom');
  return {
    ...actual,
    useOutletContext: () => ({ refresh: vi.fn(async () => undefined), markAgentRevealed: vi.fn(), resetAgentRevealed: vi.fn() }),
  };
});

function response(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function renderPage(): void {
  render(
    <ToastProvider>
      <MemoryRouter initialEntries={['/queries']}>
        <Routes>
          <Route path="/queries" element={<ApprovedQueriesPage />} />
          <Route path="/queries/:id" element={<div>Editor page for route</div>} />
        </Routes>
      </MemoryRouter>
    </ToastProvider>,
  );
}

const DATASOURCE_PAYLOAD = {
  datasource: {
    id: 'ds_1',
    type: 'postgres',
    display_name: 'Primary datasource',
    host: 'db.example.com',
    port: 5432,
    name: 'warehouse',
    user: 'analyst',
    ssl_mode: 'require',
  },
};

describe('ApprovedQueriesPage', () => {
  it('seeds a connectivity check and navigates to the editor', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') return response(DATASOURCE_PAYLOAD);
      if (url === '/api/status') return response({ version: 'test', port: 18790, datasource_configured: true });
      if (url === '/api/queries' && !init?.method) return response({ queries: [] });
      if (url === '/api/queries' && init?.method === 'POST') {
        return response({
          query: {
            query_id: 'query_seed',
            datasource_id: 'ds_1',
            natural_language_prompt: 'Connectivity check',
            additional_context: 'Verify the datasource is reachable by returning a simple boolean.',
            sql_query: 'SELECT true AS connected',
            allows_modification: false,
            parameters: [],
            output_columns: [{ name: 'connected', type: 'boolean', description: '' }],
            constraints: '',
          },
        });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    await waitFor(() => expect(screen.getByRole('button', { name: /use select true as connected/i })).toBeEnabled());

    await userEvent.click(screen.getByRole('button', { name: /use select true as connected/i }));

    await waitFor(() => expect(screen.getByText(/editor page for route/i)).toBeInTheDocument());
  });

  it('filters the list by the search input', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') return response(DATASOURCE_PAYLOAD);
      if (url === '/api/queries') {
        return response({
          queries: [
            {
              query_id: 'query_one',
              datasource_id: 'ds_1',
              natural_language_prompt: 'Account lookup',
              additional_context: 'Find one account',
              sql_query: 'SELECT * FROM accounts WHERE id = {{account_id}}',
              allows_modification: false,
              parameters: [],
              output_columns: [],
              constraints: '',
            },
            {
              query_id: 'query_two',
              datasource_id: 'ds_1',
              natural_language_prompt: 'Orders overview',
              additional_context: 'Recent orders summary',
              sql_query: 'SELECT * FROM orders',
              allows_modification: false,
              parameters: [],
              output_columns: [],
              constraints: '',
            },
          ],
        });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    await waitFor(() => expect(screen.getByText(/account lookup/i)).toBeInTheDocument());
    expect(screen.getByText(/orders overview/i)).toBeInTheDocument();

    await userEvent.type(screen.getByPlaceholderText(/search prompts/i), 'orders');

    expect(screen.queryByText(/account lookup/i)).not.toBeInTheDocument();
    expect(screen.getByText(/orders overview/i)).toBeInTheDocument();
  });

  it('renders a mutating badge when a query allows modification', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') return response(DATASOURCE_PAYLOAD);
      if (url === '/api/queries') {
        return response({
          queries: [
            {
              query_id: 'query_mutate',
              datasource_id: 'ds_1',
              natural_language_prompt: 'Retire contract',
              additional_context: '',
              sql_query: 'DELETE FROM contracts WHERE id = {{id}} RETURNING id',
              allows_modification: true,
              parameters: [{ name: 'id', type: 'uuid', description: '', required: true, default: null }],
              output_columns: [],
              constraints: '',
            },
          ],
        });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    await waitFor(() => expect(screen.getByText(/mutating/i)).toBeInTheDocument());
  });
});
