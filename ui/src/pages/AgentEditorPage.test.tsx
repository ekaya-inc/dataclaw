import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import AgentEditorPage from './AgentEditorPage';
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

function DetailStub(): JSX.Element {
  const location = useLocation();
  const state = location.state as { apiKey?: string | null } | null;
  return (
    <div>
      <div>Detail stub</div>
      <div data-testid="detail-state-apikey">{state?.apiKey ?? ''}</div>
    </div>
  );
}

function renderAt(path: string): ReturnType<typeof render> {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <ToastProvider>
        <Routes>
          <Route path="/agents" element={<div>Agents list</div>} />
          <Route path="/agents/new" element={<AgentEditorPage />} />
          <Route path="/agents/:id" element={<DetailStub />} />
          <Route path="/agents/:id/edit" element={<AgentEditorPage />} />
        </Routes>
      </ToastProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('AgentEditorPage', () => {
  it('creates an agent and navigates to the detail route with the plaintext key in state', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries') return jsonResponse({ queries: [] });
      if (url === '/api/agents' && init?.method === 'POST') {
        return jsonResponse(
          {
            agent: {
              id: 'agent_new',
              name: 'New bot',
              masked_api_key: 'dclw-nb••••',
              api_key: 'dclw-new-secret',
              can_query: true,
              can_execute: false,
              can_manage_approved_queries: false,
              approved_query_scope: 'none',
              approved_query_ids: [],
            },
          },
          201,
        );
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt('/agents/new');

    const nameInput = await screen.findByLabelText(/^name$/i);
    await userEvent.type(nameInput, 'New bot');

    const createButton = screen.getByRole('button', { name: /create access point/i });
    await userEvent.click(createButton);

    const postCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'POST');
    expect(JSON.parse(String(postCall?.[1]?.body))).toEqual({
      name: 'New bot',
      can_query: true,
      can_execute: false,
      can_manage_approved_queries: false,
      approved_query_scope: 'none',
      approved_query_ids: [],
    });

    expect(await screen.findByText('Detail stub')).toBeInTheDocument();
    expect(screen.getByTestId('detail-state-apikey')).toHaveTextContent('dclw-new-secret');
  });

  it('forces raw query on while approved-query management is enabled and submits the manager flag', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries') return jsonResponse({ queries: [] });
      if (url === '/api/agents' && init?.method === 'POST') {
        return jsonResponse(
          {
            agent: {
              id: 'agent_manager',
              name: 'Manager bot',
              masked_api_key: 'dclw-mg••••',
              api_key: 'dclw-manager-secret',
              can_query: true,
              can_execute: false,
              can_manage_approved_queries: true,
              approved_query_scope: 'none',
              approved_query_ids: [],
            },
          },
          201,
        );
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt('/agents/new');

    const nameInput = await screen.findByLabelText(/^name$/i);
    const rawQueryCheckbox = screen.getByRole('checkbox', { name: /allow agent to query entire database/i });
    const managerCheckbox = screen.getByRole('checkbox', { name: /allow agent to manage approved queries/i });

    await userEvent.click(rawQueryCheckbox);
    expect(rawQueryCheckbox).not.toBeChecked();

    const learnMoreButton = screen.getByRole('button', { name: /learn more/i });
    await userEvent.click(learnMoreButton);
    expect(screen.getByText(/let an agent build the catalog for you/i)).toBeInTheDocument();
    expect(screen.getByText(/example conversation/i)).toBeInTheDocument();

    await userEvent.click(managerCheckbox);
    expect(managerCheckbox).toBeChecked();
    expect(rawQueryCheckbox).toBeChecked();
    expect(rawQueryCheckbox).toBeDisabled();

    const noneRadio = screen.getByRole('radio', { name: /no approved queries/i });
    const allRadio = screen.getByRole('radio', { name: /all approved queries/i });
    const selectedRadio = screen.getByRole('radio', { name: /selected approved queries/i });
    expect(allRadio).toHaveAttribute('aria-checked', 'true');
    expect(noneRadio).toBeDisabled();
    expect(selectedRadio).toBeDisabled();

    await userEvent.type(nameInput, 'Manager bot');
    await userEvent.click(screen.getByRole('button', { name: /create access point/i }));

    const postCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'POST');
    expect(JSON.parse(String(postCall?.[1]?.body))).toEqual({
      name: 'Manager bot',
      can_query: true,
      can_execute: false,
      can_manage_approved_queries: true,
      approved_query_scope: 'all',
      approved_query_ids: [],
    });
  });

  it('restores prior query permissions after manager mode is toggled back off', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries') {
        return jsonResponse({
          queries: [
            {
              query_id: 'query_1',
              natural_language_prompt: 'Top accounts',
              sql_query: 'SELECT * FROM accounts',
              allows_modification: false,
              parameters: [],
              output_columns: [],
            },
          ],
        });
      }
      if (url === '/api/agents' && init?.method === 'POST') {
        return jsonResponse(
          {
            agent: {
              id: 'agent_restored',
              name: 'Scoped bot',
              masked_api_key: 'dclw-sc••••',
              api_key: 'dclw-scoped-secret',
              can_query: false,
              can_execute: false,
              can_manage_approved_queries: false,
              approved_query_scope: 'selected',
              approved_query_ids: ['query_1'],
            },
          },
          201,
        );
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt('/agents/new');

    const nameInput = await screen.findByLabelText(/^name$/i);
    const rawQueryCheckbox = screen.getByRole('checkbox', { name: /allow agent to query entire database/i });
    const managerCheckbox = screen.getByRole('checkbox', { name: /allow agent to manage approved queries/i });
    const selectedRadio = screen.getByRole('radio', { name: /selected approved queries/i });

    await userEvent.click(rawQueryCheckbox);
    expect(rawQueryCheckbox).not.toBeChecked();

    await userEvent.click(selectedRadio);
    expect(selectedRadio).toHaveAttribute('aria-checked', 'true');

    const selectedQueryCheckbox = await screen.findByRole('checkbox', { name: /top accounts/i });
    await userEvent.click(selectedQueryCheckbox);
    expect(selectedQueryCheckbox).toBeChecked();

    await userEvent.click(managerCheckbox);
    expect(managerCheckbox).toBeChecked();
    expect(rawQueryCheckbox).toBeChecked();

    await userEvent.click(managerCheckbox);
    expect(managerCheckbox).not.toBeChecked();
    expect(rawQueryCheckbox).not.toBeChecked();
    expect(selectedRadio).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByRole('checkbox', { name: /top accounts/i })).toBeChecked();

    await userEvent.type(nameInput, 'Scoped bot');
    await userEvent.click(screen.getByRole('button', { name: /create access point/i }));

    const postCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'POST');
    expect(JSON.parse(String(postCall?.[1]?.body))).toEqual({
      name: 'Scoped bot',
      can_query: false,
      can_execute: false,
      can_manage_approved_queries: false,
      approved_query_scope: 'selected',
      approved_query_ids: ['query_1'],
    });
  });

  it('loads an agent for editing and saves without a name change', async () => {
    const fetchMock = vi.spyOn(global, 'fetch').mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/queries') return jsonResponse({ queries: [] });
      if (url === '/api/agents/agent_1' && !init?.method) {
        return jsonResponse({
          agent: {
            id: 'agent_1',
            name: 'Warehouse analyst',
            masked_api_key: 'dclw-an••••',
            can_query: true,
            can_execute: false,
            can_manage_approved_queries: false,
            approved_query_scope: 'none',
            approved_query_ids: [],
          },
        });
      }
      if (url === '/api/agents/agent_1' && init?.method === 'PUT') {
        return jsonResponse({
          agent: {
            id: 'agent_1',
            name: 'Warehouse analyst',
            masked_api_key: 'dclw-an••••',
            can_query: true,
            can_execute: true,
            can_manage_approved_queries: false,
            approved_query_scope: 'none',
            approved_query_ids: [],
          },
        });
      }
      throw new Error(`Unhandled ${String(url)}`);
    });

    renderAt('/agents/agent_1/edit');

    const nameInput = await screen.findByLabelText(/^name$/i);
    await waitFor(() => expect(nameInput).toHaveValue('Warehouse analyst'));
    expect(nameInput).toHaveAttribute('readonly');

    const executeCheckbox = screen.getByRole('checkbox', { name: /allow agent full write access to the database/i });
    await userEvent.click(executeCheckbox);

    const phraseInput = await screen.findByLabelText(/type enable dangerous execute to confirm/i);
    const confirmButton = screen.getByRole('button', { name: /^enable dangerous execute$/i });
    expect(confirmButton).toBeDisabled();
    await userEvent.type(phraseInput, 'enable dangerous execute');
    expect(confirmButton).toBeEnabled();
    await userEvent.click(confirmButton);
    expect(executeCheckbox).toBeChecked();

    const saveButton = screen.getByRole('button', { name: /save changes/i });
    await userEvent.click(saveButton);

    const putCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'PUT');
    expect(JSON.parse(String(putCall?.[1]?.body))).toEqual({
      name: 'Warehouse analyst',
      can_query: true,
      can_execute: true,
      can_manage_approved_queries: false,
      approved_query_scope: 'none',
      approved_query_ids: [],
    });

    expect(await screen.findByText('Detail stub')).toBeInTheDocument();
  });
});
