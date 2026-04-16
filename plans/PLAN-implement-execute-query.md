# Plan: Implement `execute_query`

## Goal

Add an MCP tool named `execute_query` that executes a saved approved query by ID and accepts parameter values for that query.

The implementation should reuse DataClaw's existing stored-query execution path where possible, but it should first harden that path so MCP callers get explicit validation errors instead of database-level failures.

## What I Reviewed

- DataClaw MCP tool registration: `internal/mcpserver/server.go:23-29`
- DataClaw stored-query HTTP execution path: `internal/httpapi/handlers.go:72-75`, `internal/httpapi/handlers.go:195-259`
- DataClaw core saved-query execution: `internal/core/service.go:186-206`
- DataClaw SQL validation / substitution / MSSQL conversion / row execution: `internal/core/external.go:486-572`
- DataClaw parameter helpers and injection helpers: `pkg/sql/parameters.go:45-236`, `pkg/sql/injection.go:15-79`
- DataClaw current UI execute helper: `ui/src/services/api.ts:317-327`
- DataClaw current UI execute button path: `ui/src/pages/ApprovedQueriesPage.tsx:154-166`
- DataClaw current MCP/docs copy: `README.md:10-16`, `CLAUDE.md:21-29`, `ui/src/pages/OpenClawPage.tsx:142-155`
- ekaya-engine MCP execute tool: `../ekaya-engine/pkg/mcp/tools/queries.go:217-412`
- ekaya-engine parameterized execution and validation: `../ekaya-engine/pkg/services/query.go:690-781`, `../ekaya-engine/pkg/services/query.go:886-1108`

## Findings

- DataClaw already has almost all of the backend plumbing needed for parameterized saved-query execution.
- DataClaw already exposes `POST /api/queries/{id}/execute` and that endpoint already accepts `parameters` and `limit`.
- DataClaw does not expose that capability through MCP today. The MCP server only registers `query`, `list_queries`, `create_query`, `update_query`, and `delete_query`.
- DataClaw's current `ExecuteStoredQuery` path substitutes parameters and runs the query, but it does not validate required parameters, reject unknown parameters, coerce types, or run SQL injection checks before execution.
- DataClaw's current `ExecuteStoredQuery` path does not check `IsEnabled`, even though approved queries have an enabled flag.
- ekaya-engine does all of those missing checks before it executes a saved query, then calls `QueryWithParams`.
- ekaya-engine also contains extra concerns that DataClaw should not blindly port into this change: project/tenant access control, agent query ACLs, audit logging, query history, and a separate modifying-query branch with `allows_modification`.
- DataClaw's UI helper for saved-query execution currently posts an empty body, so browser execution cannot pass parameters even though the HTTP endpoint already supports them.

## Recommended Scope

Implement the MCP tool and harden DataClaw's existing stored-query execution path.

Do not port ekaya-engine's multi-tenant or audit/history code.

Do not add modifying-query support in this change. DataClaw does not have ekaya-engine's `allows_modification` field or separate write-execution result shape. Its current saved-query execution path always returns row-oriented `QueryResult`, which is a poor fit for write queries.

Recommended behavior for this change:

- `execute_query` executes only enabled saved queries.
- `execute_query` accepts `query_id`, optional `parameters`, and optional `limit`.
- The service validates and coerces parameters before substitution.
- The service rejects likely SQL injection attempts using the existing `pkg/sql` helper.
- The tool returns the same result shape as the raw `query` tool: `columns`, `rows`, `row_count`.

## Contract To Implement

### MCP tool

Tool name:

```text
execute_query
```

Arguments:

- `query_id` required string
- `parameters` optional object
- `limit` optional number, default `100`, max `1000`

Recommended description:

```text
Execute an approved query stored in DataClaw by ID. Parameters are validated and bound before execution.
```

### Response shape

Keep the response aligned with the existing `query` tool and HTTP execute endpoint:

```json
{
  "columns": [{ "name": "connected", "type": "boolean" }],
  "rows": [{ "connected": true }],
  "row_count": 1
}
```

This keeps the change small and avoids inventing a second saved-query result format.

## Implementation Plan

### 1. Add the MCP tool

Change `internal/mcpserver/server.go`.

- Register `execute_query` from `New(...)` next to the other query tools.
- Add a `registerExecuteQueryTool(...)` function.
- Use `mcp.WithString("query_id", ...)`, `mcp.WithObject("parameters", ...)`, and `mcp.WithNumber("limit", ...)`.
- Parse `parameters` as `map[string]any`.
- Default `limit` to `100`.
- Call `service.ExecuteStoredQuery(ctx, queryID, params, limit)`.
- Marshal the returned `QueryResult` exactly like the existing `query` tool does.

Important detail:

- Keep `query_id` as the canonical argument name. DataClaw's update/delete tools already use `query_id`, and this avoids creating a third naming style.
- Do not add UUID parsing here. DataClaw stores query IDs as strings in SQLite, and the current CRUD APIs treat them as opaque strings.

### 2. Harden `ExecuteStoredQuery`

Change `internal/core/service.go`.

Before `resolveSQLAndArgs(...)`, add the checks that DataClaw is missing today:

- Load the query and reject `nil` as `"query not found"` as it does now.
- Reject disabled queries with a clear error such as `"query is disabled"`.
- Validate required parameters before substitution.
- Reject unknown parameters that are not declared in `q.Parameters`.
- Coerce values to their declared parameter types before substitution.
- Run `pkg/sql.CheckAllParameters(...)` on the coerced parameter map.

Recommended helper split:

- `validateRequiredParameters(paramDefs []models.QueryParameter, supplied map[string]any) error`
- `coerceParameterTypes(paramDefs []models.QueryParameter, supplied map[string]any) (map[string]any, error)`
- `coerceValue(value any, targetType string, paramName string) (any, error)`
- small helpers for `string`, `integer`, `decimal`, `boolean`, `date`, `timestamp`, `uuid`

Borrow the behavior from ekaya-engine rather than re-inventing it:

- missing required parameter -> explicit validation error
- unknown parameter -> explicit validation error
- JSON numbers arriving as `float64` should coerce for integer/decimal params
- date should validate `YYYY-MM-DD`
- timestamp should validate RFC3339
- uuid should validate with `uuid.Parse`

Recommended minimum supported types for DataClaw:

- `string`
- `integer`
- `decimal`
- `boolean`
- `date`
- `timestamp`
- `uuid`

Optional:

- also port `string[]` and `integer[]` from ekaya-engine now, because the coercion code is already available there and the backend model stores free-form parameter types

### 3. Decide the read-only policy explicitly

Recommended choice for this change:

- reject mutating saved queries during execution until DataClaw has a real `allows_modification` model

Why:

- ekaya-engine has a separate branch for modifying queries and a separate result type: `../ekaya-engine/pkg/mcp/tools/queries.go:311-401`, `../ekaya-engine/pkg/services/query.go:784-875`
- DataClaw does not have that model
- DataClaw's current execution result is row-only: `internal/core/external.go:528-572`

Practical implementation options:

- Minimal option: leave create/update behavior alone, but in `ExecuteStoredQuery` call `validateReadOnlySQL(q.SQLQuery)` before execution and reject anything non-read-only.
- Cleaner option: also switch approved-query creation and update to enforce read-only SQL up front, which keeps the catalog consistent.

Recommended path:

- use the minimal option in this change unless the session is already touching the query creation flow

### 4. Keep the existing HTTP execute path working

No new HTTP route is required.

`internal/httpapi/handlers.go:247-259` already passes `parameters` and `limit` into `ExecuteStoredQuery(...)`.

Once `ExecuteStoredQuery(...)` is hardened, both HTTP and MCP callers benefit automatically.

### 5. Update docs and UI copy

Change these files:

- `README.md`
- `CLAUDE.md`
- `ui/src/pages/OpenClawPage.tsx`

Update the documented MCP tool list to include `execute_query`.

### 6. Optional browser parity follow-up

This is not required for the MCP tool to work, but it is a known gap.

Current UI behavior:

- `ui/src/services/api.ts:317-327` posts `{}` to `/api/queries/{id}/execute`
- `ui/src/pages/ApprovedQueriesPage.tsx:154-166` has no parameter collection step

If the implementation session wants full parity, add:

- `executeSavedQuery(id, parameters?, limit?)` in `ui/src/services/api.ts`
- a small parameter-value form when the selected saved query has parameters
- disable or warn on executing disabled queries

If UI parity is out of scope, leave a note in the PR or final report so the gap remains explicit.

## Files Likely To Change

- `internal/mcpserver/server.go`
- `internal/core/service.go`
- `README.md`
- `CLAUDE.md`
- `ui/src/pages/OpenClawPage.tsx`
- `ui/src/services/api.ts` optional
- `ui/src/pages/ApprovedQueriesPage.tsx` optional
- `ui/src/services/api.test.ts` optional
- `internal/core/service_test.go` or a new focused core execution test file

## Tests To Add

### Backend

Add tests for the new validation/coercion helpers.

Minimum cases:

- required parameter present -> succeeds
- required parameter missing -> explicit error
- optional parameter omitted with default -> succeeds
- unknown parameter supplied -> explicit error
- integer coercion from JSON `float64`
- decimal coercion from string
- boolean coercion from string
- invalid date -> explicit error
- invalid timestamp -> explicit error
- invalid uuid -> explicit error
- injection string detected by `pkg/sql.CheckAllParameters(...)` -> explicit error
- disabled query rejected

If the read-only guard is added:

- saved `UPDATE` or `DELETE` query is rejected before execution

### MCP / integration

If a dedicated MCP tool test is practical, add one. If not, rely on core tests plus manual verification.

### UI

Only if browser parity is implemented:

- add a test in `ui/src/services/api.test.ts` asserting `executeSavedQuery(...)` posts `parameters` and `limit`

## Manual Verification

After implementation:

1. Start DataClaw with a real datasource.
2. Create a saved query with parameters, for example `SELECT {{value}} AS value`.
3. Confirm `list_queries` shows the query and its parameter definitions.
4. Call `execute_query` with:
   - valid parameters
   - a missing required parameter
   - an unknown parameter
   - an invalid typed value
5. Confirm the result shape matches `query`.
6. Confirm the error messages are validation errors, not raw database errors.
7. If the read-only guard was added, confirm a saved mutating query is rejected cleanly.

## Suggested Verification Commands

Backend:

```bash
go test ./internal/core/...
go test ./pkg/sql/...
go test ./...
```

UI, only if UI files changed:

```bash
npm --prefix ui test -- --run
npm --prefix ui run lint
npm --prefix ui run typecheck
npm --prefix ui run build
```

Repo quality gate before declaring completion:

```bash
make check
```

## Deferred Work

Do not mix these into the first implementation unless there is extra time:

- full modifying-query support with an `allows_modification` field
- query execution audit/history
- enriched `execute_query` response with `query_name` or `parameters_used`
- full browser UX for entering saved-query parameters
- list-query response normalization (`id` -> `query_id`, `sql_query` -> `sql`), which is a separate compatibility cleanup already captured in `plans/ISSUE-approved-query-parameter-normalization.md`
