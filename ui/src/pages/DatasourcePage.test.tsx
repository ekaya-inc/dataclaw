import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';

import DatasourcePage from './DatasourcePage';

function response(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('DatasourcePage', () => {
  it('shows the empty state and saves a datasource', async () => {
    const fetchMock = vi.spyOn(global, 'fetch');
    fetchMock.mockImplementation(async (input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/datasource' && !init?.method) return response({ datasource: null });
      if (url === '/api/datasource/test') return response({ success: true, message: 'Connected' });
      if (url === '/api/datasource' && init?.method === 'PUT') return response({ datasource: { id: 'ds_1', type: 'postgres', display_name: 'Primary datasource', host: 'db.example.com', port: 5432, name: 'warehouse', user: 'analyst', ssl_mode: 'require' } });
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<DatasourcePage />);

    await waitFor(() => expect(screen.getByText(/save datasource/i)).toBeInTheDocument());

    await userEvent.type(screen.getByLabelText(/host/i), 'db.example.com');
    await userEvent.type(screen.getByLabelText(/^database$/i, { selector: 'input' }), 'warehouse');
    await userEvent.type(screen.getByLabelText(/username/i), 'analyst');
    await userEvent.click(screen.getByRole('button', { name: /test connection/i }));

    await waitFor(() => expect(screen.getByText(/connected/i)).toBeInTheDocument());

    await userEvent.click(screen.getByRole('button', { name: /save datasource/i }));
    await waitFor(() => expect(screen.getByText(/datasource saved/i)).toBeInTheDocument());
  });
});
