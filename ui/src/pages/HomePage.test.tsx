import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi } from 'vitest';

import HomePage from './HomePage';

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function renderHomePage(props: { datasourceConfigured?: boolean; statusLoaded?: boolean } = {}): void {
  render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route
          path="/"
          element={<HomePage datasourceConfigured={props.datasourceConfigured ?? true} statusLoaded={props.statusLoaded ?? true} />}
        />
        <Route path="/datasource" element={<div>Datasource page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('HomePage', () => {
  it('renders the dashboard table with the Agent column and no security UI', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({
        success: true,
        data: {
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
      }),
    );

    renderHomePage();

    await waitFor(() => expect(screen.getByRole('heading', { name: /dashboard/i })).toBeInTheDocument());
    expect(screen.getByText(/monitor mcp tool activity by agent/i)).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /agent/i })).toBeInTheDocument();
    expect(screen.getByText('Marketing bot')).toBeInTheDocument();
    expect(screen.queryByText(/security/i)).not.toBeInTheDocument();
  });

  it('refreshes the dashboard when refresh is clicked', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({ success: true, data: { items: [], total: 0, limit: 50, offset: 0 } }),
    );

    renderHomePage();

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole('button', { name: /refresh/i }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });

  it('shows the empty state when there are no tracked events', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({ success: true, data: { items: [], total: 0, limit: 50, offset: 0 } }),
    );

    renderHomePage();

    await waitFor(() => expect(screen.getByText(/no mcp activity yet/i)).toBeInTheDocument());
    expect(screen.getByText(/tracked mcp tool calls will appear here/i)).toBeInTheDocument();
  });

  it('expands a row to fetch and display request, result, query name, SQL, and error details', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url.startsWith('/api/mcp-events/evt_2')) {
        return jsonResponse({
          success: true,
          data: {
            event: {
              id: 'evt_2',
              request_params: { query_id: 'query_1' },
              result_summary: { row_count: 0 },
              error_message: 'permission denied',
              query_name: 'Top accounts',
              sql_text: 'SELECT account_id FROM accounts',
            },
          },
        });
      }
      return jsonResponse({
        success: true,
        data: {
          items: [
            {
              id: 'evt_2',
              created_at: '2026-04-17T17:00:00Z',
              agent_name: 'Marketing bot',
              tool_name: 'execute_query',
              event_type: 'tool_error',
              was_successful: false,
              duration_ms: 28,
              has_details: true,
            },
          ],
          total: 1,
          limit: 50,
          offset: 0,
        },
      });
    });

    renderHomePage();

    await waitFor(() => expect(screen.getByText('Marketing bot')).toBeInTheDocument());
    await userEvent.click(screen.getByRole('button', { name: /expand details for execute_query/i }));

    await waitFor(() => expect(screen.getByText('Top accounts')).toBeInTheDocument());
    expect(screen.getByText(/request summary/i)).toBeInTheDocument();
    expect(screen.getByText(/result summary/i)).toBeInTheDocument();
    expect(screen.getByText(/permission denied/i)).toBeInTheDocument();
    expect(screen.getByText(/SELECT account_id FROM accounts/i)).toBeInTheDocument();
    expect(screen.getAllByText(/query_1/i).length).toBeGreaterThan(0);
  });

  it('updates request params when filters change', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockResolvedValue(
      jsonResponse({ success: true, data: { items: [], total: 0, limit: 50, offset: 0 } }),
    );

    renderHomePage();

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    await userEvent.click(screen.getByRole('button', { name: /last 24h/i }));
    await userEvent.selectOptions(screen.getByLabelText(/event type/i), 'tool_error');
    await userEvent.type(screen.getByLabelText(/filter by tool/i), 'execute');
    await userEvent.type(screen.getByLabelText(/filter by agent/i), 'Marketing');

    await waitFor(() => {
      const rawUrl = fetchMock.mock.calls[fetchMock.mock.calls.length - 1]?.[0];
      const url = new URL(String(rawUrl), 'http://localhost');
      expect(url.pathname).toBe('/api/mcp-events');
      expect(url.searchParams.get('range')).toBe('24h');
      expect(url.searchParams.get('event_type')).toBe('tool_error');
      expect(url.searchParams.get('tool_name')).toBe('execute');
      expect(url.searchParams.get('agent_name')).toBe('Marketing');
      expect(url.searchParams.get('limit')).toBe('50');
      expect(url.searchParams.get('offset')).toBe('0');
    });
  });
});
