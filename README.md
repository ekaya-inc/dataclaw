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

DataClaw starts on `http://127.0.0.1:18790` by default. If that port is busy, it increments to the next free localhost port and logs the actual URL to open in your browser.

## Environment variables

See [.env.example](./.env.example) for documented defaults and shell-friendly examples.

## ClawHub

The public DataClaw discovery skill lives in [`skills/dataclaw`](./skills/dataclaw). That directory is published to ClawHub from [`.github/workflows/release.yml`](./.github/workflows/release.yml) after the normal GitHub release job succeeds for a standard `v*` release tag. [`.github/workflows/clawhub-publish.yml`](./.github/workflows/clawhub-publish.yml) is validation-only for pull requests. The skill directory carries its own `MIT-0` license file for registry distribution.

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
