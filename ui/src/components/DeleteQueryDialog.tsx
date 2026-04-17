import { AlertTriangle } from 'lucide-react';
import { useEffect } from 'react';

import { Button } from './ui/Button';

export function DeleteQueryDialog({
  open,
  queryPrompt,
  disabled,
  deleting,
  onCancel,
  onConfirm,
}: {
  open: boolean;
  queryPrompt: string;
  disabled?: boolean;
  deleting: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}): JSX.Element | null {
  useEffect(() => {
    if (!open) return;
    const handleKey = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        event.stopPropagation();
        onCancel();
      }
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, [open, onCancel]);

  if (!open) {
    return null;
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="delete-query-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-4"
    >
      <div className="w-full max-w-md rounded-2xl border border-border-light bg-surface-primary p-6 shadow-xl">
        <div className="flex items-start gap-3">
          <AlertTriangle className="mt-0.5 h-6 w-6 shrink-0 text-red-500" aria-hidden />
          <div>
            <h2 id="delete-query-title" className="text-lg font-semibold text-text-primary">
              Delete approved query?
            </h2>
            <p className="mt-1 text-sm text-text-secondary">
              This will permanently remove the approved query. Agents will no longer be able to execute it.
            </p>
            <p className="mt-3 rounded-lg bg-surface-secondary px-3 py-2 text-sm text-text-primary">
              {queryPrompt || 'Untitled query'}
            </p>
          </div>
        </div>
        <div className="mt-6 flex justify-end gap-3">
          <Button type="button" variant="outline" onClick={onCancel} disabled={deleting || disabled}>
            Cancel
          </Button>
          <Button type="button" variant="destructive" onClick={onConfirm} disabled={deleting || disabled}>
            {deleting ? 'Deleting…' : 'Delete query'}
          </Button>
        </div>
      </div>
    </div>
  );
}
