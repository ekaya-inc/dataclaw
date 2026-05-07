import { Bot, CheckCircle2, DatabaseZap, FileCheck2, Heart, LayoutDashboard, Menu, Settings } from 'lucide-react';
import { useState } from 'react';
import { Link, NavLink, Outlet } from 'react-router-dom';

import type { AppOutletContext } from '../App';
import type { RuntimeStatus } from '../types/datasource';

import { useSupportDismissed } from '../hooks/useSupportDismissed';
import { cn } from '../utils/cn';

interface Completion {
  datasource: boolean;
  queries: boolean;
  agent: boolean;
}

const NAV_ITEMS: ReadonlyArray<{
  to: string;
  label: string;
  icon: typeof DatabaseZap;
  completionKey?: keyof Completion;
}> = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/datasource', label: 'Datasource', icon: DatabaseZap, completionKey: 'datasource' },
  { to: '/queries', label: 'Approved Queries', icon: FileCheck2, completionKey: 'queries' },
  { to: '/agents', label: 'Agent Access', icon: Bot, completionKey: 'agent' },
  { to: '/support', label: 'Support', icon: Heart },
];

interface AppShellProps {
  status: RuntimeStatus | null;
  completion: Completion;
  outletContext: AppOutletContext;
}

export function AppShell({ status, completion, outletContext }: AppShellProps): JSX.Element {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [supportDismissed] = useSupportDismissed();
  const visibleNavItems = NAV_ITEMS.filter((item) => {
    if (item.to !== '/support') return true;
    return completion.agent && !supportDismissed;
  });

  return (
    <div className="min-h-screen bg-surface-secondary text-text-primary">
      <div className="mx-auto flex min-h-screen max-w-screen-2xl">
        <aside
          className={cn(
            'fixed inset-y-0 left-0 z-20 flex w-72 flex-col border-r border-sidebar-border bg-sidebar-bg px-5 py-6 text-sidebar-fg transition-transform lg:static lg:translate-x-0',
            mobileNavOpen ? 'translate-x-0' : '-translate-x-full',
          )}
        >
          <div className="flex items-center justify-between">
            <Link
              aria-label="DataClaw"
              className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-purple"
              to="/"
              onClick={() => setMobileNavOpen(false)}
            >
              <img
                src="/assets/logos/dataclaw-lockup-light-512.png"
                alt="DataClaw"
                className="h-auto w-56 max-w-full"
              />
            </Link>
            <button
              className="rounded-lg border border-sidebar-border px-3 py-2 lg:hidden"
              onClick={() => setMobileNavOpen(false)}
            >
              Close
            </button>
          </div>
          <p className="mt-4 text-sm leading-6 text-sidebar-fg-muted">
            Connect local agents to your data safely and securely.
          </p>
          <nav className="mt-8 flex-1 space-y-2">
            {visibleNavItems.map((item) => {
              const Icon = item.icon;
              const isComplete = item.completionKey ? completion[item.completionKey] : false;
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    cn(
                      'flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors',
                      isActive
                        ? 'bg-sidebar-item-active-bg text-sidebar-item-active-fg'
                        : 'text-sidebar-fg-muted hover:bg-sidebar-item-hover-bg hover:text-sidebar-item-hover-fg',
                    )
                  }
                  onClick={() => setMobileNavOpen(false)}
                >
                  <Icon className="h-4 w-4" />
                  <span className="flex-1">{item.label}</span>
                  {isComplete ? (
                    <CheckCircle2 className="h-4 w-4 text-[#2dd4bf]" aria-label="Completed" />
                  ) : null}
                </NavLink>
              );
            })}
          </nav>
          {status?.version ? (
            <div className="mt-4 text-right text-xs text-sidebar-fg-faint" aria-label="Server version">
              {status.version}
            </div>
          ) : null}
          <NavLink
            to="/settings"
            className={({ isActive }) =>
              cn(
                'mt-4 flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-sidebar-item-active-bg text-sidebar-item-active-fg'
                  : 'text-sidebar-fg-muted hover:bg-sidebar-item-hover-bg hover:text-sidebar-item-hover-fg',
              )
            }
            onClick={() => setMobileNavOpen(false)}
          >
            <Settings className="h-4 w-4" />
            <span className="flex-1">Settings</span>
          </NavLink>
        </aside>
        <div className="flex min-h-screen min-w-0 flex-1 flex-col lg:ml-0">
          <main className="flex-1 px-4 py-6 sm:px-6 lg:px-10 lg:py-10">
            <div className="mb-4 lg:hidden">
              <button className="inline-flex items-center gap-2 rounded-lg border border-border-light px-3 py-2 text-sm" onClick={() => setMobileNavOpen(true)}>
                <Menu className="h-4 w-4" />
                Menu
              </button>
            </div>
            <Outlet context={outletContext} />
          </main>
        </div>
      </div>
    </div>
  );
}
