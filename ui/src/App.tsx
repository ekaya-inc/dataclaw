import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { useCallback, useEffect, useState } from 'react';

import { AppShell } from './components/AppShell';
import { ToastProvider } from './components/ui/Toast';
import DatasourcePage from './pages/DatasourcePage';
import ApprovedQueriesPage from './pages/ApprovedQueriesPage';
import AgentsPage from './pages/AgentsPage';
import QueryEditorPage from './pages/QueryEditorPage';
import { getStatus, listQueries } from './services/api';
import type { RuntimeStatus } from './types/datasource';

export interface AppOutletContext {
  refresh: () => Promise<void>;
}

export default function App(): JSX.Element {
  const [status, setStatus] = useState<RuntimeStatus | null>(null);
  const [queryCount, setQueryCount] = useState(0);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [nextStatus, queries] = await Promise.all([getStatus(), listQueries().catch(() => [])]);
      setStatus(nextStatus);
      setQueryCount(queries.length);
    } catch {
      setStatus(null);
      setQueryCount(0);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void refresh();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [refresh]);

  const completion = {
    datasource: Boolean(status?.datasourceConfigured),
    queries: queryCount > 0,
    agent: (status?.agentCount ?? 0) > 0,
  };

  const outletContext: AppOutletContext = { refresh };

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
            <Route path="/agents" element={<AgentsPage />} />
            <Route path="*" element={<Navigate to="/datasource" replace />} />
          </Route>
        </Routes>
      </ToastProvider>
    </BrowserRouter>
  );
}
