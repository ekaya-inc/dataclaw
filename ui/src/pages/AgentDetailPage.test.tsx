import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import AgentDetailPage, { endpointUrl } from './AgentDetailPage';
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

const AGENT = {
  id: 'agent_1',
  name: 'Warehouse analyst',
  masked_api_key: 'dclw-an••••',
  can_query: true,
  can_execute: false,
  approved_query_scope: 'selected',
  approved_query_ids: ['query_1'],
  created_at: '2026-03-01T12:00:00Z',
  last_used_at: null,
};

function renderAt(
  entries: Array<string | { pathname: string; state: unknown }>,
): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={entries}>
      <ToastProvider>
        <Routes>
          <Route path="/agents" element={<div>Agents list</div>} />
          <Route path="/agents/:id" element={<AgentDetailPage />} />
          <Route path="/agents/:id/edit" element={<div>Edit route</div>} />
        </Routes>
      </ToastProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  Object.defineProperty(globalThis.navigator, 'clipboard', {
    value: { writeText: vi.fn(async () => undefined) },
    configurable: true,
  });
});

describe('AgentDetailPage', () => {
  it('prefers the current browser origin for MCP instructions', () => {
    expect(
      endpointUrl(
        {
          port: 18790,
          baseUrl: 'http://127.0.0.1:18790',
          mcpUrl: 'http://127.0.0.1:18790/mcp',
        },
        'http://127.0.0.1:18791',
      ),
    ).toBe('http://127.0.0.1:18791/mcp');
  });

  it('reveals the plaintext API key from router state on first load', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/agents/agent_1') return jsonResponse({ agent: AGENT });
      if (url === '/api/status') {
        return jsonResponse({
          port: 18791,
          mcp_url: 'http://127.0.0.1:18791/mcp',
          datasource_configured: true,
          agent_count: 1,
        });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt([{ pathname: '/agents/agent_1', state: { apiKey: 'dclw-fresh-secret' } }]);

    await waitFor(() => {
      expect(screen.getByDisplayValue('dclw-fresh-secret')).toBeInTheDocument();
    });
  });

  it('reveals the API key on demand via the reveal-key endpoint', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/agents/agent_1' && !init?.method) return jsonResponse({ agent: AGENT });
      if (url === '/api/status') {
        return jsonResponse({
          port: 18791,
          mcp_url: 'http://127.0.0.1:18791/mcp',
          datasource_configured: true,
          agent_count: 1,
        });
      }
      if (url === '/api/agents/agent_1/reveal-key' && init?.method === 'POST') {
        return jsonResponse({
          agent: { ...AGENT, api_key: 'dclw-revealed' },
        });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt(['/agents/agent_1']);

    await waitFor(() => expect(screen.getByText('Warehouse analyst')).toBeInTheDocument());

    const revealButton = screen.getByRole('button', { name: /reveal key/i });
    await userEvent.click(revealButton);

    await waitFor(() => {
      expect(screen.getByDisplayValue('dclw-revealed')).toBeInTheDocument();
    });
  });

  it('navigates to the edit route when Edit is clicked', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/agents/agent_1') return jsonResponse({ agent: AGENT });
      if (url === '/api/status') {
        return jsonResponse({
          port: 18791,
          mcp_url: 'http://127.0.0.1:18791/mcp',
          datasource_configured: true,
          agent_count: 1,
        });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt(['/agents/agent_1']);

    const editButton = await screen.findByRole('button', { name: /^edit$/i });
    await userEvent.click(editButton);

    expect(await screen.findByText('Edit route')).toBeInTheDocument();
  });
});
