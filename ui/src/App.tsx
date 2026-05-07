import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { useCallback, useEffect, useMemo, useState } from 'react';

import { AppShell } from './components/AppShell';
import { ToastProvider } from './components/ui/Toast';
import HomePage from './pages/HomePage';
import DatasourcePage from './pages/DatasourcePage';
import ApprovedQueriesPage from './pages/ApprovedQueriesPage';
import AgentDetailPage from './pages/AgentDetailPage';
import AgentEditorPage from './pages/AgentEditorPage';
import AgentsPage from './pages/AgentsPage';
import QueryDetailPage from './pages/QueryDetailPage';
import QueryEditorPage from './pages/QueryEditorPage';
import SettingsPage from './pages/SettingsPage';
import SignInPage from './pages/SignInPage';
import SupportPage from './pages/SupportPage';
import { AUTH_UNAUTHORIZED_EVENT, getAuthSession, getStatus, listQueries, logout } from './services/api';
import type { RuntimeStatus } from './types/datasource';

export interface AppOutletContext {
  refresh: () => Promise<void>;
}

type AuthState = 'checking' | 'authenticated' | 'unauthenticated';

function currentPath(location: ReturnType<typeof useLocation>): string {
  return `${location.pathname}${location.search}${location.hash}`;
}

function signinPathFor(location: ReturnType<typeof useLocation>): string {
  const next = currentPath(location);
  if (location.pathname === '/signin') return '/signin';
  return `/signin?next=${encodeURIComponent(next)}`;
}

function safeNextPath(search: string): string {
  const params = new URLSearchParams(search);
  const value = params.get('next') ?? params.get('redirect');
  if (!value || !value.startsWith('/') || value.startsWith('//')) return '/';
  return value;
}

function LoadingScreen(): JSX.Element {
  return (
    <main className="flex min-h-screen items-center justify-center bg-surface-secondary px-4 text-sm text-text-secondary">
      Checking admin session…
    </main>
  );
}

function AppRoutes(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  const [authState, setAuthState] = useState<AuthState>('checking');

  const signedOutPath = useMemo(() => signinPathFor(location), [location]);

  const markSignedOut = useCallback((includeNext = true): void => {
    setAuthState('unauthenticated');
    const destination = includeNext ? signinPathFor(location) : '/signin';
    if (location.pathname !== '/signin') {
      navigate(destination, { replace: true });
    }
  }, [location, navigate]);

  const markSignedIn = useCallback((): void => {
    setAuthState('authenticated');
  }, []);

  useEffect(() => {
    const handleUnauthorized = (): void => markSignedOut(true);
    window.addEventListener(AUTH_UNAUTHORIZED_EVENT, handleUnauthorized);
    return () => window.removeEventListener(AUTH_UNAUTHORIZED_EVENT, handleUnauthorized);
  }, [markSignedOut]);

  useEffect(() => {
    let cancelled = false;
    const checkSession = async (): Promise<void> => {
      try {
        const session = await getAuthSession();
        if (!cancelled) setAuthState(session.authenticated ? 'authenticated' : 'unauthenticated');
      } catch {
        if (!cancelled) setAuthState('unauthenticated');
      }
    };
    void checkSession();
    return () => {
      cancelled = true;
    };
  }, []);

  if (authState === 'checking') return <LoadingScreen />;

  if (authState === 'unauthenticated') {
    return (
      <Routes>
        <Route path="/signin" element={<SignInPage onSignedIn={markSignedIn} />} />
        <Route path="*" element={<Navigate to={signedOutPath} replace />} />
      </Routes>
    );
  }

  if (location.pathname === '/signin') {
    return <Navigate to={safeNextPath(location.search)} replace />;
  }

  return <AuthenticatedApp onSignedOut={() => markSignedOut(false)} />;
}

function AuthenticatedApp({ onSignedOut }: { onSignedOut: () => void }): JSX.Element {
  const [status, setStatus] = useState<RuntimeStatus | null>(null);
  const [statusLoaded, setStatusLoaded] = useState(false);
  const [queryCount, setQueryCount] = useState(0);

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [nextStatus, queries] = await Promise.all([getStatus(), listQueries().catch(() => [])]);
      setStatus(nextStatus);
      setQueryCount(queries.length);
    } catch {
      setStatus(null);
      setQueryCount(0);
    } finally {
      setStatusLoaded(true);
    }
  }, []);

  const handleLogout = useCallback(async (): Promise<void> => {
    try {
      await logout();
    } finally {
      onSignedOut();
    }
  }, [onSignedOut]);
  const handleLogoutClick = useCallback((): void => {
    void handleLogout();
  }, [handleLogout]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void refresh();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [refresh]);

  const datasourceConfigured = Boolean(status?.datasourceConfigured);
  const completion = {
    datasource: datasourceConfigured,
    queries: datasourceConfigured && queryCount > 0,
    agent: datasourceConfigured && (status?.agentCount ?? 0) > 0,
  };

  const outletContext: AppOutletContext = { refresh };

  return (
    <Routes>
      <Route
        element={
          <AppShell
            status={status}
            completion={completion}
            outletContext={outletContext}
            onLogout={handleLogoutClick}
          />
        }
      >
        <Route index element={<HomePage datasourceConfigured={status?.datasourceConfigured} statusLoaded={statusLoaded} />} />
        <Route path="/datasource" element={<DatasourcePage />} />
        <Route path="/queries" element={<ApprovedQueriesPage />} />
        <Route path="/queries/new" element={<QueryEditorPage />} />
        <Route path="/queries/:id/edit" element={<QueryEditorPage />} />
        <Route path="/queries/:id" element={<QueryDetailPage />} />
        <Route path="/agents" element={<AgentsPage />} />
        <Route path="/agents/new" element={<AgentEditorPage />} />
        <Route path="/agents/:id" element={<AgentDetailPage />} />
        <Route path="/agents/:id/edit" element={<AgentEditorPage />} />
        <Route path="/settings" element={<SettingsPage status={status} onLogout={handleLogoutClick} />} />
        <Route path="/support" element={<SupportPage />} />
        <Route path="*" element={<Navigate to="/datasource" replace />} />
      </Route>
    </Routes>
  );
}

export default function App(): JSX.Element {
  return (
    <BrowserRouter>
      <ToastProvider>
        <AppRoutes />
      </ToastProvider>
    </BrowserRouter>
  );
}
