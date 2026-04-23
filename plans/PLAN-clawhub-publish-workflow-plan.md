# ClawHub Publish Workflow Plan

## Goal

Add a GitHub Actions workflow in this repository that publishes the public DataClaw discovery skill at `skills/dataclaw/` to ClawHub in a repeatable, low-surprise way.

This plan is intentionally written as a handoff artifact for a fresh session working inside `/Users/damondanieli/go/src/github.com/ekaya-inc/dataclaw`.

## Current State

- The public ClawHub skill now exists at [`skills/dataclaw/SKILL.md`](</Users/damondanieli/go/src/github.com/ekaya-inc/dataclaw/skills/dataclaw/SKILL.md>).
- Existing GitHub Actions in this repo:
  - [`/.github/workflows/pr-checks.yml`](</Users/damondanieli/go/src/github.com/ekaya-inc/dataclaw/.github/workflows/pr-checks.yml>)
  - [`/.github/workflows/build-main.yml`](</Users/damondanieli/go/src/github.com/ekaya-inc/dataclaw/.github/workflows/build-main.yml>)
  - [`/.github/workflows/release.yml`](</Users/damondanieli/go/src/github.com/ekaya-inc/dataclaw/.github/workflows/release.yml>)
- Binary releases already use top-level `v*` tags. The ClawHub workflow must not accidentally trigger or be triggered by that release workflow.

## Research Notes

- ClawHub skill publish command shape is documented as:

```bash
clawhub skill publish ./my-skill \
  --slug my-skill \
  --name "My Skill" \
  --version 1.0.0 \
  --tags latest \
  --changelog "Initial release"
```

- Official docs for skills describe `clawhub skill publish` and `clawhub sync`, but the official reusable GitHub workflow currently documented by ClawHub is for **packages/plugins**, not skills.
- Therefore, for DataClaw, the workflow should use the ClawHub CLI directly rather than depending on a non-existent skill-publish reusable workflow.
- ClawHub docs say published skills are licensed under MIT-0. Confirm this is acceptable for the public `skills/dataclaw/` content before first publish.

## Recommended Workflow Design

### Trigger strategy

Use two trigger modes:

1. `workflow_dispatch`
   - safest path for the first publish
   - lets the operator manually enter a version and changelog
   - avoids accidental registry pollution

2. tag push for a **namespaced tag pattern**
   - recommended pattern: `clawhub/dataclaw/v*`
   - example: `clawhub/dataclaw/v0.1.0`
   - this does **not** overlap with the existing binary release workflow, which listens on `v*`

Do **not** use bare `v*` for the skill publish workflow.

### Skill metadata choices

Publish the skill folder:

- path: `skills/dataclaw`
- slug: `dataclaw`
- name: `DataClaw`

Version should come from either:

- `workflow_dispatch` input, or
- tag parsing from `refs/tags/clawhub/dataclaw/vX.Y.Z`

Tags sent to ClawHub:

- default: `latest`
- optional: `beta` when the version contains prerelease markers such as `-beta`, `-rc`, or `-alpha`

### Authentication

Use a repository secret:

- `CLAWHUB_TOKEN`

Implementation note:

- ClawHub docs clearly reference registry auth and show GitHub Actions examples passing `clawhub_token` for package workflows.
- If the `clawhub` CLI accepts `CLAWHUB_TOKEN` directly as an env var, use that.
- If it does not, write a temporary config under `CLAWHUB_CONFIG_PATH` during the workflow, then delete it in a cleanup step.

The workflow should fail clearly if `CLAWHUB_TOKEN` is missing on publish paths.

## Files To Add Or Modify

### New workflow

Add:

- [`/.github/workflows/clawhub-publish.yml`](</Users/damondanieli/go/src/github.com/ekaya-inc/dataclaw/.github/workflows/clawhub-publish.yml>)

### Documentation updates

Update:

- [`README.md`](</Users/damondanieli/go/src/github.com/ekaya-inc/dataclaw/README.md>)

Add a short section explaining:

- where the public skill lives
- how to publish it
- what tag format to use
- required secret: `CLAWHUB_TOKEN`

Optional:

- add a small `.github` or `plans` note if the repo keeps release-process docs separate

## Workflow Requirements

The workflow should do all of the following:

1. Check out the repo.
2. Resolve publish mode:
   - PR / dry validation
   - manual publish
   - tag publish
3. Determine:
   - skill directory: `skills/dataclaw`
   - slug: `dataclaw`
   - name: `DataClaw`
   - version
   - changelog text
   - ClawHub tags
4. Validate preconditions:
   - `skills/dataclaw/SKILL.md` exists
   - version is semver without a leading `v` when passed to ClawHub
5. Install or invoke the ClawHub CLI.
6. Authenticate.
7. Run a non-publishing validation path on pull requests if possible.
8. Publish only from trusted events.
9. Surface the resolved slug/version/path in the workflow logs.

## Suggested Workflow Shape

### Events

Recommended event block:

```yaml
on:
  pull_request:
    paths:
      - "skills/dataclaw/**"
      - ".github/workflows/clawhub-publish.yml"
      - "README.md"
  workflow_dispatch:
    inputs:
      version:
        description: "Skill version to publish (semver without leading v)"
        required: true
      changelog:
        description: "Changelog text for this ClawHub version"
        required: true
  push:
    tags:
      - "clawhub/dataclaw/v*"
```

### Jobs

Recommended jobs:

1. `validate`
   - runs on `pull_request`
   - does not publish
   - checks the skill folder, metadata presence, and parsed values
   - optionally runs `npx clawhub skill publish ...` only if the CLI supports a true dry-run mode for skill publish; if not, keep this job as static validation only

2. `publish`
   - runs on `workflow_dispatch` and matching tag pushes
   - requires `CLAWHUB_TOKEN`
   - publishes exactly one skill folder

## Version Parsing Rules

### Manual dispatch

Input:

- `version`: use as-is, but strip leading `v` defensively
- `changelog`: use as-is

### Tag publish

Expected ref format:

- `refs/tags/clawhub/dataclaw/v0.1.0`

Parsing rules:

- ensure prefix is exactly `clawhub/dataclaw/`
- final segment must start with `v`
- publish version passed to ClawHub must be `0.1.0`, not `v0.1.0`
- changelog can default to:
  - `Publish DataClaw skill version 0.1.0`

### Registry tags

Recommended:

- `latest` for stable versions
- `beta` for prerelease versions

Implementation rule:

- if version contains `-alpha`, `-beta`, or `-rc`, publish with `--tags beta`
- otherwise use `--tags latest`

## CLI Invocation

Start from this command:

```bash
npx clawhub skill publish skills/dataclaw \
  --slug dataclaw \
  --name "DataClaw" \
  --version "${VERSION}" \
  --tags "${PUBLISH_TAGS}" \
  --changelog "${CHANGELOG}"
```

Notes:

- Prefer `npx clawhub ...` to avoid adding a permanent repo-level Node dependency just for publishing.
- If `npx` resolution is flaky in CI, switch to installing the CLI globally in the workflow step:

```bash
npm install -g clawhub
clawhub skill publish ...
```

## Authentication Options

### Preferred

Try:

```yaml
env:
  CLAWHUB_TOKEN: ${{ secrets.CLAWHUB_TOKEN }}
```

and let the CLI consume it directly if supported.

### Fallback

If the CLI requires a config file instead of reading the env var directly:

1. set `CLAWHUB_CONFIG_PATH` to a temporary file in `${RUNNER_TEMP}`
2. write the minimum token-bearing config the CLI expects
3. run publish
4. delete the file in a final cleanup step

A new session implementing this plan should check the current CLI behavior before finalizing the workflow.

## Validation Path For PRs

Because the official ClawHub docs do not clearly document `--dry-run` for `skill publish`, keep PR validation conservative:

1. verify `skills/dataclaw/SKILL.md` exists
2. verify frontmatter contains at least:
   - `name`
   - `description`
3. verify the path matches the expected publish target
4. optionally run a lightweight parser check with shell or Node if needed

If `clawhub skill publish --dry-run` is confirmed to work in the current CLI, upgrade the PR job to use it.

## README Update Requirements

Add a section such as:

`## ClawHub`

Include:

- public skill path: `skills/dataclaw`
- publish secret: `CLAWHUB_TOKEN`
- manual publish trigger
- tag publish pattern: `clawhub/dataclaw/v*`
- first publish example:

```bash
git tag clawhub/dataclaw/v0.1.0
git push origin clawhub/dataclaw/v0.1.0
```

## Acceptance Criteria

The work is complete when:

1. `skills/dataclaw` can be published to ClawHub from GitHub Actions.
2. The workflow does not conflict with the existing binary release workflow.
3. Pull requests touching the skill run validation without publishing.
4. Manual publish via `workflow_dispatch` works with version and changelog inputs.
5. Tag publish works with `clawhub/dataclaw/v*`.
6. The repo documents the publish process and required secret.

## Verification Checklist

### Local / static verification

Run:

```bash
git diff -- .github/workflows/clawhub-publish.yml README.md skills/dataclaw/SKILL.md
```

Check:

- trigger patterns are correct
- no overlap with `release.yml` tag handling
- resolved skill path is `skills/dataclaw`

### Workflow verification

1. Open a PR that changes only the skill or workflow.
2. Confirm the PR workflow validates and does not publish.
3. Run `workflow_dispatch` with a throwaway prerelease version if safe, for example `0.1.0-beta.1`.
4. Confirm the resulting ClawHub listing/version looks correct.
5. After that, test a real tag push:

```bash
git tag clawhub/dataclaw/v0.1.0
git push origin clawhub/dataclaw/v0.1.0
```

## Risks And Mitigations

### Risk: release workflow collision

- Mitigation: use `clawhub/dataclaw/v*`, not `v*`

### Risk: unclear CLI auth mode in CI

- Mitigation: implement env-token first, config-file fallback second

### Risk: first publish fails because the skill content is treated as MIT-0 by ClawHub

- Mitigation: confirm the public skill text is acceptable under that distribution model before first publish

### Risk: PR validation accidentally publishes

- Mitigation: separate `validate` and `publish` jobs and gate publish jobs to trusted events only

### Risk: ClawHub CLI command shape drifts

- Mitigation: check the current official docs before implementation; this plan is based on docs current as of 2026-04-23

## Recommended Implementation Order

1. Read current ClawHub docs one more time to confirm CLI flags and auth behavior.
2. Add `/.github/workflows/clawhub-publish.yml`.
3. Add README publish instructions.
4. Validate YAML and event logic locally.
5. Open a PR and confirm non-publishing validation behavior.
6. Configure `CLAWHUB_TOKEN` in GitHub repo secrets.
7. Run first manual publish via `workflow_dispatch`.
8. After success, use the namespaced tag flow for normal releases.

## Sources

- [OpenClaw ClawHub docs](https://github.com/openclaw/openclaw/blob/main/docs/tools/clawhub.md)
- [ClawHub CLI docs](https://github.com/openclaw/clawhub/blob/main/docs/cli.md)
- [ClawHub quickstart](https://github.com/openclaw/clawhub/blob/main/docs/quickstart.md)
- [ClawHub skill format](https://github.com/openclaw/clawhub/blob/main/docs/skill-format.md)
- [Example multi-skill publish repo](https://github.com/autonomys/openclaw-skills)
