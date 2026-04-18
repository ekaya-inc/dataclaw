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
    useOutletContext: () => ({ refresh: vi.fn(async () => undefined) }),
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
          <Route path="/queries/:id" element={<div>Detail page for route</div>} />
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
  it('seeds a single example query and lists it on the page', async () => {
    let postCount = 0;
    const seeded: Array<Record<string, unknown>> = [];
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') return response(DATASOURCE_PAYLOAD);
      if (url === '/api/status') return response({ version: 'test', port: 18790, datasource_configured: true });
      if (url === '/api/queries' && !init?.method) return response({ queries: seeded });
      if (url === '/api/queries' && init?.method === 'POST') {
        postCount += 1;
        const body = typeof init.body === 'string' ? (JSON.parse(init.body) as Record<string, unknown>) : {};
        const saved = {
          query_id: `query_seed_${postCount}`,
          datasource_id: 'ds_1',
          natural_language_prompt: body.natural_language_prompt,
          additional_context: body.additional_context,
          sql_query: body.sql_query,
          allows_modification: false,
          parameters: body.parameters ?? [],
          output_columns: body.output_columns ?? [],
          constraints: '',
        };
        seeded.push(saved);
        return response({ query: saved });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    await waitFor(() => expect(screen.getByRole('button', { name: /seed with an example query/i })).toBeEnabled());

    await userEvent.click(screen.getByRole('button', { name: /seed with an example query/i }));

    await waitFor(() => expect(postCount).toBe(1));
    await waitFor(() => expect(screen.getByText(/connectivity check/i)).toBeInTheDocument());
    expect(screen.queryByText(/detail page for route/i)).not.toBeInTheDocument();
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
