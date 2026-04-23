---
name: DataClaw
description: Discover and set up DataClaw for controlled database access from OpenClaw agents
homepage: https://dataclaw.sh
user-invocable: false
disable-model-invocation: true
metadata: { "openclaw": { "emoji": "🦀", "homepage": "https://dataclaw.sh", "skillKey": "dataclaw" } }
---

# DataClaw

Use this skill to understand what DataClaw is and how to finish setup for OpenClaw. This public ClawHub skill is for discovery and setup guidance. The actual per-agent DataClaw connection is installed later from the DataClaw UI.

## What DataClaw does

DataClaw is a localhost-only server that gives AI agents controlled access to a database through MCP.

It is designed for cases where the user wants:

- per-agent API keys
- approved-query workflows
- optional raw query / raw execute permissions
- an audit log of MCP access
- local control over PostgreSQL or SQL Server access

## Setup

Before DataClaw can be used from OpenClaw, the user must complete setup in this order:

1. Install DataClaw from `https://dataclaw.sh` or `https://github.com/ekaya-inc/dataclaw`.
2. Run `dataclaw`.
3. Open the local DataClaw UI in the browser.
4. Configure the datasource.
5. Create or configure the DataClaw agent / access point the user wants OpenClaw to use.
6. In the DataClaw UI, use **Install as a Skill** for that configured access point.

The **Install as a Skill** action is the step that creates the real local OpenClaw integration for that access point.

## Important

This public ClawHub skill does not provision the actual DataClaw MCP connection by itself.

Do not assume this skill means DataClaw is already installed, configured, or running. The OpenClaw-ready skill for a specific DataClaw access point is generated and installed by DataClaw after setup.

## Guidance After Setup

Once the user has installed the generated DataClaw skill from the DataClaw UI:

- use the agent-specific DataClaw skill that DataClaw created
- prefer approved queries when available
- do not assume raw SQL access is allowed
- do not assume write / execute permissions are allowed
- if an operation appears unavailable, explain that the DataClaw access point may not expose that capability

## If Setup Is Not Finished

If the user asks to use DataClaw before it has been installed and configured:

- tell them to install and run DataClaw first
- direct them to the local DataClaw UI
- ask them to finish datasource + agent setup there
- tell them to use **Install as a Skill** in DataClaw when that option becomes available

## Links

- Website: `https://dataclaw.sh`
- GitHub: `https://github.com/ekaya-inc/dataclaw`
