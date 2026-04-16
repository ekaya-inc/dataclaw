import type { ReactNode } from 'react';

export function EmptyState({ title, body, actions }: { title: string; body: string; actions?: ReactNode }): JSX.Element {
  return (
    <div className="rounded-2xl border border-dashed border-border-light bg-surface-secondary/60 p-8 text-center">
      <h3 className="text-lg font-semibold text-text-primary">{title}</h3>
      <p className="mx-auto mt-2 max-w-2xl text-sm text-text-secondary">{body}</p>
      {actions ? <div className="mt-4 flex flex-wrap justify-center gap-3">{actions}</div> : null}
    </div>
  );
}
