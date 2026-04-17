import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { useCallback, useEffect, useState } from 'react';

import { AppShell } from './components/AppShell';
import { ToastProvider } from './components/ui/Toast';
import { getStatus, listQueries } from './services/api';
import type { RuntimeStatus } from './types/datasource';
import DatasourcePage from './pages/DatasourcePage';
import ApprovedQueriesPage from './pages/ApprovedQueriesPage';
import QueryEditorPage from './pages/QueryEditorPage';
import OpenClawPage from './pages/OpenClawPage';

const AGENT_REVEALED_STORAGE_KEY = 'dataclaw:agent-revealed';

function readAgentRevealed(): boolean {
  try {
    return localStorage.getItem(AGENT_REVEALED_STORAGE_KEY) === 'true';
  } catch {
    return false;
  }
}

export interface AppOutletContext {
  refresh: () => Promise<void>;
  markAgentRevealed: () => void;
  resetAgentRevealed: () => void;
}

export default function App(): JSX.Element {
  const [status, setStatus] = useState<RuntimeStatus | null>(null);
  const [queryCount, setQueryCount] = useState(0);
  const [agentRevealed, setAgentRevealed] = useState<boolean>(readAgentRevealed);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [nextStatus, queries] = await Promise.all([
        getStatus(),
        listQueries().catch(() => []),
      ]);
      setStatus(nextStatus);
      setQueryCount(queries.length);
      setAgentRevealed(readAgentRevealed());
    } catch {
      setStatus(null);
    }
  }, []);

  useEffect(() => {
    void (async () => {
      await refresh();
    })();
  }, [refresh]);

  const markAgentRevealed = useCallback((): void => {
    try {
      localStorage.setItem(AGENT_REVEALED_STORAGE_KEY, 'true');
    } catch {
      // ignore storage failures — completion marker is best-effort
    }
    setAgentRevealed(true);
  }, []);

  const resetAgentRevealed = useCallback((): void => {
    try {
      localStorage.removeItem(AGENT_REVEALED_STORAGE_KEY);
    } catch {
      // ignore storage failures — completion marker is best-effort
    }
    setAgentRevealed(false);
  }, []);

  const completion = {
    datasource: Boolean(status?.datasourceConfigured),
    queries: queryCount > 0,
    agent: agentRevealed,
  };

  const outletContext: AppOutletContext = { refresh, markAgentRevealed, resetAgentRevealed };

  return (
    <BrowserRouter>
      <ToastProvider>
        <Routes>
          <Route element={<AppShell status={status} completion={completion} outletContext={outletContext} />}>
            <Route index element={<DatasourcePage />} />
            <Route path="/datasource" element={<DatasourcePage />} />
            <Route path="/queries" element={<ApprovedQueriesPage />} />
            <Route path="/queries/new" element={<QueryEditorPage />} />
            <Route path="/queries/:id" element={<QueryEditorPage />} />
            <Route path="/openclaw" element={<OpenClawPage />} />
            <Route path="*" element={<Navigate to="/datasource" replace />} />
          </Route>
        </Routes>
      </ToastProvider>
    </BrowserRouter>
  );
}
