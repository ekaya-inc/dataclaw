import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import AgentsPage from './AgentsPage';
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

function renderPage(): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={['/agents']}>
      <ToastProvider>
        <Routes>
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="/agents/new" element={<div>New agent route</div>} />
          <Route path="/agents/:id" element={<div>Detail route</div>} />
          <Route path="/agents/:id/edit" element={<div>Edit route</div>} />
        </Routes>
      </ToastProvider>
    </MemoryRouter>,
  );
}

function statusResponse(agentCount = 1): Response {
  return jsonResponse({
    port: 18791,
    base_url: 'http://127.0.0.1:18791',
    mcp_url: 'http://127.0.0.1:18791/mcp',
    datasource_configured: true,
    agent_count: agentCount,
  });
}

function warehouseAgent(overrides: Partial<Record<string, unknown>> = {}): Record<string, unknown> {
  return {
    id: 'agent_1',
    name: 'Warehouse analyst',
    masked_api_key: 'dclw-an••••',
    can_query: true,
    can_execute: false,
    approved_query_scope: 'selected',
    approved_query_ids: ['query_1'],
    created_at: '2026-03-01T12:00:00Z',
    last_used_at: null,
    ...overrides,
  };
}

beforeEach(() => {
  vi.restoreAllMocks();
  Object.defineProperty(globalThis.navigator, 'clipboard', {
    value: { writeText: vi.fn(async () => undefined) },
    configurable: true,
  });
});

describe('AgentsPage', () => {
  it('blocks the screen with a Configure datasource CTA when no datasource exists', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') {
        return jsonResponse({
          port: 18791,
          datasource_configured: false,
          agent_count: 0,
        });
      }
      if (url === '/api/agents') return jsonResponse({ agents: [] });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    await waitFor(() =>
      expect(screen.getByText(/start by adding a datasource/i)).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: /configure datasource/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /new agent/i })).not.toBeInTheDocument();
  });

  it('shows the empty state when no agents exist', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') return statusResponse(0);
      if (url === '/api/agents') return jsonResponse({ agents: [] });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    await waitFor(() =>
      expect(screen.getByText(/no agents yet/i)).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: /new agent/i })).toBeInTheDocument();
  });

  it('renders the agent row with tool pills', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') return statusResponse();
      if (url === '/api/agents') return jsonResponse({ agents: [warehouseAgent()] });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    await waitFor(() => expect(screen.getByText('Warehouse analyst')).toBeInTheDocument());
    expect(screen.getByText('query')).toBeInTheDocument();
    expect(screen.getAllByText('1 query').length).toBeGreaterThanOrEqual(1);
  });

  it('navigates to /agents/new when New agent is clicked', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') return statusResponse(0);
      if (url === '/api/agents') return jsonResponse({ agents: [] });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    const newAgentButton = await screen.findByRole('button', { name: /new agent/i });
    await userEvent.click(newAgentButton);

    expect(await screen.findByText('New agent route')).toBeInTheDocument();
  });

  it('navigates to the detail route on row click', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') return statusResponse();
      if (url === '/api/agents') return jsonResponse({ agents: [warehouseAgent()] });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    renderPage();

    const row = await screen.findByText('Warehouse analyst');
    await userEvent.click(row);

    expect(await screen.findByText('Detail route')).toBeInTheDocument();
  });

  it('requires typing "delete agent" to enable deletion', async () => {
    let agents: Array<Record<string, unknown>> = [warehouseAgent()];
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') return statusResponse(agents.length);
      if (url === '/api/agents' && !init?.method) return jsonResponse({ agents });
      if (url === '/api/agents/agent_1' && init?.method === 'DELETE') {
        agents = [];
        return jsonResponse({ deleted: true });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderPage();

    await screen.findByText('Warehouse analyst');
    const deleteTrigger = screen.getByRole('button', { name: /delete warehouse analyst/i });
    await userEvent.click(deleteTrigger);

    const dialog = await screen.findByRole('dialog');
    const confirmInput = within(dialog).getByLabelText(/type .* to confirm/i);
    const deleteButton = within(dialog).getByRole('button', { name: /delete agent/i });

    expect(deleteButton).toBeDisabled();
    await userEvent.type(confirmInput, 'delete agent');
    expect(deleteButton).toBeEnabled();
    await userEvent.click(deleteButton);

    await waitFor(() => expect(screen.getByText(/no agents yet/i)).toBeInTheDocument());
  });
});
