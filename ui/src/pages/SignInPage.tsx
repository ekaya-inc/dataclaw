import { LockKeyhole } from 'lucide-react';
import type { FormEvent } from 'react';
import { useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';

import { Button } from '../components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card';
import { Input } from '../components/ui/Input';
import { Label } from '../components/ui/Label';
import { signin } from '../services/api';

function safeNextPath(value: string | null): string {
  if (!value || !value.startsWith('/') || value.startsWith('//')) return '/';
  return value;
}

export default function SignInPage({ onSignedIn }: { onSignedIn: () => void }): JSX.Element {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const nextPath = useMemo(() => safeNextPath(searchParams.get('next')), [searchParams]);

  const [password, setPassword] = useState('');
  const [remember, setRemember] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (!password || submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      await signin(password, remember);
      onSignedIn();
      navigate(nextPath, { replace: true });
    } catch (signinError) {
      setError(signinError instanceof Error ? signinError.message : 'Sign in failed.');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="flex min-h-screen items-center justify-center bg-surface-secondary px-4 py-10 text-text-primary">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-surface-submit/10 text-surface-submit">
            <LockKeyhole className="h-6 w-6" aria-hidden="true" />
          </div>
          <CardTitle className="text-2xl">Sign in to DataClaw</CardTitle>
          <CardDescription>
            Enter the admin password to manage datasource, approved query, and agent access settings.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-5" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <Label htmlFor="admin-password">Admin password</Label>
              <Input
                id="admin-password"
                name="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                autoFocus
              />
            </div>
            <label className="flex items-start gap-3 text-sm text-text-secondary">
              <input
                type="checkbox"
                className="mt-1 h-4 w-4 rounded border-border-medium text-surface-submit focus:ring-brand-purple"
                checked={remember}
                onChange={(event) => setRemember(event.target.checked)}
              />
              <span>Keep me signed in on this browser.</span>
            </label>
            {error ? (
              <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                {error}
              </div>
            ) : null}
            <Button type="submit" className="w-full" disabled={!password || submitting}>
              {submitting ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
