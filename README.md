# DataClaw

![DataClaw installation demo](https://dataclaw.sh/assets/demo/dataclaw-installation-demo.gif)

DataClaw is a localhost-only server that makes it fast to connect multiple local AI agents to a database each with explicit MCP permissions on what tools they can run and what queries they can execute.  All MCP Access is logged in the dashboard.

## How it works

- runs as a single-user local app on the same system as OpenClaw
- stores its own metadata in bundled SQLite
- serves a browser UI directly from the binary
- lets you create multiple named agents, each with:
  - its own API key
  - raw query / raw execute toggles
  - approved-query scope set to none, all, or selected queries

## Install

### macOS and Linux

Install in `/usr/local/bin`:

```bash
curl -fsSL https://dataclaw.sh/install.sh | sh
```

Or you can set where you want it to install (recommended):

```bash
curl -fsSL https://dataclaw.sh/install.sh | INSTALL_DIR=$HOME/.local/bin/ sh
```

### Windows or Manual Download

Download the latest `.zip` from [GitHub Releases](https://github.com/ekaya-inc/dataclaw/releases), extract it, add the extracted directory to your `PATH`, and run `dataclaw.exe`.

### Run after install

```bash
dataclaw
```

DataClaw starts two loopback listeners by default: the admin web UI/API on `http://127.0.0.1:18790` and MCP on `http://127.0.0.1:18791/mcp`. If either preferred port is busy, DataClaw probes upward independently and logs the actual admin and MCP URLs.

## Runtime configuration

See [.env.example](./.env.example) for shell-friendly examples. DataClaw does not auto-load dotenv files; export the variables in your shell or source them before starting `dataclaw`.

Admin sign-in uses username `admin`. Set an admin password before exposing the admin listener beyond loopback:

```bash
export DATACLAW_ADMIN_PASSWORD='replace-with-a-strong-password'
```

If neither `DATACLAW_ADMIN_PASSWORD` nor `admin.password` in the JSON config is set, DataClaw accepts the default password `admin` and logs a WARN-level startup message.

| Variable | Default | Notes |
| --- | --- | --- |
| `DATACLAW_CONFIG_PATH` | unset | Optional JSON config file. If unset, DataClaw reads `$DATACLAW_DATA_DIR/config.json` when it exists. An explicit missing path fails startup. |
| `DATACLAW_DATA_DIR` | `~/.dataclaw` | Base data directory for local DataClaw state. |
| `DATACLAW_DB_PATH` | `$DATACLAW_DATA_DIR/dataclaw.sqlite` | SQLite metadata database path. |
| `DATACLAW_SECRET_PATH` | `$DATACLAW_DATA_DIR/secret.key` | Encryption key path for stored datasource credentials. |
| `DATACLAW_LOG_LEVEL` | `info` | Structured log level: `debug`, `info`, `warn`, `warning`, or `error` (case-insensitive). |
| `DATACLAW_ADMIN_BIND_ADDR` | `127.0.0.1` | Admin web UI/API bind address. May be loopback, `0.0.0.0`, an IP, or an FQDN. |
| `DATACLAW_ADMIN_PORT` | `18790` | Preferred admin port. If busy, DataClaw probes upward. |
| `DATACLAW_ADMIN_ADVERTISED_HOST` | bind address | Hostname/IP used when generating admin URLs; do not include scheme or port. |
| `DATACLAW_ADMIN_ADVERTISED_BASE_URL` | derived | Absolute `http`/`https` admin base URL. Overrides advertised host and probed port in generated URLs. |
| `DATACLAW_ADMIN_TLS` | `false` | `true` advertises HTTPS. With no cert/key this is reverse-proxy mode and the local listener remains plain HTTP. |
| `DATACLAW_ADMIN_TLS_CERT_FILE` | unset | Admin TLS certificate file. Must be set together with key file to serve TLS directly. |
| `DATACLAW_ADMIN_TLS_KEY_FILE` | unset | Admin TLS key file. Must be set together with certificate file. |
| `DATACLAW_ADMIN_PASSWORD` | `admin` | Admin password. The default works but emits a WARN-level startup message. |
| `DATACLAW_ADMIN_SESSION_TTL` | `12h` | Normal admin session cookie lifetime. |
| `DATACLAW_ADMIN_SESSION_LONG_TTL` | `720h` | “Keep me signed in” admin session lifetime; capped at 90 days. |
| `DATACLAW_MCP_BIND_ADDR` | `127.0.0.1` | MCP listener bind address. The MCP listener serves MCP routes only. |
| `DATACLAW_MCP_PORT` | `18791` | Preferred MCP port. If busy, DataClaw probes upward independently of the admin listener. |
| `DATACLAW_MCP_ADVERTISED_HOST` | bind address | Hostname/IP used when generating MCP URLs; do not include scheme or port. |
| `DATACLAW_MCP_ADVERTISED_BASE_URL` | derived | Absolute `http`/`https` MCP base URL used in generated agent install manifests. |
| `DATACLAW_MCP_TLS` | `false` | TLS semantics match admin TLS. |
| `DATACLAW_MCP_TLS_CERT_FILE` | unset | MCP TLS certificate file. Must be set together with key file to serve TLS directly. |
| `DATACLAW_MCP_TLS_KEY_FILE` | unset | MCP TLS key file. Must be set together with certificate file. |
| `DATACLAW_BIND_ADDR` | `127.0.0.1` | Deprecated compatibility alias for `DATACLAW_ADMIN_BIND_ADDR`; affects admin only and logs a warning. |
| `DATACLAW_PORT` | `18790` | Deprecated compatibility alias for `DATACLAW_ADMIN_PORT`; affects admin only and logs a warning. |

Equivalent JSON config can be placed at `$DATACLAW_DATA_DIR/config.json` or the path named by `DATACLAW_CONFIG_PATH`:

```json
{
  "config_version": 1,
  "admin": {
    "bind_addr": "127.0.0.1",
    "port": 18790,
    "password": "replace-with-a-strong-password"
  },
  "mcp": {
    "bind_addr": "127.0.0.1",
    "port": 18791
  }
}
```

Environment variables override JSON values. Unknown JSON fields, invalid ports/durations/URLs, and partial TLS cert/key pairs fail startup.

## ClawHub

The public DataClaw setup skill lives in [`skills/dataclaw`](./skills/dataclaw). It publishes to ClawHub as `dataclaw-setup`, not as the runtime access-point skill a user talks to later. Generated access-point skills remain separate and are installed from the DataClaw UI with names such as `dataclaw-marketing`. The public setup skill directory is published from [`.github/workflows/release.yml`](./.github/workflows/release.yml) after the normal GitHub release job succeeds for a standard `v*` release tag. [`.github/workflows/clawhub-publish.yml`](./.github/workflows/clawhub-publish.yml) is validation-only for pull requests. The skill directory carries its own `MIT-0` license file for registry distribution.

Stable tags publish with the ClawHub `latest` tag. Prerelease versions containing `-alpha`, `-beta`, or `-rc` publish with the ClawHub `beta` tag.

Example:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow requires the repository secret `CLAWHUB_TOKEN`. Pull requests that touch the skill, release workflow, validation workflow, or this README run validation only and do not publish.

## Testing

Use `make check` for the standard repo gate.

Coverage workflows are available separately:

- `make coverage-go` for package-local Go coverage
- `make coverage-go-instrumented` for repo-wide instrumented Go coverage via `-coverpkg=./...`
- `make coverage-go-integration` for runtime coverage from an instrumented binary
- `make coverage-ui` for Vitest UI coverage
- `make coverage-gate` for the first-pass provisional floors on critical backend packages and the targeted UI coverage set
- `make coverage` for the primary Go package and UI coverage runs

The initial coverage rollout is measurement-first. Thin runtime shells such as `main.go` and `internal/uifs`, plus the full `internal/app.Run` bootstrap path, are informational in the first pass rather than gating targets.

First-pass provisional floors:

- `internal/config` package-local Go coverage: `>= 90%`
- `internal/security` package-local Go coverage: `>= 75%`
- `internal/httpapi` package-local Go coverage: `>= 55%`
- `internal/adapters/datasource` package-local Go coverage: `>= 35%`
- targeted UI coverage include set (`SqlEditor`, `useStoredParameterValues`, `useSupportDismissed`): `>= 90%` statements and `>= 80%` branches

CI now runs `make coverage-gate` after `make check` on pull requests and `main`.
