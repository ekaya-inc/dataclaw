import { LogOut, Monitor, Moon, ShieldCheck, Sun } from 'lucide-react';

import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { useTheme, type ThemeMode } from '../hooks/useTheme';
import type { RuntimeStatus } from '../types/datasource';
import { cn } from '../utils/cn';

interface SettingsPageProps {
  status: RuntimeStatus | null;
  onLogout: () => void;
}

const THEME_OPTIONS: ReadonlyArray<{ value: ThemeMode; label: string; icon: typeof Sun }> = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
];

export default function SettingsPage({ status, onLogout }: SettingsPageProps): JSX.Element {
  const [theme, setTheme] = useTheme();

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div>
        <p className="text-sm font-semibold uppercase tracking-wide text-surface-submit">Admin</p>
        <h1 className="mt-2 text-3xl font-semibold text-text-primary">Settings</h1>
        <p className="mt-2 text-sm text-text-secondary">
          Review listener details and end the admin session in this browser.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Appearance</CardTitle>
          <CardDescription>Pick how DataClaw looks in this browser. System follows your OS preference.</CardDescription>
        </CardHeader>
        <CardContent>
          <div
            role="radiogroup"
            aria-label="Theme"
            className="inline-flex rounded-xl border border-border-light bg-surface-secondary p-1"
          >
            {THEME_OPTIONS.map((option) => {
              const Icon = option.icon;
              const selected = theme === option.value;
              return (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  onClick={() => setTheme(option.value)}
                  className={cn(
                    'inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors',
                    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-purple',
                    selected
                      ? 'bg-surface-primary text-text-primary shadow-sm'
                      : 'text-text-secondary hover:text-text-primary',
                  )}
                >
                  <Icon className="h-4 w-4" aria-hidden="true" />
                  {option.label}
                </button>
              );
            })}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center gap-3">
            <ShieldCheck className="h-5 w-5 text-surface-submit" aria-hidden="true" />
            <div>
              <CardTitle>Runtime listeners</CardTitle>
              <CardDescription>Admin and MCP are served from separate listener boundaries.</CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt className="font-medium text-text-secondary">Admin base URL</dt>
              <dd className="mt-1 break-all text-text-primary">{status?.adminBaseUrl ?? 'Loading…'}</dd>
            </div>
            <div>
              <dt className="font-medium text-text-secondary">MCP URL</dt>
              <dd className="mt-1 break-all text-text-primary">{status?.mcpUrl ?? 'Loading…'}</dd>
            </div>
            <div>
              <dt className="font-medium text-text-secondary">Admin port</dt>
              <dd className="mt-1 text-text-primary">{status?.adminPort ?? 'Loading…'}</dd>
            </div>
            <div>
              <dt className="font-medium text-text-secondary">MCP port</dt>
              <dd className="mt-1 text-text-primary">{status?.mcpPort ?? 'Loading…'}</dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Admin session</CardTitle>
          <CardDescription>Logout clears the admin session cookie stored by this browser.</CardDescription>
        </CardHeader>
        <CardContent>
          <Button type="button" variant="outline" onClick={onLogout}>
            <LogOut className="h-4 w-4" />
            Logout
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
