import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import DatasourcePage from './DatasourcePage';

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof ReactRouterDom>('react-router-dom');
  return {
    ...actual,
    useOutletContext: () => ({ refresh: vi.fn(async () => undefined), markAgentRevealed: vi.fn(), resetAgentRevealed: vi.fn() }),
  };
});

function response(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('DatasourcePage', () => {
  it('gates save on a successful test connection and saves a datasource', async () => {
    const fetchMock = vi.spyOn(global, 'fetch');
    fetchMock.mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource' && !init?.method) return response({ datasource: null });
      if (url === '/api/datasource/test') return response({ success: true, message: 'Connected' });
      if (url === '/api/datasource' && init?.method === 'PUT') return response({ datasource: { id: 'ds_1', type: 'postgres', display_name: 'dataclaw', host: 'db.example.com', port: 5432, name: 'warehouse', user: 'analyst', ssl_mode: 'require' } });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<DatasourcePage />);

    const saveButton = await screen.findByRole('button', { name: /save datasource/i });
    expect(saveButton).toBeDisabled();
    expect(screen.getByText(/run test connection successfully to enable saving/i)).toBeInTheDocument();
    expect(screen.getByText(/configure the datasource\./i)).toBeInTheDocument();
    expect(screen.queryByText(/this name is used for the mcp server/i)).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText(/host/i), 'db.example.com');
    await userEvent.type(screen.getByLabelText(/database name/i), 'warehouse');
    await userEvent.type(screen.getByLabelText(/username/i), 'analyst');

    expect(saveButton).toBeDisabled();

    await userEvent.click(screen.getByRole('button', { name: /test connection/i }));
    await waitFor(() => expect(screen.getByText(/connected/i)).toBeInTheDocument());
    await waitFor(() => expect(saveButton).toBeEnabled());

    await userEvent.click(saveButton);
    await waitFor(() => expect(screen.getByText(/datasource saved/i)).toBeInTheDocument());
  });

  it('resets the save gate when a connection field changes after a successful test', async () => {
    const fetchMock = vi.spyOn(global, 'fetch');
    fetchMock.mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource' && !init?.method) return response({ datasource: null });
      if (url === '/api/datasource/test') return response({ success: true, message: 'Connected' });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<DatasourcePage />);

    const saveButton = await screen.findByRole('button', { name: /save datasource/i });
    await userEvent.type(screen.getByLabelText(/host/i), 'db.example.com');
    await userEvent.type(screen.getByLabelText(/database name/i), 'warehouse');
    await userEvent.click(screen.getByRole('button', { name: /test connection/i }));

    await waitFor(() => expect(saveButton).toBeEnabled());

    await userEvent.type(screen.getByLabelText(/host/i), 'x');
    expect(saveButton).toBeDisabled();
  });

  it('locks connection fields when a datasource already exists and auto-saves display name on blur', async () => {
    const savedDatasource = {
      id: 'ds_1',
      type: 'postgres',
      provider: 'postgres',
      display_name: 'dataclaw',
      host: 'db.example.com',
      port: 5432,
      name: 'warehouse',
      user: 'analyst',
      password: 'secret',
      ssl_mode: 'require',
    };
    const fetchMock = vi.spyOn(global, 'fetch');
    fetchMock.mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource' && !init?.method) {
        return response({ datasource: savedDatasource });
      }
      if (url === '/api/datasource' && init?.method === 'PUT') {
        return response({ datasource: { ...savedDatasource, display_name: 'renamed' } });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<DatasourcePage />);

    await waitFor(() => expect(screen.getByRole('button', { name: /edit display name/i })).toBeInTheDocument());

    expect(screen.getByLabelText(/datasource type/i)).toBeDisabled();
    expect(screen.getByLabelText(/database name/i)).toHaveAttribute('readonly');
    expect(screen.getByLabelText(/host/i)).toHaveAttribute('readonly');
    expect(screen.getByLabelText(/port/i)).toHaveAttribute('readonly');
    expect(screen.getByLabelText(/username/i)).toHaveAttribute('readonly');
    expect(screen.getByLabelText(/password/i)).toHaveAttribute('readonly');
    expect(screen.getByLabelText(/ssl mode/i)).toBeDisabled();
    expect(screen.queryByRole('button', { name: /save display name/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /save datasource/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/this name is used for the mcp server/i)).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /edit display name/i }));
    const input = screen.getByLabelText(/display name/i);
    expect(input).toHaveValue('dataclaw');

    await userEvent.clear(input);
    await userEvent.type(input, 'renamed');
    await userEvent.tab();

    await waitFor(() => expect(screen.getByText(/display name updated/i)).toBeInTheDocument());
  });

  it('requires typed confirmation before disconnecting the datasource', async () => {
    let deleteCalled = false;
    const fetchMock = vi.spyOn(global, 'fetch');
    fetchMock.mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource' && !init?.method) {
        return response({
          datasource: {
            id: 'ds_1',
            type: 'postgres',
            provider: 'postgres',
            display_name: 'dataclaw',
            host: 'db.example.com',
            port: 5432,
            name: 'warehouse',
            user: 'analyst',
            password: 'secret',
            ssl_mode: 'require',
          },
        });
      }
      if (url === '/api/datasource' && init?.method === 'DELETE') {
        deleteCalled = true;
        return response({ deleted: true });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<DatasourcePage />);

    const openDialog = await screen.findByRole('button', { name: /disconnect datasource/i });
    await userEvent.click(openDialog);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent(/clears all saved approved queries/i);

    const confirmInput = within(dialog).getByLabelText(/type disconnect datasource to confirm/i);
    const dialogConfirmButton = within(dialog).getByRole('button', { name: /disconnect datasource/i });
    expect(dialogConfirmButton).toBeDisabled();

    await userEvent.type(confirmInput, 'wrong text');
    expect(dialogConfirmButton).toBeDisabled();
    expect(deleteCalled).toBe(false);

    await userEvent.clear(confirmInput);
    await userEvent.type(confirmInput, 'disconnect datasource');
    expect(dialogConfirmButton).toBeEnabled();

    await userEvent.click(dialogConfirmButton);
    await waitFor(() => expect(deleteCalled).toBe(true));
    await waitFor(() => expect(screen.getByText(/datasource disconnected/i)).toBeInTheDocument());
  });
});
