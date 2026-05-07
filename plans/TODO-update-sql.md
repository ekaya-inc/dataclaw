# TODO: SQL MCP Improvements

This roadmap is ordered from near-term adapter-boundary work to later, capability-gated SQL/MCP intelligence. See `.omx/plans/prd-mcp-database-sql-improvements.md`, `.omx/plans/test-spec-mcp-database-sql-improvements.md`, and `.omx/context/ekaya-schema-ontology-review-20260502T193120Z.md` for the consensus plan and Ekaya schema/ontology review.

## Phase 0: Adapter Boundary, Permissions, and Tool Safety Metadata

- Remove hardcoded SQL Server-specific pagination guidance from generic MCP tool descriptions; replace with adapter-neutral wording or adapter-provided capability text.
- Add focused tests/checks so new SQL/MCP capabilities do not place database-specific SQL, DMV/catalog names, execution policy, or engine-specific prose in MCP/core/UI layers.
- Keep legitimate product-surface labels allowed: datasource type names, display labels, SQL dialect labels, and adapter-local tests.
- Assert MCP annotations for query, count, execute, future schema tools, and local metadata write tools.
- Model capabilities in two layers: static adapter support and runtime availability/limitations for the current datasource/user.
- Add a narrow Admin/agent permission for schema/ontology metadata management; do not reuse raw execute permission.

## Phase 1: Schema Exploration MVP

- Add a dedicated schema exploration MCP tool behind a generic adapter contract.
- Support PostgreSQL and SQL Server introspection behind one server-level contract.
- Add optional filters:
  - `schema_name`
  - object/table name
  - compact/full detail mode
- Add automatic summary mode for large schemas.
- Include table/object counts where cheaply available and truncated-result hints so agents can narrow follow-up calls.
- Return structured `limitations` / `unavailable_reason` fields when metadata is unsupported or unavailable.

## Phase 0/1 Exit Checklist and Integration Risks

- Keep the MCP/core schema exploration surface database-neutral: no SQL Server/PostgreSQL catalog SQL, DMV names, pagination rules, or engine-specific prose outside adapter-local packages/tests.
- Treat PostgreSQL and SQL Server as parity lanes for Phase 1: any shared schema-exploration field must either be implemented by both adapters or returned with an adapter-local `limitations` / `unavailable_reason` entry.
- Verify the generic MCP schema tool is read-only and backed only by the adapter contract; it must not mutate the user database or DataClaw-local schema/ontology metadata.
- Preserve Phase 2+ boundaries during final integration: do not add schema registry tables, metadata CRUD, refresh/sync tools, LLM/DAG ontology extraction, RAG/search, or UI setup flows in the Phase 0/1 merge.
- Final verification gate after implementation lanes merge: run `make check` from a clean worktree and record PASS/FAIL evidence before marking Phase 0/1 complete.

## Phase 2: Persisted Schema Registry and MCP Schema Refresh

- Add DataClaw store tables/models/repositories for schema tables, columns, and relationships.
- Add a core schema refresh service that calls only the generic adapter schema contract.
- Add `refresh_schema` / `sync_schema` as an Admin-gated MCP tool that mutates only DataClaw local metadata, never the user database.
- Report tables/columns/relationships added, removed, or modified, plus adapter limitations and pending changes created.
- Avoid expensive row counts by default; use estimates or explicit opt-in for costly counts.

## Phase 3: MCP-Authored Schema/Ontology Metadata

- Add table metadata: table type, description, usage notes, lifecycle flags, preferred alternative, features, provenance, actor, timestamps.
- Add column metadata: classification path, purpose, semantic type, role, description, enum values/features, identifier/timestamp/boolean/monetary features, clarification flags, sensitivity override, provenance, actor, timestamps.
- Add project knowledge tools for terminology, business rules, enumerations, and conventions.
- Add business glossary tools for terms, definitions, aliases, optional defining SQL or approved-query references, output columns, review status, and provenance.
- Expose `get_schema`, `get_ontology`, `get_column_metadata`, `update_table`, `delete_table_metadata`, `update_column`, `delete_column_metadata`, knowledge tools, and glossary tools through MCP for Admin-capable agents.
- Ensure `get_ontology` uses progressive disclosure and requires table filters for full column depth.
- Do not add an `extract_ontology` UI, route, or MCP tool.

## Phase 4: Deterministic Questions and Pending Schema-Change Review

- Add deterministic validators that create questions for missing/ambiguous metadata; do not call an LLM.
- Add pending schema-change records from refresh deltas: new/dropped table, new/dropped/modified column, enum changes, and FK pattern changes.
- Add MCP review tools for listing and resolving/skipping/escalating/dismissing questions.
- Add MCP review tools for listing/approving/rejecting pending schema changes.
- Require actual metadata updates before resolving questions whenever the question is meant to enrich the ontology.

## Phase 5: Versioned Schema/Ontology Bundle Portability

- Adapt Ekaya's bundle concept later for DataClaw-local schema/ontology/approved-query state.
- Export schema snapshot, relationships, table/column metadata, project knowledge, glossary terms, approved queries, format/version, and explicit security flags.
- Exclude datasource credentials, AI config, agent API keys, and local secrets.
- Validate target datasource type and schema shape before import; report structured missing/unexpected table/column/relationship problems.

## Phase 6: Read-Only Guardrails and Query Tool Guidance

- Keep raw query and count paths read-only by default.
- Move database-specific transaction, timeout, and pagination behavior into adapter implementations.
- Add tests for multi-statement bypass, transaction-control statements, `SELECT INTO`, DML inside CTEs, and parameterized queries for both PostgreSQL and SQL Server where applicable.
- Update query-oriented tool descriptions to encourage small exploratory limits.
- Mention `count_rows` before broad exploratory queries.
- Encourage `WHERE` filters over fetching large result sets.

## Phase 7: Query Plan Diagnostics

- Add generic explain/plan diagnostics through adapter implementations.
- Default to estimated/non-executing plans.
- Treat actual execution plans as explicit opt-in, timeout-bound, capability-gated, and accurately annotated; prefer a separate actual-explain MCP tool.
- Implement PostgreSQL and SQL Server mappings behind the adapter contract; validate SQL Server-specific behavior before implementation.

## Phase 8: Deterministic Health Reports

- Add a generic adapter-backed health report with severity, observations, evidence, runtime limitations, and unavailable reasons.
- Implement PostgreSQL checks where privileges/extensions allow.
- Map SQL Server checks only after adapter-local design validation against official behavior.
- Keep output advisory-only; do not mutate database state.

## Phase 9: Workload / Top-Query / Index Advisory

- Add capability-gated workload/top-query analysis through adapter contracts.
- Use PostgreSQL `pg_stat_statements` when available.
- Validate SQL Server Query Store/DMV mappings before implementation.
- Add advisory-only index recommendations with evidence and confidence.
- Treat hypothetical index/plan-cost comparisons as optional adapter capabilities, not MVP requirements.

## Phase 10: Prompt / Playbook Guidance

- Add reusable prompt/playbook guidance for database exploration, ontology maintenance, and query diagnosis.
- Prefer schema-first, count-before-broad-query, filters, small limits, compact output modes, and metadata updates before question resolution.

## Future Search Tools

- If semantic/vector search is added, start with progressive output modes:
  - `ids_only`
  - `summary`
  - `full`
- Make lightweight modes the default for exploratory search.
- Track full semantic/RAG work in `plans/CONSIDER-semantic-rag-search.md` until explicitly approved.

## Not Planned For Now

- Conversation compaction inside DataClaw, unless the product later owns stored/replayed LLM conversation history.
- Ekaya-style LLM/DAG ontology extraction, Ontology Forge UI, autonomous DBA agent, LLM-driven index tuning, and full RAG/semantic search remain CONSIDER-only until explicitly approved.
