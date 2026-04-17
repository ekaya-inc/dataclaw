import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import QueryEditorPage from './QueryEditorPage';
import { ToastProvider } from '../components/ui/Toast';

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof ReactRouterDom>('react-router-dom');
  return {
    ...actual,
    useOutletContext: () => ({ refresh: vi.fn(async () => undefined) }),
  };
});

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

const DATASOURCE = {
  datasource: {
    id: 'ds_1',
    type: 'postgres',
    provider: 'postgres',
    display_name: 'Primary datasource',
    sql_dialect: 'PostgreSQL',
  },
};

const QUERY = {
  query_id: 'query_1',
  datasource_id: 'ds_1',
  natural_language_prompt: 'Connectivity check',
  additional_context: 'Verify the datasource is reachable by returning a simple boolean.',
  sql_query: 'SELECT true AS connected',
  allows_modification: false,
  parameters: [],
  output_columns: [{ name: 'connected', type: 'boolean', description: 'True when the datasource responds.' }],
  constraints: 'Only run for smoke tests.',
};

function renderAt(entry: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <ToastProvider>
        <Routes>
          <Route path="/queries" element={<div>Queries list</div>} />
          <Route path="/queries/new" element={<QueryEditorPage />} />
          <Route path="/queries/:id" element={<div>Query detail route</div>} />
          <Route path="/queries/:id/edit" element={<QueryEditorPage />} />
        </Routes>
      </ToastProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('QueryEditorPage', () => {
  it('creates a query and redirects to the detail route', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') return jsonResponse(DATASOURCE);
      if (url === '/api/queries/validate') return jsonResponse({ valid: true });
      if (url === '/api/queries' && init?.method === 'POST') {
        return jsonResponse({ query: { ...QUERY, query_id: 'query_created' } });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt('/queries/new');

    expect(screen.queryByRole('button', { name: /^execute saved query$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /delete query/i })).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText(/natural language prompt/i), 'Customer connectivity check');
    await userEvent.click(screen.getByRole('button', { name: /^create query$/i }));

    expect(await screen.findByText(/query detail route/i)).toBeInTheDocument();
  });

  it('loads an existing query for edit and cancels back to the detail route', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') return jsonResponse(DATASOURCE);
      if (url === '/api/queries/query_1') return jsonResponse({ query: QUERY });
      if (url === '/api/queries/validate') return jsonResponse({ valid: true });
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt('/queries/query_1/edit');

    await waitFor(() => expect(screen.getByDisplayValue('Connectivity check')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /test draft query/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^execute saved query$/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /delete query/i })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /^cancel$/i }));

    expect(await screen.findByText(/query detail route/i)).toBeInTheDocument();
  });

  it('saves edits and redirects back to the detail route', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource') return jsonResponse(DATASOURCE);
      if (url === '/api/queries/query_1' && !init?.method) return jsonResponse({ query: QUERY });
      if (url === '/api/queries/query_1' && init?.method === 'PUT') {
        return jsonResponse({
          query: {
            ...QUERY,
            natural_language_prompt: 'Connectivity check updated',
          },
        });
      }
      if (url === '/api/queries/validate') return jsonResponse({ valid: true });
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt('/queries/query_1/edit');

    const promptInput = await screen.findByLabelText(/natural language prompt/i);
    await userEvent.clear(promptInput);
    await userEvent.type(promptInput, 'Connectivity check updated');
    await userEvent.click(screen.getByRole('button', { name: /save changes/i }));

    expect(await screen.findByText(/query detail route/i)).toBeInTheDocument();
  });
});
