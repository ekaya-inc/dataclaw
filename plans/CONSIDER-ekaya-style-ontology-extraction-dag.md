# CONSIDER: Ekaya-Style Ontology Extraction DAG in DataClaw

## Status

Not planned for near-term implementation. Capture only.

## Source Context

Ekaya Engine (`../ekaya-engine`) includes an ontology extraction lifecycle designed around a product-owned LLM, DAG nodes, ontology completion state, Admin setup UI, questions, glossary enrichment, and import/export. Relevant reviewed areas include:

- Ekaya Engine ontology/DAG SQL definitions for ontology, DAG, and incremental ontology state.
- `pkg/services/dag/` and LLM-backed ontology services for extraction nodes.
- Ekaya Engine LLM/config SQL definitions for LLM conversation logging.
- UI routes such as `OntologyForgePage` / ontology questions in `../ekaya-engine/ui/src`.

## Why This Is Not DataClaw's Current Shape

DataClaw should let an Admin connect an MCP client right away and configure schema/ontology metadata through MCP tools. The user's MCP client/LLM owns the intelligence loop. DataClaw should store and serve the context, not run a separate product-owned extraction workflow.

## Potential Value If Revisited

- One-click ontology bootstrapping for users without a capable MCP client.
- Automatic question generation and glossary suggestions.
- More opinionated business semantics and metrics setup.

## Risks / Open Questions

- Adds LLM credentials, privacy concerns, prompt/version management, and cost controls.
- Requires UI/lifecycle ownership that DataClaw is intentionally avoiding now.
- Harder to preserve database-neutral behavior and adapter-boundary clarity.
- Requires a much deeper security review around sending schema/sample data to LLM providers.

## If Revisited

Treat as a separate product decision and planning effort. Keep it outside the current SQL/MCP plan unless explicitly approved. The current PRD only adapts deterministic schema registry, MCP-authored metadata, deterministic questions/pending changes, and later redacted bundle portability.
