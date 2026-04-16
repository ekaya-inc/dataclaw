import { Cable, DatabaseZap, FileCheck2, Menu } from 'lucide-react';
import { useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';

import type { RuntimeStatus } from '../types/datasource';

import { cn } from '../utils/cn';

const NAV_ITEMS = [
  { to: '/datasource', label: 'Datasource', icon: DatabaseZap },
  { to: '/queries', label: 'Approved Queries', icon: FileCheck2 },
  { to: '/openclaw', label: 'OpenClaw', icon: Cable },
];

export function AppShell({ status }: { status: RuntimeStatus | null }): JSX.Element {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const runtimeLabel = status?.port ? `localhost:${status.port}` : 'localhost';

  return (
    <div className="min-h-screen bg-surface-secondary text-text-primary">
      <div className="mx-auto flex min-h-screen max-w-screen-2xl">
        <aside
          className={cn(
            'fixed inset-y-0 left-0 z-20 w-72 border-r border-border-light bg-slate-950 px-5 py-6 text-slate-100 transition-transform lg:static lg:translate-x-0',
            mobileNavOpen ? 'translate-x-0' : '-translate-x-full',
          )}
        >
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-slate-400">DataClaw</p>
              <h1 className="mt-2 text-2xl font-semibold">OpenClaw bridge</h1>
            </div>
            <button className="rounded-lg border border-slate-700 px-3 py-2 lg:hidden" onClick={() => setMobileNavOpen(false)}>
              Close
            </button>
          </div>
          <p className="mt-4 text-sm leading-6 text-slate-300">
            Local-first setup for one datasource, a small approved-query catalog, and a single OpenClaw API key.
          </p>
          <div className="mt-6 rounded-xl border border-slate-800 bg-slate-900/80 p-4">
            <div className="text-xs uppercase tracking-[0.18em] text-slate-500">Runtime</div>
            <div className="mt-2 text-base font-medium">{runtimeLabel}</div>
            <div className="mt-1 text-sm text-slate-400">
              {status?.datasourceConfigured ? 'Datasource configured' : 'Datasource not configured yet'}
            </div>
          </div>
          <nav className="mt-8 space-y-2">
            {NAV_ITEMS.map((item) => {
              const Icon = item.icon;
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    cn(
                      'flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors',
                      isActive ? 'bg-slate-100 text-slate-950' : 'text-slate-300 hover:bg-slate-900 hover:text-slate-50',
                    )
                  }
                  onClick={() => setMobileNavOpen(false)}
                >
                  <Icon className="h-4 w-4" />
                  {item.label}
                </NavLink>
              );
            })}
          </nav>
        </aside>
        <div className="flex min-h-screen flex-1 flex-col lg:ml-0">
          <header className="sticky top-0 z-10 border-b border-border-light bg-surface-primary/95 backdrop-blur">
            <div className="flex items-center justify-between px-4 py-4 sm:px-6 lg:px-10">
              <button className="inline-flex items-center gap-2 rounded-lg border border-border-light px-3 py-2 text-sm lg:hidden" onClick={() => setMobileNavOpen(true)}>
                <Menu className="h-4 w-4" />
                Menu
              </button>
              <div className="hidden text-sm text-text-secondary lg:block">
                Local API: <span className="font-mono text-text-primary">{status?.baseUrl ?? 'http://127.0.0.1:18790'}</span>
              </div>
            </div>
          </header>
          <main className="flex-1 px-4 py-6 sm:px-6 lg:px-10 lg:py-10">
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  );
}
