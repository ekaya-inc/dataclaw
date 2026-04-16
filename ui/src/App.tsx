import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { useEffect, useState } from 'react';

import { AppShell } from './components/AppShell';
import { getStatus } from './services/api';
import type { RuntimeStatus } from './types/datasource';
import DatasourcePage from './pages/DatasourcePage';
import ApprovedQueriesPage from './pages/ApprovedQueriesPage';
import OpenClawPage from './pages/OpenClawPage';

export default function App(): JSX.Element {
  const [status, setStatus] = useState<RuntimeStatus | null>(null);

  useEffect(() => {
    void (async () => {
      try {
        setStatus(await getStatus());
      } catch {
        setStatus(null);
      }
    })();
  }, []);

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppShell status={status} />}>
          <Route index element={<DatasourcePage />} />
          <Route path="/datasource" element={<DatasourcePage />} />
          <Route path="/queries" element={<ApprovedQueriesPage />} />
          <Route path="/openclaw" element={<OpenClawPage />} />
          <Route path="*" element={<Navigate to="/datasource" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
