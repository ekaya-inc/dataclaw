import { Bot, CheckCircle2, DatabaseZap, FileCheck2, Heart, Menu } from 'lucide-react';
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
            'fixed inset-y-0 left-0 z-20 flex w-72 flex-col border-r border-border-light bg-slate-950 px-5 py-6 text-slate-100 transition-transform lg:static lg:translate-x-0',
            mobileNavOpen ? 'translate-x-0' : '-translate-x-full',
          )}
        >
          <div className="flex items-center justify-between">
            <Link
              aria-label="DataClaw"
              className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-300"
              to="/"
              onClick={() => setMobileNavOpen(false)}
            >
              <img
                src="/assets/logos/dataclaw-lockup-light-512.png"
                alt="DataClaw"
                className="h-auto w-56 max-w-full"
              />
            </Link>
            <button className="rounded-lg border border-slate-700 px-3 py-2 lg:hidden" onClick={() => setMobileNavOpen(false)}>
              Close
            </button>
          </div>
          <p className="mt-4 text-sm leading-6 text-slate-300">
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
                      isActive ? 'bg-slate-100 text-slate-950' : 'text-slate-300 hover:bg-slate-900 hover:text-slate-50',
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
            <div className="mt-4 text-right text-xs text-slate-600" aria-label="Server version">
              {status.version}
            </div>
          ) : null}
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
