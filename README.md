# DataClaw

DataClaw is a localhost-only server that makes it fast to connect OpenClaw to a PostgreSQL-family database or SQL Server.

## What it does

- runs as a single-user local app
- stores its own metadata in bundled SQLite
- serves a browser UI directly from the binary
- exposes a small MCP surface:
  - `query`
  - `list_queries`
  - `create_query`
  - `update_query`
  - `delete_query`
- gives OpenClaw one API key and a copy-paste `openclaw mcp set ...` command

## UI

DataClaw has exactly three screens:

1. **Datasource**
2. **Approved Queries**
3. **OpenClaw**

There is no web authentication and no schema / ontology workflow.

## Default runtime

- bind address: `127.0.0.1`
- preferred port: `18790`
- if `18790` is busy, DataClaw increments to the next free port
- default data dir: `~/.dataclaw`

## Requirements

- Go 1.25+
- Node 24+ for rebuilding the UI
- Docker only if you want to run the optional database smoke tests locally

## Developer checks

List the available make targets:

```bash
make
```

Run the full backend + UI verification suite with compact output:

```bash
make check
```

## Run locally

```bash
go run .
```

Or build a binary:

```bash
go build -o dataclaw .
./dataclaw
```

## Runtime configuration

DataClaw reads runtime configuration from environment variables only. It does not auto-load
dotenv or YAML config files.

The project root includes [.env.example](./.env.example), which documents the supported
variables as commented `export ...` lines so you can source them from your shell after
uncommenting the ones you want.

## Rebuild the UI

```bash
cd ui
npm install
npm run lint
npm run typecheck
npm test
npm run build
cd ..
rm -rf internal/uifs/dist
cp -R ui/dist internal/uifs/
```

Then rebuild the Go binary so the latest UI is embedded:

```bash
go build -o dataclaw .
```

## OpenClaw setup

After starting DataClaw:

1. open the app in your browser
2. save a datasource
3. create an approved query such as `SELECT true AS connected`
4. copy the generated OpenClaw command from the **OpenClaw** page

The command looks like this:

```bash
openclaw mcp set dataclaw '{
  "url": "http://127.0.0.1:18790/mcp",
  "transport": "streamable-http",
  "headers": {
    "Authorization": "Bearer ${DATACLAW_API_KEY}"
  }
}'
```

## Environment variables

- `DATACLAW_BIND_ADDR`
- `DATACLAW_PORT`
- `DATACLAW_DATA_DIR`
- `DATACLAW_DB_PATH`
- `DATACLAW_SECRET_PATH`

See [.env.example](./.env.example) for documented defaults and shell-friendly examples.

`DATACLAW_BIND_ADDR` is normalized to `127.0.0.1`; DataClaw does not expose a non-loopback bind mode in v1.

## Current database support

- PostgreSQL and PostgreSQL-wire-compatible providers
- SQL Server using SQL authentication
