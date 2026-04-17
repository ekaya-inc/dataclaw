import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Link, MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import QueryDetailPage from './QueryDetailPage';
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

const QUERY = {
  query_id: 'query_1',
  datasource_id: 'ds_1',
  natural_language_prompt: 'Connectivity check',
  additional_context: 'Verify the datasource is reachable by returning a simple boolean.',
  sql_query: 'SELECT true AS connected',
  allows_modification: false,
  parameters: [{ name: 'account_id', type: 'uuid', description: 'Account id', required: true, default: null }],
  output_columns: [{ name: 'connected', type: 'boolean', description: 'True when the datasource responds.' }],
  constraints: 'Only run for smoke tests.',
  updated_at: '2026-04-18T09:30:00Z',
};

function renderAt(entries: string[]): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={entries}>
      <Link to="/other">Go to other route</Link>
      <ToastProvider>
        <Routes>
          <Route path="/queries" element={<div>Queries list</div>} />
          <Route path="/queries/:id" element={<QueryDetailPage />} />
          <Route path="/queries/:id/edit" element={<div>Edit query route</div>} />
          <Route path="/other" element={<div>Other route</div>} />
        </Routes>
      </ToastProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('QueryDetailPage', () => {
  it('renders the saved query read-only and routes to edit', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries/query_1') return jsonResponse({ query: QUERY });
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt(['/queries/query_1']);

    await waitFor(() => expect(screen.getByRole('heading', { name: /connectivity check/i })).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /^execute saved query$/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /save changes/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /test draft query/i })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /^edit$/i }));

    expect(await screen.findByText(/edit query route/i)).toBeInTheDocument();
  });

  it('executes the saved query from the detail page', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries/query_1' && !init?.method) return jsonResponse({ query: QUERY });
      if (url === '/api/queries/query_1/execute' && init?.method === 'POST') {
        return jsonResponse({
          result: {
            columns: [{ name: 'connected', type: 'boolean' }],
            rows: [{ connected: true }],
            row_count: 1,
          },
        });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt(['/queries/query_1']);

    const parameterInput = await screen.findByLabelText(/account_id/i);
    await userEvent.type(parameterInput, '550e8400-e29b-41d4-a716-446655440000');
    await userEvent.click(screen.getByRole('button', { name: /^execute saved query$/i }));

    await waitFor(() => expect(screen.getByText(/returned 1 rows\./i)).toBeInTheDocument());
    expect(screen.getByText('true')).toBeInTheDocument();

    const executeCall = fetchMock.mock.calls.find(([input]) => String(input).includes('/api/queries/query_1/execute'));
    expect(JSON.parse(String(executeCall?.[1]?.body))).toEqual({
      parameters: { account_id: '550e8400-e29b-41d4-a716-446655440000' },
      limit: 100,
    });
  });

  it('deletes the saved query from the detail page', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries/query_1' && !init?.method) return jsonResponse({ query: QUERY });
      if (url === '/api/queries/query_1' && init?.method === 'DELETE') return jsonResponse({ deleted: true });
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt(['/queries/query_1']);

    await userEvent.click(await screen.findByRole('button', { name: /^delete$/i }));
    await userEvent.click(screen.getByRole('button', { name: /delete query/i }));

    expect(await screen.findByText(/queries list/i)).toBeInTheDocument();
  });

  it('ignores late load failures after the user navigates away', async () => {
    let rejectQueryLoad: ((reason?: unknown) => void) | undefined;

    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries/query_1') {
        return await new Promise<Response>((_, reject) => {
          rejectQueryLoad = reject;
        });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt(['/queries/query_1']);

    await userEvent.click(screen.getByRole('link', { name: /go to other route/i }));
    expect(await screen.findByText(/^Other route$/i)).toBeInTheDocument();

    await act(async () => {
      rejectQueryLoad?.(new Error('load failed'));
      await Promise.resolve();
    });

    expect(screen.getByText(/^Other route$/i)).toBeInTheDocument();
    expect(screen.queryByText(/queries list/i)).not.toBeInTheDocument();
  });

  it('disables delete while execution is in flight', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries/query_1' && !init?.method) return jsonResponse({ query: QUERY });
      if (url === '/api/queries/query_1/execute' && init?.method === 'POST') {
        return await new Promise<Response>(() => undefined);
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt(['/queries/query_1']);

    const parameterInput = await screen.findByLabelText(/account_id/i);
    await userEvent.type(parameterInput, '550e8400-e29b-41d4-a716-446655440000');
    await userEvent.click(screen.getByRole('button', { name: /^execute saved query$/i }));

    await waitFor(() => expect(screen.getByRole('button', { name: /^delete$/i })).toBeDisabled());
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
