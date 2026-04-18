# ISSUE: Add `get_datasource_information` MCP tool; stop leaking unusable `datasource_id`

Status: OPEN

## Motivation

DataClaw is a single-datasource-per-server product (localhost-only, one configured datasource at a time). Today, MCP clients receive two unhelpful shapes of datasource information:

1. **An opaque `datasource_id` UUID** on every approved query in `list_queries` / `create_query` / `update_query` results. There is no MCP tool that resolves it to a name, type, or anything else. It is pure noise for an MCP caller — nothing they can do with a UUID.

2. **No direct way to learn the database dialect/version** unless the agent holds `CanQuery` or `CanExecute` and blindly issues `SELECT version()` (which itself is Postgres-specific syntax — the agent can't know to try it without already knowing the dialect). Agents that can only invoke approved queries have no path at all. They don't need the dialect to *generate* SQL (they can't), but knowing it helps them interpret results and shape follow-on reasoning.

The `health` tool does expose `datasource.type` and `datasource.name`, but its purpose is connectivity status, not introspection, and the information is minimal.

## Proposal

### 1. Remove `datasource_id` from MCP tool responses

- `internal/mcpserver/server.go:28` — drop the `DatasourceID` field from `approvedQueryResponse`.
- `internal/mcpserver/server.go:378` — drop the `DatasourceID:` assignment in `normalizeApprovedQuery`.
- Audit `internal/mcpserver/tool_events.go:301` — the `datasource_id` key there is read from an event record for telemetry; determine whether it needs to stay (internal) or be stripped (external). Likely stays, since it's not on the tool-response path.
- Update `internal/mcpserver/server_test.go` and any other tests that assert on `datasource_id` in MCP responses.

The HTTP API (`/api/*`) can continue to expose `datasource_id` — it is the surface that actually manages datasources. The change is scoped to the MCP surface.

### 2. New MCP tool: `get_datasource_information`

**Availability:** Registered unconditionally in `buildMCPServer`, but surfaced by `filterToolsForContext` **only when a datasource is configured** (`service.HasDatasource(ctx)` is true). Unlike `query`/`execute`/`list_queries`/etc., it does NOT gate on any agent permission — every authenticated agent with a configured datasource sees it. This is the "one tool every client can rely on" for basic introspection.

**Registration location:** new file `internal/mcpserver/datasource_info.go` alongside `health.go` (same pattern).

**Annotations (once the hint-annotation ISSUE is landed):** `readOnly=true, destructive=false, idempotent=true, openWorld=false`.

**Behavior at call time:** returns a structured payload like:

```json
{
  "name": "dataclaw",
  "type": "postgres",
  "database_name": "the_look",
  "schema_name": "public",
  "current_user": "ekaya",
  "version": "PostgreSQL 18.2 (Homebrew) on aarch64-apple-darwin24.6.0, compiled by Apple clang version 17.0.0 (clang-1700.6.3.2), 64-bit"
}
```

The exact field set is TBD by the adapter (see §3), but `name`, `type`, and `version` are mandatory; `database_name`, `schema_name`, and `current_user` are strongly encouraged when the backend supports them.

### 3. Adapter interface addition

Add a new capability to the datasource adapter interface (`internal/adapters/datasource/interfaces.go`) so each backend owns its own introspection query. The existing `QueryExecutor` and `ConnectionTester` interfaces are the pattern to mirror.

Suggested shape:

```go
type DatasourceIntrospector interface {
    DatasourceInfo(ctx context.Context) (*DatasourceInfo, error)
    Close() error
}

type DatasourceInfo struct {
    DatabaseName string `json:"database_name,omitempty"`
    SchemaName   string `json:"schema_name,omitempty"`
    CurrentUser  string `json:"current_user,omitempty"`
    Version      string `json:"version,omitempty"`
    // Extra is adapter-specific; leave room for additions without interface churn.
    Extra        map[string]string `json:"extra,omitempty"`
}
```

Also extend `Registration` in `interfaces.go` with a `DatasourceIntrospectorFactory` field, mirroring `ConnectionTesterFactory` / `QueryExecutorFactory`.

**Postgres implementation** (`internal/adapters/datasource/postgres/`):

```sql
SELECT
  current_database() AS database_name,
  current_schema() AS schema_name,
  current_user AS current_user,
  version() AS version
```

**SQL Server implementation** (`internal/adapters/datasource/mssql/`):

```sql
SELECT
  DB_NAME() AS database_name,
  SCHEMA_NAME() AS schema_name,
  SUSER_SNAME() AS current_user,
  @@VERSION AS version
```

Validate both against live instances before merging. If future adapters are added, implementing this interface should be mandatory (the `get_datasource_information` tool depends on it existing for every supported type).

### 4. Inject the info into the tool description at registration

**This is the core idea, and the point the user wants to validate.** The tool description is sent to the LLM as part of the tool list, so embedding the datasource facts there means every request benefits from them *without the LLM needing to call the tool first*. The tool itself still exists so callers that don't see descriptions (or that want the structured payload) can retrieve the same info programmatically.

**Mechanism:**

- Tool registration is currently static. `get_datasource_information` needs the registration to run after the datasource is known and to re-run if the datasource changes.
- Simplest path: re-register the tool each time `HasDatasource` transitions, rebuilding the MCP server. But `server.NewMCPServer` is built once in `app.Run` — this would require either (a) dynamic tool updates via `mark3labs/mcp-go`'s tool-change notifications, or (b) rebuilding the server whenever the datasource changes.
- Alternative: register the tool with a generic description at startup, and update the description lazily the first time it is listed (inside the existing `AddAfterListTools` hook in `internal/mcpserver/server.go:60`). That hook already mutates the tool list per-request — it can also rewrite the description of `get_datasource_information` with current datasource info. This avoids server rebuilds entirely and keeps the information fresh even if the datasource changes mid-session.

**The afterListTools approach is the recommended path.** Sketch:

1. In `filterToolsForContext` (or a sibling hook), when `get_datasource_information` is in the allowed set, call the adapter's `DatasourceInfo(ctx)` (with a short timeout — this runs on every ListTools call) and rewrite the tool's `Description`.
2. Cache the result with a short TTL (e.g., 30s) so ListTools stays cheap. Invalidate on datasource upsert/delete via `core.Service` signaling.
3. Description template:

   > `Returns detailed information about the connected datasource. Current values: name="dataclaw", type="postgres", database_name="the_look", schema_name="public", current_user="ekaya", version="PostgreSQL 18.2 ...". Call this tool to re-fetch if the session is long-running.`

4. If `DatasourceInfo` fails (e.g., DB temporarily unreachable), fall back to a generic description — don't fail ListTools.

Token cost: meaningful but bounded. A ~200-token description injected into every request is acceptable for a product whose whole purpose is database access. Document the tradeoff in the PR.

### 5. Tool response body

Even with description injection, the tool call must return the structured payload — clients with non-LLM consumers, logs, or description stripping need it. Shape matches `DatasourceInfo` plus `name` and `type` pulled from the already-loaded datasource record.

## Files to touch

- `internal/adapters/datasource/interfaces.go` — add `DatasourceIntrospector` interface, `DatasourceInfo` struct, and `DatasourceIntrospectorFactory` on `Registration`.
- `internal/adapters/datasource/postgres/` — implement introspector.
- `internal/adapters/datasource/mssql/` — implement introspector.
- `internal/adapters/datasource/factory.go` / `registry.go` — wire the new factory.
- `internal/core/service.go` + `internal/core/external.go` — add a `Service.GetDatasourceInformation(ctx)` method that decrypts the configured datasource, constructs the introspector, and returns `*DatasourceInfo`. Handler code must never touch adapters directly (per the CLAUDE.md rule: "httpapi and mcpserver are thin adapters; they must not touch store directly").
- `internal/mcpserver/datasource_info.go` — new file, `registerDatasourceInformationTool(...)`. Follow `health.go` for style.
- `internal/mcpserver/server.go`:
  - Call `registerDatasourceInformationTool` from `buildMCPServer` (line ~68).
  - Update `allowedTools` (line ~259) to unconditionally include `get_datasource_information` when a datasource is configured (the `HasDatasource` check in `filterToolsForContext` already guards the outer case).
  - Update the `AddAfterListTools` hook to rewrite the description with current datasource info (with caching — see §4).
  - Remove `DatasourceID` from `approvedQueryResponse` and `normalizeApprovedQuery`.
- Tests:
  - `internal/mcpserver/server_test.go` — assert the new tool is listed when a datasource is configured and hidden when not; assert description contains the current datasource facts; assert `datasource_id` is NOT present in `list_queries` output.
  - Adapter-level tests for each backend's introspection query.

## Verification

- `make check` passes.
- Reconnect crabby — tool list shows `get_datasource_information` with a description that embeds name/type/database/schema/user/version.
- Call the tool — returns the same info as a structured JSON payload.
- Inspect `list_queries` output — no `datasource_id` field.
- Delete the datasource — `get_datasource_information` disappears from the tool list (along with the other gated tools), matching existing `filterToolsForContext` behavior.

## Out of scope

- Multi-datasource support. DataClaw is single-datasource by design (see CLAUDE.md: "no fourth page or auth flow without an explicit product-level change").
- Exposing additional introspection (table lists, schema dumps). This tool is about the *connection*'s identity, not the data model.
- Changes to the HTTP API — `/api/datasources` can continue to return `datasource_id`.
- Dynamic tool *registration* changes via `mark3labs/mcp-go` notifications — use the `AddAfterListTools` description-rewrite path instead.

## Open questions for the implementer

- Should `get_datasource_information` also surface the adapter's `AdapterInfo.SQLDialect` (already available in `internal/adapters/datasource/interfaces.go:31`)? Probably yes — it's cheap and useful.
- TTL for the description cache: 30s is a reasonable default. Confirm or propose an alternative.
- If the datasource is temporarily unreachable during `ListTools`, should the description show stale cached info, a "connection unavailable" note, or a generic fallback? Recommend: stale info up to the TTL, then generic fallback with a `connection_status` hint.
