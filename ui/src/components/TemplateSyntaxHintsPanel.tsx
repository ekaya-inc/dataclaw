import type { TemplateSyntaxHints } from '../types/datasource';

interface Props {
  dialect?: string | undefined;
  hints?: TemplateSyntaxHints | undefined;
}

export function TemplateSyntaxHintsPanel({ dialect, hints }: Props): JSX.Element | null {
  if (!hints) return null;
  const placeholders = hints.placeholderAntiExamples ?? [];
  const pagination = hints.paginationAntiExamples ?? [];
  if (placeholders.length === 0 && pagination.length === 0 && !hints.notes) {
    return null;
  }
  const dialectLabel = dialect ? ` (${dialect})` : '';
  return (
    <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
      <div className="font-semibold">Template syntax hints{dialectLabel}</div>
      <p className="mt-1 text-amber-900/80">
        Use <code className="rounded bg-amber-100 px-1 py-0.5 font-mono text-xs">{'{{parameter_name}}'}</code> placeholders
        and the tool&apos;s <code className="rounded bg-amber-100 px-1 py-0.5 font-mono text-xs">limit</code> /{' '}
        <code className="rounded bg-amber-100 px-1 py-0.5 font-mono text-xs">offset</code> arguments. Avoid these
        dialect-native tokens directly in approved query templates:
      </p>
      <ul className="mt-2 grid gap-2 sm:grid-cols-2">
        {placeholders.length > 0 ? (
          <li>
            <div className="text-xs font-semibold uppercase tracking-wide text-amber-800">Placeholders</div>
            <div className="mt-1 flex flex-wrap gap-1">
              {placeholders.map((token) => (
                <code key={token} className="rounded bg-amber-100 px-1.5 py-0.5 font-mono text-xs">
                  {token}
                </code>
              ))}
            </div>
          </li>
        ) : null}
        {pagination.length > 0 ? (
          <li>
            <div className="text-xs font-semibold uppercase tracking-wide text-amber-800">Pagination</div>
            <div className="mt-1 flex flex-wrap gap-1">
              {pagination.map((token) => (
                <code key={token} className="rounded bg-amber-100 px-1.5 py-0.5 font-mono text-xs">
                  {token}
                </code>
              ))}
            </div>
          </li>
        ) : null}
      </ul>
      {hints.notes ? <p className="mt-3 text-xs text-amber-900/80">{hints.notes}</p> : null}
    </div>
  );
}
