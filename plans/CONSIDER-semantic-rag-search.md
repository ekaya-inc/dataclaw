# CONSIDER: Full Semantic / RAG / Hybrid Search Subsystem

## Status

Not planned for near-term implementation. Capture only.

## Source Context

pgEdge documents embedding generation, BM25+MMR hybrid search, vector table discovery, semantic-search setup prompts, and provider configuration: https://github.com/pgEdge/pgedge-postgres-mcp and https://raw.githubusercontent.com/pgEdge/pgedge-postgres-mcp/main/docs/reference/tools.md

## Why This Is Large Scope

A full semantic/RAG subsystem would add embedding providers, vector schema detection, token budgets, chunking, relevance ranking, provider credentials, and potentially database-specific vector extensions. This is broader than safe SQL/MCP schema/query tooling.

## Potential Value

- Search over large text/document tables.
- Agent-friendly retrieval workflows.
- Progressive output modes for token control.

## Risks / Open Questions

- Which embedding providers are supported?
- How are credentials and private content protected?
- How should SQL Server parity work if vector features differ?
- Should this be built into DataClaw or integrated through external services?

## If Revisited

Start with progressive output contracts (`ids_only`, `summary`, `full`) and adapter capability probes. Do not add provider dependencies without explicit approval.
