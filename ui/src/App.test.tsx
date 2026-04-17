import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, vi } from 'vitest';

import App from './App';

function mockFetch(routes: Record<string, unknown>): void {
  vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
    const body = routes[url];
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('App shell', () => {
  it('redirects / to /datasource when no datasource is connected', async () => {
    window.history.pushState({}, '', '/');
    mockFetch({
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 0, datasource_configured: false },
      '/api/datasource': { datasource: null },
      '/api/datasource/types': { types: [{ type: 'postgres', display_name: 'PostgreSQL' }, { type: 'mssql', display_name: 'Microsoft SQL Server' }] },
      '/api/queries': { queries: [] },
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('link', { name: /datasource/i })).toBeInTheDocument();
    });
    await waitFor(() => expect(screen.getByText(/choose a datasource type to connect/i)).toBeInTheDocument());

    expect(screen.getByRole('link', { name: /^approved queries$/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /^agents$/i })).toBeInTheDocument();
    expect(screen.queryByText(/schema/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/ontology/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/local api/i)).not.toBeInTheDocument();
    expect(screen.getByText(/connect local agents to your data with explicit, trackable access controls/i)).toBeInTheDocument();
  });

  it('keeps / blank when a datasource is connected', async () => {
    window.history.pushState({}, '', '/');
    mockFetch({
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 0, datasource_configured: true },
      '/api/queries': { queries: [] },
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('link', { name: /dataclaw/i })).toBeInTheDocument());
    await waitFor(() => expect(window.location.pathname).toBe('/'));

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
    });

    render(<App />);

    await waitFor(() => expect(screen.getByRole('button', { name: /edit display name/i })).toBeInTheDocument());

    await userEvent.click(screen.getByRole('link', { name: /dataclaw/i }));

    await waitFor(() => expect(window.location.pathname).toBe('/'));
    await waitFor(() => expect(screen.queryByRole('button', { name: /edit display name/i })).not.toBeInTheDocument());
  });
});
