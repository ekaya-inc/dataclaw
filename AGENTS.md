# DataClaw Agent Notes

Scope: this file applies to the repository root and everything under it.

## Repo conventions

- Run `make check` before declaring substantive work complete.
- Keep `.env.example`, the README runtime configuration section, and `internal/config/config.go` in sync when adding or changing supported environment variables.
- DataClaw does not auto-load dotenv files. `.env.example` is documentation for shell-sourced `export` statements only.
- After changing files under `ui/src` or other shipped UI assets, rebuild the frontend and refresh `internal/uifs/dist` so the embedded UI matches source.

## Editing guidance

- Keep the app localhost-only unless the task explicitly changes that product constraint.
- Prefer small, reversible diffs and reuse the existing backend/UI patterns before introducing new abstractions.
