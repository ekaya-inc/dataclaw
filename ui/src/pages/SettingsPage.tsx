import { LogOut, ShieldCheck } from 'lucide-react';

import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import type { RuntimeStatus } from '../types/datasource';

interface SettingsPageProps {
  status: RuntimeStatus | null;
  onLogout: () => void;
}

export default function SettingsPage({ status, onLogout }: SettingsPageProps): JSX.Element {
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
