# CONSIDER: LLM/DAG Semantic Ontology / Business-Metrics Layer

## Status

Split decision after the Ekaya Engine deep dive on 2026-05-02:

- **Planned in the main PRD:** a lightweight DataClaw schema registry plus Admin-authored table/column/project/glossary metadata managed through MCP tools.
- **Still CONSIDER-only:** Ekaya-style LLM/DAG ontology extraction, semantic reasoning, ontology Forge/setup UI, automatic glossary enrichment, and business-metrics generation.

## Source Context

- CrystalDBA's related-projects list references Wren MCP Server as a semantic engine for business intelligence across Postgres and other databases: https://github.com/crystaldba/postgres-mcp
- Ekaya Engine has a mature schema/ontology system in `../ekaya-engine`, including schema registry tables, table/column metadata, questions, project knowledge, glossary, ontology import/export, and an LLM/DAG extraction workflow.

## Why This Is Large Scope

A semantic ontology layer that DataClaw generates or reasons over would require DataClaw to own business concepts, metrics definitions, joins, dimensions, governance, lifecycle management, likely LLM credentials, and probably UI/editor workflows. That is larger than adapter-backed schema/query tools and MCP-authored metadata.

## Potential Value

- Business-friendly query planning over raw schemas.
- Reusable metrics and relationship definitions.
- Better natural-language grounding for analytics agents.
- Transferable project context if paired with a redacted import/export bundle.

## Risks / Open Questions

- Who authors and approves generated ontology definitions?
- How are changes versioned, tested, and rolled back?
- How does the layer remain database-neutral while still mapping to engine-specific capabilities?
- Does this overlap with existing BI/semantic-layer tools the user already uses?
- Should DataClaw ever own LLM calls, or should the user's MCP client/LLM remain the intelligence layer?

## If Revisited

Start with the planned DataClaw schema registry and MCP-authored metadata first. Revisit LLM/DAG extraction only after schema exploration, safe query execution, metadata CRUD, questions, pending changes, and import/export are stable.
