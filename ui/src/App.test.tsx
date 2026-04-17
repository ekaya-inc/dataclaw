import { render, screen, waitFor } from '@testing-library/react';
import { vi } from 'vitest';

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

describe('App shell', () => {
  it('renders exactly the three nav screens and lands in the app shell without auth', async () => {
    window.history.pushState({}, '', '/');
    mockFetch({
      '/api/status': { port: 18790, base_url: 'http://127.0.0.1:18790', agent_count: 0 },
      '/api/datasource': { datasource: null },
      '/api/datasource/types': { types: [{ type: 'postgres', display_name: 'PostgreSQL' }, { type: 'mssql', display_name: 'Microsoft SQL Server' }] },
      '/api/queries': { queries: [] },
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('link', { name: /datasource/i })).toBeInTheDocument();
    });

    expect(screen.getByRole('link', { name: /^approved queries$/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /^agents$/i })).toBeInTheDocument();
    expect(screen.queryByText(/schema/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/ontology/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/local api/i)).not.toBeInTheDocument();
    expect(screen.getByText(/connect local agents to your data with explicit, testable access controls/i)).toBeInTheDocument();
  });
});
