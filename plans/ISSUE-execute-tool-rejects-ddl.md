# ISSUE: `execute` MCP tool rejects DDL despite UI/description promising it

Status: OPEN
Reported: 2026-04-18

## What was observed

The `execute` MCP tool advertises support for DDL/DML, but the server-side
validator only accepts statements starting with `INSERT`, `UPDATE`, or `DELETE`.
Any DDL (`CREATE`, `ALTER`, `DROP`, `TRUNCATE`, etc.) is rejected with:

```
mutating queries must start with INSERT, UPDATE, or DELETE
```

This contradicts:

1. The MCP tool description (registered in
   `internal/mcpserver/server.go::registerExecuteTool`):
   > "Execute ad-hoc mutating SQL/DDL/DML against the configured datasource
   > when the authenticated agent has raw execute access."

2. The agent-editor UI checkbox copy in
   `ui/src/components/AgentFormFields.tsx` for "Allow agent full write access
   to the database":
   > "Expose the `execute` tool which gives full permissions granted by the
   > datasource credentials — potentially including `create`, `alter`, and
   > `drop database` as well as `insert`, `update`, and `delete` rows of data."

So we have advertised behavior (DDL allowed if creds permit) vs. actual
behavior (only INSERT/UPDATE/DELETE allowed). Either the validator is too
strict or the descriptions are misleading.

## Steps to reproduce

1. Configure a Postgres datasource.
2. Create an agent with `can_execute = true` and authenticate via MCP using
   its API key.
3. Call the `execute` tool with any DDL statement, for example:

   ```json
   {
     "sql": "CREATE TABLE public.scratch_demo (id SERIAL PRIMARY KEY, body TEXT)"
   }
   ```

4. Observe the error response:

   ```
   mutating queries must start with INSERT, UPDATE, or DELETE
   ```

5. Repeat with `INSERT INTO ... VALUES (...)` — that succeeds (assuming the
   target table exists), confirming the validator is the gate, not the
   datasource credentials.

Reproduced today (2026-04-18) against the local dev server using the
`Admin` agent's API key.

## Impact / why it matters

- Agents that "manage approved queries" frequently want to bootstrap a
  scratch/staging table to prototype SQL before registering an approved
  query. Without DDL in `execute`, that workflow is blocked unless a human
  uses `psql` outside DataClaw to create the table.
- The new dangerous-execute confirmation modal we just shipped explicitly
  warns the user about `create`, `alter`, and `drop database`. If the
  validator never allows those, that warning overstates the risk and
  understates the limitation.
- A human reviewing the agent's permission card may grant `execute` on the
  assumption that the agent can perform schema changes (because the UI says
  so) and later be surprised when the agent reports it cannot.

## Suspected location

- Validator: `pkg/sql` — specifically the `dsadapter.ValidateMutatingSQL`
  call inside `internal/core/agents.go::ExecuteRawMutation` (also surfaced
  via the `execute` MCP tool handler in `internal/mcpserver/server.go`).
- Tool description: `internal/mcpserver/server.go::registerExecuteTool`.
- UI copy: `ui/src/components/AgentFormFields.tsx` (the "Allow agent full
  write access to the database" checkbox description) and the dangerous-
  execute confirmation modal in
  `ui/src/components/DangerousExecuteDialog.tsx`.

Root cause not investigated — that belongs in a follow-up FIX file.

## Decision needed

Pick one of:

1. **Loosen the validator** to accept DDL when the agent has `can_execute`,
   matching the advertised behavior. Probably the right call given how the
   feature is positioned, but needs a security review (a malicious agent
   with `execute` could `DROP DATABASE` if the creds allow it — which is
   the existing UI warning).
2. **Tighten the descriptions** in the MCP tool registration and the UI
   copy to reflect the INSERT/UPDATE/DELETE-only reality, and rename or
   re-scope the capability accordingly.

Either path is fine; we just need them to agree.
