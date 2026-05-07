import type { TemplateSyntaxHints } from '../types/datasource';

const EXAMPLE_SQL = `SELECT
  {{user_id}}                                    AS user_id,            -- uuid
  {{user_id}}::text || ' (current)'              AS labelled_user,      -- uuid → text
  {{is_active}}                                  AS active_flag,        -- boolean
  CASE WHEN {{is_active}} THEN 'yes' ELSE 'no' END AS active_label,
  {{min_total}}                                  AS min_total,          -- decimal
  ROUND({{min_total}} * 1.08, 2)                 AS min_total_with_tax,
  {{quantity}}                                   AS quantity,           -- integer
  {{quantity}} * 2                               AS quantity_doubled,
  {{start_date}}                                 AS start_date,         -- date
  {{end_date}}                                   AS end_date,
  ({{end_date}}::date - {{start_date}}::date)    AS window_days,        -- DB-side cast
  {{cutoff_ts}}                                  AS cutoff_ts,          -- timestamp
  NOW() > {{cutoff_ts}}::timestamptz             AS past_cutoff,        -- DB-side cast
  {{allowed_statuses}}                           AS allowed_statuses,   -- string[]
  'pending' = ANY({{allowed_statuses}})          AS pending_allowed,
  {{ids}}                                        AS ids,                -- integer[]
  array_length({{ids}}, 1)                       AS id_count;`;

const PAGINATION_DONT_SQL = `SELECT order_id, status, created_at
FROM public.orders
WHERE status = {{status}}
ORDER BY created_at DESC, order_id DESC
LIMIT 10 OFFSET 20;`;

const PAGINATION_DO_SQL = `SELECT order_id, status, created_at
FROM public.orders
WHERE status = {{status}}
ORDER BY created_at DESC, order_id DESC;`;

const PAGINATION_TOOL_CALL = `{
  "name": "execute_query",
  "arguments": {
    "query_id": "recent_orders_by_status",
    "parameters": { "status": "Processing" },
    "limit": 10,
    "offset": 20
  }
}`;

interface TypeRow {
  name: string;
  input: string;
  behavior: string;
}

const TYPE_ROWS: TypeRow[] = [
  { name: 'string', input: '"pending"', behavior: 'Passed as text. Scanned for SQL injection.' },
  { name: 'integer', input: '42 or "42"', behavior: 'Coerced to int64. Floats with fractions are rejected.' },
  { name: 'decimal', input: '99.95 or "99.95"', behavior: 'Coerced to float64.' },
  { name: 'boolean', input: 'true / "true" / 1', behavior: 'Coerced to bool.' },
  { name: 'date', input: '"2026-04-18"', behavior: 'Validated as ISO YYYY-MM-DD.' },
  { name: 'timestamp', input: '"2026-04-18T09:53:00Z"', behavior: 'Validated as RFC3339.' },
  { name: 'uuid', input: '"550e8400-e29b-41d4-a716-446655440000"', behavior: 'Validated as a UUID.' },
  { name: 'string[]', input: '["a","b"] or "a, b"', behavior: 'Use with ANY(...). Postgres only.' },
  { name: 'integer[]', input: '[1,2] or "1, 2"', behavior: 'Use with ANY(...). Postgres only.' },
];

interface Props {
  panelId: string;
  dialect?: string | undefined;
  hints?: TemplateSyntaxHints | undefined;
}

export function ParameterHelp({ panelId, dialect, hints }: Props): JSX.Element {
  const placeholderTokens = hints?.placeholderAntiExamples ?? [];
  const paginationTokens = hints?.paginationAntiExamples ?? [];
  const dialectLabel = dialect ?? 'this dialect';

  return (
    <div
      id={panelId}
      className="space-y-4 rounded-xl border border-border-light bg-surface-secondary p-4 text-sm text-text-secondary"
    >
      <div className="space-y-1">
        <h4 className="text-sm font-semibold text-text-primary">Using parameters</h4>
        <p>
          Reference parameters in your SQL with <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">{'{{name}}'}</code>.
          Names must start with a letter or underscore and may contain letters, digits, and underscores. Callers supply values at execute time; the same placeholder may appear multiple times.
        </p>
      </div>

      <div className="space-y-2">
        <h5 className="text-xs font-semibold uppercase tracking-wide text-text-primary">Supported types</h5>
        <div className="overflow-x-auto rounded-lg border border-border-light bg-surface-primary">
          <table className="w-full text-left text-xs">
            <thead className="bg-surface-secondary text-text-primary">
              <tr>
                <th className="px-3 py-2 font-semibold">Type</th>
                <th className="px-3 py-2 font-semibold">Example input</th>
                <th className="px-3 py-2 font-semibold">Behavior</th>
              </tr>
            </thead>
            <tbody>
              {TYPE_ROWS.map((row) => (
                <tr key={row.name} className="border-t border-border-light">
                  <td className="px-3 py-2 font-mono text-text-primary">{row.name}</td>
                  <td className="px-3 py-2 font-mono">{row.input}</td>
                  <td className="px-3 py-2">{row.behavior}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <div className="space-y-2">
        <h5 className="text-xs font-semibold uppercase tracking-wide text-text-primary">Pagination &amp; dialect tokens</h5>
        <p>
          Don&apos;t put <code className="font-mono text-xs">LIMIT</code> / <code className="font-mono text-xs">OFFSET</code> or dialect-native bind markers in your approved query SQL. DataClaw applies pagination at execute time from the tool&apos;s <code className="font-mono text-xs">limit</code> and <code className="font-mono text-xs">offset</code> arguments, and binds named parameters as prepared statements.
        </p>
        <div className="grid gap-3 md:grid-cols-2">
          <div className="space-y-1">
            <div className="text-xs font-semibold uppercase tracking-wide text-text-tertiary">Don&apos;t</div>
            <pre className="overflow-x-auto whitespace-pre rounded-lg border border-border-light bg-surface-primary p-3 font-mono text-xs text-text-primary">
              <code>{PAGINATION_DONT_SQL}</code>
            </pre>
          </div>
          <div className="space-y-1">
            <div className="text-xs font-semibold uppercase tracking-wide text-text-tertiary">Do</div>
            <pre className="overflow-x-auto whitespace-pre rounded-lg border border-border-light bg-surface-primary p-3 font-mono text-xs text-text-primary">
              <code>{PAGINATION_DO_SQL}</code>
            </pre>
          </div>
        </div>
        <p>Callers paginate via the tool call:</p>
        <pre className="overflow-x-auto whitespace-pre rounded-lg border border-border-light bg-surface-primary p-3 font-mono text-xs text-text-primary">
          <code>{PAGINATION_TOOL_CALL}</code>
        </pre>
        {placeholderTokens.length > 0 || paginationTokens.length > 0 ? (
          <div className="space-y-1 pt-1">
            <div className="text-xs font-semibold uppercase tracking-wide text-text-tertiary">
              Avoid in {dialectLabel} templates
            </div>
            <div className="flex flex-wrap gap-1">
              {[...placeholderTokens, ...paginationTokens].map((token) => (
                <code key={token} className="rounded bg-surface-primary px-1.5 py-0.5 font-mono text-xs text-text-primary">
                  {token}
                </code>
              ))}
            </div>
          </div>
        ) : null}
        {hints?.notes ? <p className="text-xs">{hints.notes}</p> : null}
      </div>

      <div className="space-y-2">
        <h5 className="text-xs font-semibold uppercase tracking-wide text-text-primary">Do you need to cast?</h5>
        <ul className="list-disc space-y-1 pl-5">
          <li>
            <strong className="text-text-primary">No</strong> for the input itself — the supplied value is coerced to the declared type and bound to the database as a prepared‑statement parameter. You don&apos;t cast inputs with <code className="font-mono text-xs">::type</code> to get them into the query.
          </li>
          <li>
            <strong className="text-text-primary">Yes</strong> when the database needs a specific type for an operator — e.g. date arithmetic (<code className="font-mono text-xs">{'{{end_date}}::date - {{start_date}}::date'}</code>), timestamp math with <code className="font-mono text-xs">NOW()</code>, or comparing against a column whose type differs from the parameter.
          </li>
        </ul>
      </div>

      <div className="space-y-2">
        <h5 className="text-xs font-semibold uppercase tracking-wide text-text-primary">Arrays</h5>
        <p>
          Use <code className="font-mono text-xs">ANY({'{{arr}}'})</code> rather than <code className="font-mono text-xs">IN (...)</code>. Array parameters are supported on PostgreSQL only — SQL Server datasources reject them at execute time.
        </p>
      </div>

      <div className="space-y-2">
        <h5 className="text-xs font-semibold uppercase tracking-wide text-text-primary">Example</h5>
        <p>
          A single <code className="font-mono text-xs">SELECT</code> that exercises every type through comparisons and formatted outputs:
        </p>
        <pre className="overflow-x-auto whitespace-pre rounded-lg border border-border-light bg-surface-primary p-3 font-mono text-xs text-text-primary">
          <code>{EXAMPLE_SQL}</code>
        </pre>
      </div>
    </div>
  );
}
