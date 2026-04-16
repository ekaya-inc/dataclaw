import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';

import type * as ReactRouterDom from 'react-router-dom';

import OpenClawPage from './OpenClawPage';

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

describe('OpenClawPage', () => {
  it('renders one key and the OpenClaw setup command', async () => {
    const fetchMock = vi.spyOn(global, 'fetch');
    fetchMock.mockImplementation(async (input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.pathname : input.url;
      if (url === '/api/status') return response({ port: 18791, base_url: 'http://127.0.0.1:18791' });
      if (url === '/api/openclaw') {
        return response({
          api_key: 'dc_live_secret',
          masked_api_key: 'dc_live_••••',
          openclaw_cli:
            'openclaw mcp set dataclaw \'{"url":"http://127.0.0.1:18791/mcp","transport":"streamable-http","headers":{"Authorization":"Bearer ${DATACLAW_API_KEY}"}}\'',
        });
      }
      throw new Error(`Unhandled request: ${String(url)}`);
    });

    render(<OpenClawPage />);

    await waitFor(() => expect(screen.getByText(/single api key/i)).toBeInTheDocument());
    expect(screen.getByText(/\$\{DATACLAW_API_KEY\}/)).toBeInTheDocument();
    expect(screen.queryByText(/Bearer dc_live_secret/)).not.toBeInTheDocument();
    expect(screen.getAllByText(/18791\/mcp/i).length).toBeGreaterThan(0);

    await userEvent.click(screen.getByRole('button', { name: /reveal/i }));
    expect(screen.getByText('dc_live_secret')).toBeInTheDocument();
  });
});
