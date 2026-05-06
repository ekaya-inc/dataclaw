import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, vi } from 'vitest';

import App from './App';

type MockRoute =
  | unknown
  | {
      status: number;
      body: unknown;
    };

function isMockResponse(value: MockRoute): value is { status: number; body: unknown } {
  return typeof value === 'object' && value !== null && 'status' in value && 'body' in value;
}

function mockFetch(routes: Record<string, MockRoute>) {
  const allRoutes: Record<string, MockRoute> = {
    '/api/auth/session': { authenticated: true },
    ...routes,
  };
  return vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
    const rawUrl = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    const url = new URL(rawUrl, 'http://localhost');
    const key = `${url.pathname}${url.search}`;
    const route = key in allRoutes ? allRoutes[key] : allRoutes[url.pathname];
    const status = isMockResponse(route) ? route.status : 200;
    const body = isMockResponse(route) ? route.body : (route ?? {});
    return new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    });
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('App shell', () => {
  it('shows the dashboard empty state on / when no datasource is connected', async () => {
    window.history.pushState({}, '', '/');
    mockFetch({
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 2, datasource_configured: false },
      '/api/datasource': { datasource: null },
      '/api/datasource/types': { types: [{ type: 'postgres', display_name: 'PostgreSQL' }, { type: 'mssql', display_name: 'Microsoft SQL Server' }] },
      '/api/queries': { queries: [] },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('heading', { name: /dashboard/i })).toBeInTheDocument());
    await waitFor(() => expect(window.location.pathname).toBe('/'));

    expect(screen.getByText(/start by adding a datasource/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /configure datasource/i })).toBeInTheDocument();

    const agentsLink = screen.getByRole('link', { name: /^agent access$/i });
    expect(agentsLink.querySelector('[aria-label="Completed"]')).toBeNull();
    const queriesLink = screen.getByRole('link', { name: /^approved queries$/i });
    expect(queriesLink.querySelector('[aria-label="Completed"]')).toBeNull();
  });

  it('renders the dashboard on / when a datasource is connected', async () => {
    window.history.pushState({}, '', '/');
    mockFetch({
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 1, datasource_configured: true },
      '/api/queries': { queries: [] },
      '/api/mcp-events': {
        items: [
          {
            id: 'evt_1',
            created_at: '2026-04-17T17:00:00Z',
            agent_name: 'Marketing bot',
            tool_name: 'execute_query',
            event_type: 'tool_call',
            was_successful: true,
            duration_ms: 28,
            request_params: { query_id: 'query_1' },
            result_summary: { row_count: 2 },
            error_message: '',
          },
        ],
        total: 1,
        limit: 50,
        offset: 0,
      },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('heading', { name: /dashboard/i })).toBeInTheDocument());
    await waitFor(() => expect(window.location.pathname).toBe('/'));

    expect(screen.getByText('Marketing bot')).toBeInTheDocument();
    expect(screen.queryByText(/configure the datasource\./i)).not.toBeInTheDocument();
  });

  it('routes the DataClaw heading back to /', async () => {
    window.history.pushState({}, '', '/datasource');
    expect(window.location.pathname).toBe('/datasource');
    mockFetch({
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 0, datasource_configured: true },
      '/api/datasource': {
        datasource: {
          id: 'ds_1',
          type: 'postgres',
          provider: 'postgres',
          display_name: 'dataclaw',
          host: 'db.example.com',
          port: 5432,
          name: 'warehouse',
          user: 'analyst',
          ssl_mode: 'require',
        },
      },
      '/api/datasource/types': { types: [{ type: 'postgres', display_name: 'PostgreSQL' }, { type: 'mssql', display_name: 'Microsoft SQL Server' }] },
      '/api/queries': { queries: [] },
      '/api/mcp-events': { items: [], total: 0, limit: 50, offset: 0 },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('button', { name: /edit display name/i })).toBeInTheDocument());

    await userEvent.click(screen.getByRole('link', { name: /dataclaw/i }));

    await waitFor(() => expect(window.location.pathname).toBe('/'));
    await waitFor(() => expect(screen.queryByRole('button', { name: /edit display name/i })).not.toBeInTheDocument());
  });

  it('routes unauthenticated admin sessions to signin with the current path as next', async () => {
    window.history.pushState({}, '', '/agents');
    mockFetch({
      '/api/auth/session': { status: 401, body: { success: false, error: 'unauthorized' } },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('heading', { name: /sign in to dataclaw/i })).toBeInTheDocument());
    expect(window.location.pathname).toBe('/signin');
    expect(new URLSearchParams(window.location.search).get('next')).toBe('/agents');
  });

  it('signs in with the admin password and does not persist it in localStorage', async () => {
    window.history.pushState({}, '', '/signin?next=%2Fagents');
    const setItemSpy = vi.spyOn(Storage.prototype, 'setItem');
    const fetchMock = mockFetch({
      '/api/auth/session': { status: 401, body: { success: false, error: 'unauthorized' } },
      '/api/auth/signin': { success: true, data: { authenticated: true } },
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 0, datasource_configured: true },
      '/api/queries': { queries: [] },
      '/api/agents': { agents: [] },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('heading', { name: /sign in to dataclaw/i })).toBeInTheDocument());
    await userEvent.type(screen.getByLabelText(/admin password/i), 'admin-password');
    await userEvent.click(screen.getByRole('button', { name: /^sign in$/i }));

    await waitFor(() => expect(window.location.pathname).toBe('/agents'));
    const signinCall = fetchMock.mock.calls.find(([input]) => input === '/api/auth/signin');
    expect(signinCall?.[1]?.credentials).toBe('same-origin');
    expect(JSON.parse(String(signinCall?.[1]?.body))).toEqual({ password: 'admin-password', remember: false });
    expect(setItemSpy).not.toHaveBeenCalledWith(expect.stringMatching(/password|session/i), expect.any(String));
  });

  it('logs out through the auth API and returns to signin', async () => {
    window.history.pushState({}, '', '/');
    const fetchMock = mockFetch({
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 0, datasource_configured: false },
      '/api/queries': { queries: [] },
      '/api/auth/logout': { success: true },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('heading', { name: /dashboard/i })).toBeInTheDocument());
    await userEvent.click(screen.getByRole('button', { name: /sign out/i }));

    await waitFor(() => expect(window.location.pathname).toBe('/signin'));
    const logoutCall = fetchMock.mock.calls.find(([input]) => input === '/api/auth/logout');
    expect(logoutCall?.[1]?.method).toBe('POST');
    expect(logoutCall?.[1]?.credentials).toBe('same-origin');
  });
});
