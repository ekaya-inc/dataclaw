# CONSIDER: LLM-Driven Iterative Index Tuning

## Status

Not planned for near-term implementation. Capture only.

## Source Context

CrystalDBA documents experimental LLM-based index tuning that proposes index configurations, predicts impact with HypoPG, feeds results back to the LLM, and repeats until no further improvements: https://github.com/crystaldba/postgres-mcp

## Why This Is Large Scope

This requires LLM credentials, query/workload capture, plan-cost simulation, privacy controls, iterative orchestration, and strong validation. It also starts with PostgreSQL-specific HypoPG; SQL Server parity would need separate adapter-local research and official-behavior validation.

## Potential Value

- Better index suggestions for large search spaces.
- Iterative evaluation with plan-cost evidence.
- DBA-assist workflows for difficult workloads.

## Risks / Open Questions

- How are sensitive schema/query details protected before sending to an LLM?
- How are candidate indexes validated across engines?
- What prevents over-indexing or workload regressions?
- Should DataClaw require human approval for every recommendation?

## If Revisited

Implement deterministic workload/top-query and advisory index recommendations first. Keep LLM-driven tuning behind an explicit opt-in design and separate security review.
