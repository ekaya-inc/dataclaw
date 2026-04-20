# DataClaw

DataClaw is a localhost-only server that makes it fast to connect multiple local AI agents to a PostgreSQL-family database or SQL Server with explicit MCP permissions.

## What it does

- runs as a single-user local app
- stores its own metadata in bundled SQLite
- serves a browser UI directly from the binary
- exposes a small MCP surface, gated per agent:
  - `query`
  - `execute`
  - `list_queries`
  - `execute_query`
- lets you create multiple named agents, each with:
  - its own API key
  - raw query / raw execute toggles
  - approved-query scope set to none, all, or selected queries

## UI

DataClaw has exactly three screens:

1. **Datasource**
2. **Agents**
3. **Approved Queries**

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

## Install

### macOS and Linux

```bash
curl --proto '=https' --tlsv1.2 -LsSf https://github.com/ekaya-inc/dataclaw/releases/latest/download/install.sh | sh
```

If you prefer a shorter URL, `https://dataclaw.sh/install.sh` redirects to the same installer.

To pin a specific version:

```bash
curl --proto '=https' --tlsv1.2 -LsSf https://github.com/ekaya-inc/dataclaw/releases/latest/download/install.sh | VERSION=v1.0.0 sh
```

### Windows

Download the latest `.zip` from [GitHub Releases](https://github.com/ekaya-inc/dataclaw/releases), extract it, add the extracted directory to your `PATH`, and run `dataclaw.exe`.

### Manual download

Pre-built binaries are available on [GitHub Releases](https://github.com/ekaya-inc/dataclaw/releases) for:

- macOS (Intel and Apple Silicon)
- Linux (x86_64 and ARM64)
- Windows (x86_64)

Each release also includes `checksums.txt`, `.env.example`, and this README.

## Run after install

```bash
dataclaw
```

DataClaw starts on `http://127.0.0.1:18790` by default. If that port is busy, it increments to the next free localhost port and logs the actual URL to open in your browser.

## Developer checks

List the available make targets:

```bash
make
```

Run the full backend + UI verification suite with compact output:

```bash
make check
```

Build the embedded UI and a local binary:

```bash
make build
```

Start the server, rebuilding embedded UI assets first when the checked-in bundle is stale:

```bash
make run
```

Run the server against `ui/dist` on disk (skips the embed) for a live dev loop:

```bash
make dev
```

In a second terminal, rebuild `ui/dist` on every save:

```bash
make dev-ui
```

`make dev` and `make dev-ui` do not refresh `internal/uifs/dist`, so run `make run` or `make check` before shipping.

## Run from source

```bash
go run .
```

Or build a binary:

```bash
make build
./bin/dataclaw
```

## Runtime configuration

DataClaw reads runtime configuration from environment variables only. It does not auto-load
dotenv or YAML config files.

The project root includes [.env.example](./.env.example), which documents the supported
variables as commented `export ...` lines so you can source them from your shell after
uncommenting the ones you want.

## Rebuild the UI

`make run` rebuilds `internal/uifs/dist` when `ui/src` is newer. For an interactive dev loop, use `make dev` + `make dev-ui`. `make check` runs the full UI verification suite.

## Agent setup

After starting DataClaw:

1. open the app in your browser
2. save a datasource
3. create approved queries if the agent should use approved-query tools
4. create an agent on the **Agents** page
5. reveal or rotate the agent key
6. copy the generated MCP config snippet and set `DATACLAW_API_KEY` to that agent key

A typical MCP config snippet looks like this:

```json
{
  "mcpServers": {
    "warehouse-analyst-123456": {
      "type": "http",
      "url": "http://127.0.0.1:<actual-port>/mcp",
      "headers": {
        "Authorization": "Bearer ${DATACLAW_API_KEY}"
      }
    }
  }
}
```

Copy the generated snippet from the agent detail page so the agent name and localhost port match your running app.

OpenClaw-style bundle installs are also exposed per access point at:

```text
http://127.0.0.1:<actual-port>/bundles/<agent-slug>
```

For example, if the access point slug is `marketing`, an OpenClaw client can install it from `http://127.0.0.1:<actual-port>/bundles/marketing`. The manifest points at a downloadable zip that contains the matching skill directory and MCP server config for that access point.

## Environment variables

- `DATACLAW_BIND_ADDR`
- `DATACLAW_PORT`
- `DATACLAW_DATA_DIR`
- `DATACLAW_DB_PATH`
- `DATACLAW_SECRET_PATH`
- `DATACLAW_LOG_LEVEL`

See [.env.example](./.env.example) for documented defaults and shell-friendly examples.

`DATACLAW_BIND_ADDR` is normalized to `127.0.0.1`; DataClaw does not expose a non-loopback bind mode in v1.

## Current database support

- PostgreSQL and PostgreSQL-wire-compatible providers
- SQL Server using SQL authentication
