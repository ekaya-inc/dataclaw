import { AlertTriangle } from 'lucide-react';
import { useEffect, useState } from 'react';

import { Button } from './ui/Button';
import { Input } from './ui/Input';

const REQUIRED_PHRASE = 'enable dangerous execute';

export function DangerousExecuteDialog({
  onCancel,
  onConfirm,
}: {
  onCancel: () => void;
  onConfirm: () => void;
}): JSX.Element {
  const [phrase, setPhrase] = useState('');

  useEffect(() => {
    const handleKey = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        event.stopPropagation();
        onCancel();
      }
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, [onCancel]);

  const matches = phrase.trim() === REQUIRED_PHRASE;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="dangerous-execute-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-4"
    >
      <div className="w-full max-w-lg rounded-2xl border border-border-light bg-surface-primary p-6 shadow-xl">
        <div className="flex items-start gap-3">
          <AlertTriangle className="mt-0.5 h-6 w-6 shrink-0 text-red-500" aria-hidden />
          <div className="min-w-0 flex-1">
            <h2 id="dangerous-execute-title" className="text-lg font-semibold text-text-primary">
              Enable dangerous execute?
            </h2>
            <p className="mt-1 text-sm text-text-secondary">
              This grants the agent full write access at the level of the datasource credentials. The agent could
              <strong className="text-text-primary"> destroy data</strong>, drop tables, or alter schema with no further
              confirmation. Only enable this for agents under direct human supervision by an administrator.
            </p>
            <div className="mt-4 space-y-2">
              <label htmlFor="dangerous-execute-phrase" className="block text-sm text-text-secondary">
                Type{' '}
                <code className="rounded bg-surface-secondary px-1 py-0.5 font-mono text-xs text-text-primary">
                  {REQUIRED_PHRASE}
                </code>{' '}
                to confirm.
              </label>
              <Input
                id="dangerous-execute-phrase"
                value={phrase}
                autoFocus
                autoComplete="off"
                spellCheck={false}
                onChange={(event) => setPhrase(event.target.value)}
                placeholder={REQUIRED_PHRASE}
              />
            </div>
          </div>
        </div>
        <div className="mt-6 flex justify-end gap-3">
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="button" variant="destructive" onClick={onConfirm} disabled={!matches}>
            Enable dangerous execute
          </Button>
        </div>
      </div>
    </div>
  );
}
