const SCHEMA_PROBE_SQL = `SELECT column_name, data_type
FROM information_schema.columns
WHERE table_name = 'orders' AND table_schema = 'public';`;

const SCHEMA_PROBE_RESULT = `order_id     uuid
user_id      uuid
status       text
created_at   timestamp
num_of_item  integer`;

const PROTOTYPE_SQL = `SELECT order_id, user_id, status, created_at
FROM public.orders
WHERE status = 'Processing'
ORDER BY created_at DESC, order_id DESC
LIMIT 10 OFFSET 0;`;

const CREATE_QUERY_ARGS = `{
  "natural_language_prompt": "Recent orders by status, paginated",
  "additional_context": "Use when an analyst needs to scan recent orders filtered by status. Stable ordering on created_at DESC, order_id DESC.",
  "sql_query": "SELECT order_id, user_id, status, created_at FROM public.orders WHERE status = {{status}} ORDER BY created_at DESC, order_id DESC LIMIT {{limit}} OFFSET {{offset}}",
  "parameters": [
    { "name": "status", "type": "string",  "description": "Lifecycle status filter (e.g. Processing, Shipped).", "required": true },
    { "name": "limit",  "type": "integer", "description": "Page size.",   "required": false, "default": 10 },
    { "name": "offset", "type": "integer", "description": "Page offset.", "required": false, "default": 0 }
  ],
  "output_columns": [
    { "name": "order_id",   "type": "uuid",      "description": "Order identifier." },
    { "name": "user_id",    "type": "uuid",      "description": "Buyer identifier." },
    { "name": "status",     "type": "text",      "description": "Lifecycle status." },
    { "name": "created_at", "type": "timestamp", "description": "When the order was placed." }
  ]
}`;

const CREATE_QUERY_RESULT = `{ "query": { "query_id": "8a3c…", "natural_language_prompt": "Recent orders by status, paginated" } }`;

interface TurnProps {
  role: 'User' | 'Assistant' | 'Tool result';
  accent: string;
  children: React.ReactNode;
}

function Turn({ role, accent, children }: TurnProps): JSX.Element {
  return (
    <div className={`rounded-lg border-l-4 ${accent} bg-surface-primary px-3 py-2`}>
      <div className="text-[10px] font-semibold uppercase tracking-wide text-text-tertiary">{role}</div>
      <div className="mt-1 space-y-2 text-text-primary">{children}</div>
    </div>
  );
}

function ToolCall({ tool, args }: { tool: string; args: string }): JSX.Element {
  return (
    <div className="space-y-1">
      <div className="text-xs text-text-secondary">
        Calls <code className="rounded bg-surface-secondary px-1 py-0.5 font-mono text-[11px] text-text-primary">{tool}</code>
      </div>
      <pre className="overflow-x-auto whitespace-pre rounded-lg border border-border-light bg-surface-secondary p-2 font-mono text-[11px] text-text-primary">
        <code>{args}</code>
      </pre>
    </div>
  );
}

function ToolResult({ children }: { children: React.ReactNode }): JSX.Element {
  return (
    <pre className="overflow-x-auto whitespace-pre rounded-lg border border-border-light bg-surface-secondary p-2 font-mono text-[11px] text-text-primary">
      <code>{children}</code>
    </pre>
  );
}

export function ApprovedQueryManagerHelp({ panelId }: { panelId: string }): JSX.Element {
  return (
    <div
      id={panelId}
      className="space-y-4 rounded-xl border border-border-light bg-surface-secondary p-4 text-sm text-text-secondary"
    >
      <div className="space-y-1">
        <h4 className="text-sm font-semibold text-text-primary">Let an agent build the catalog for you</h4>
        <p>
          With this enabled, an MCP client (Claude Code, Cursor, your own agent) can connect with this agent&apos;s
          API key, explore your schema, prototype SQL, and register fully-parameterized approved queries with
          documented output columns — no hand-crafting required. Other agents then consume those queries via{' '}
          <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">execute_query</code>.
        </p>
        <p>
          A manager agent always has{' '}
          <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">query</code>,{' '}
          <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">list_queries</code>,{' '}
          <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">create_query</code>,{' '}
          <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">update_query</code>,{' '}
          <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">delete_query</code>, and{' '}
          <code className="rounded bg-surface-primary px-1 py-0.5 font-mono text-xs text-text-primary">execute_query</code> over
          every approved query — that&apos;s why the scope is locked to <em>All approved queries</em>.
        </p>
      </div>

      <div className="space-y-3">
        <h5 className="text-xs font-semibold uppercase tracking-wide text-text-primary">Example conversation</h5>

        <Turn role="User" accent="border-emerald-500">
          <p>
            Create an approved query that returns the most recent orders for a given status, with optional pagination.
          </p>
        </Turn>

        <Turn role="Assistant" accent="border-blue-500">
          <p>I&apos;ll inspect the schema first.</p>
          <ToolCall tool="query" args={`{ "sql": ${JSON.stringify(SCHEMA_PROBE_SQL)} }`} />
        </Turn>

        <Turn role="Tool result" accent="border-amber-500">
          <ToolResult>{SCHEMA_PROBE_RESULT}</ToolResult>
        </Turn>

        <Turn role="Assistant" accent="border-blue-500">
          <p>Now let me prototype the SQL with realistic values.</p>
          <ToolCall tool="query" args={`{ "sql": ${JSON.stringify(PROTOTYPE_SQL)}, "limit": 10 }`} />
        </Turn>

        <Turn role="Tool result" accent="border-amber-500">
          <ToolResult>10 rows returned. Sample: order_id=&quot;f1a2…&quot;, status=&quot;Processing&quot;, created_at=&quot;2026-04-17T18:42:00Z&quot;.</ToolResult>
        </Turn>

        <Turn role="Assistant" accent="border-blue-500">
          <p>The query works. Registering it with named parameters and output columns so callers get full schema metadata.</p>
          <ToolCall tool="create_query" args={CREATE_QUERY_ARGS} />
        </Turn>

        <Turn role="Tool result" accent="border-amber-500">
          <ToolResult>{CREATE_QUERY_RESULT}</ToolResult>
        </Turn>

        <Turn role="Assistant" accent="border-blue-500">
          <p>
            Done. <em>Recent orders by status, paginated</em> is now in the catalog. Grant other agents access on the Agents page.
          </p>
        </Turn>
      </div>
    </div>
  );
}
