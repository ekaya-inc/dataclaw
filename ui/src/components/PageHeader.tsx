import type { ReactNode } from 'react';

export function PageHeader({ title, description, actions }: { title: string; description: string; actions?: ReactNode }): JSX.Element {
  return (
    <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div>
        <h1 className="text-3xl font-semibold tracking-tight text-text-primary">{title}</h1>
        <p className="mt-2 max-w-3xl text-sm text-text-secondary">{description}</p>
      </div>
      {actions ? <div className="flex flex-wrap gap-3">{actions}</div> : null}
    </div>
  );
}
