# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make check` — full quality gate: `go mod tidy` drift check, `gofmt`, `go test ./...`, `go build ./...`, UI deps/lint/typecheck/test/build. Run before declaring substantive work complete.
- `make run` — rebuilds the embedded UI if `ui/src` (or listed UI config files) is newer than `internal/uifs/dist/index.html`, then `go run .`.
- `make dev` + `make dev-ui` (two terminals) — dev loop. `make dev` runs the Go server with `DATACLAW_UI_DIR=$(pwd)/ui/dist`, so the handler reads the bundle from disk on every request instead of using the embed. `make dev-ui` runs `vite build --watch` to keep `ui/dist` fresh. Refresh the browser to pick up UI changes; restart `make dev` for Go changes. Does NOT refresh `internal/uifs/dist`, so run `make run` (or `make check`) before shipping.
- `go run .` — start the server without touching embedded assets (uses whatever is already in `internal/uifs/dist`).
- `go test ./internal/core/...` — run tests for a single Go package. Use `go test -run TestName ./pkg/sql` for a single test.
- `npm --prefix ui test -- --run` — UI tests once (vitest). `npm --prefix ui test:watch` for watch mode. `npm --prefix ui run lint|typecheck|build` for individual UI gates.

## Architecture

`main.go` → `internal/app.Run` wires the whole process. Order matters:

1. `internal/config` loads env vars (`DATACLAW_*`), normalizes `BindAddr` to `127.0.0.1` (loopback-only is a product constraint, not just a default).
2. `internal/security.LoadOrCreateSecret` loads/creates the AES key at `DATACLAW_SECRET_PATH`; used to encrypt datasource configs and the OpenClaw API key before they go into SQLite.
3. `internal/store.Open` opens SQLite via `modernc.org/sqlite` and applies `migrations/*.sql` (embedded via `migrations/embed.go`). Tables: `datasources`, `approved_queries`, `openclaw_credentials`, `app_settings`.
4. `internal/runtime.ListenIncrement` binds the preferred port, incrementing up to 100 times if busy — the actual port isn't known until this returns.
5. `internal/core.Service` is the single orchestrator — **all business logic goes through it**. `httpapi` and `mcpserver` are thin adapters; they must not touch `store` directly.
6. `internal/httpapi` mounts `/api/*` routes on the shared `http.ServeMux`.
7. `internal/mcpserver` mounts `/mcp` (streamable HTTP via `mark3labs/mcp-go`), bearer-token gated by `core.Service.ValidateOpenClawKey`. MCP tools: `query`, `list_queries`, `create_query`, `update_query`, `delete_query`.
8. `internal/uifs` embeds `ui/dist` via `go:embed`; `uifs.Load()` returns an `fs.FS` that normally reads the embed, but switches to `os.DirFS($DATACLAW_UI_DIR)` when that env var is set (dev mode). `app.registerUIRoutes` serves it with SPA fallback (unknown paths → `index.html`), explicitly skipping `/api/` and `/mcp`.

The UI lives in `ui/` (React 18 + Vite + Tailwind + react-router + CodeMirror). It has **exactly three pages** — Datasource, Approved Queries, OpenClaw — and no auth. Don't add a fourth page or an auth flow without an explicit product-level change. Built assets are checked into `internal/uifs/dist/` so the Go binary is self-contained; `make run` keeps them fresh.

`pkg/sql` is SQL validation and parameter binding (uses `corazawaf/libinjection-go`). `validateReadOnlySQL` gates raw queries through the `query` MCP tool and `/api/queries/test`; `validateStoredSQL` gates approved queries. Both run **before** anything hits a real database. When adding query features, route through these — don't bypass.

Datasource support is PostgreSQL (`jackc/pgx/v5`) and SQL Server (`microsoft/go-mssqldb`), dispatched inside `internal/core/external.go`. Adding another driver means extending the executor, the `tester`, and the `UpsertDatasource` type guard in `core/service.go`.

## Non-obvious rules

- Keep `.env.example`, the README "Environment variables" section, and `internal/config/config.go` in sync when adding/removing env vars. DataClaw does **not** auto-load dotenv — `.env.example` is shell-sourced documentation only.
- When you edit anything under `ui/src` (or UI config like `vite.config.ts`, `tailwind.config.js`, `package.json`), `make run` will rebuild and refresh `internal/uifs/dist` for you. If you're building manually (`go build`), you must rebuild the UI and copy `ui/dist` → `internal/uifs/dist` first, or the binary ships stale assets.
- Localhost-only is a hard constraint. `normalizeBindAddr` throws away any non-loopback value — don't "fix" that without an explicit product change.
- Datasource `Config` is encrypted at rest. Always call `Service.GetDatasource` / `requireDatasource` (which decrypt) rather than reading `store.GetDatasource` directly from handler code.
- Prefer small, reversible diffs and reuse existing backend/UI patterns before introducing new abstractions (per `AGENTS.md`).
