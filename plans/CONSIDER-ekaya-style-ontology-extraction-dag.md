# CONSIDER: Ekaya-Style Ontology Extraction DAG

Product-owned LLM/DAG that extracts ontology, generates questions, enriches glossary, and exposes an Ontology Forge setup UI — modeled on `../ekaya-engine` (`pkg/services/dag/`, ontology SQL, `OntologyForgePage`).

Deferred: pulls LLM credentials, prompt/version management, cost controls, and lifecycle UI ownership into DataClaw; conflicts with the "MCP client/LLM is the intelligence layer" stance; and needs a deeper privacy/security review for sending schema and sample data to LLM providers.

If revisited: separate product decision. Manual MCP-authored metadata, deterministic questions, and pending-changes review (TODO Phases 3–4) come first.
