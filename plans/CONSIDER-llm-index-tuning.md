# CONSIDER: LLM-Driven Iterative Index Tuning

CrystalDBA-style loop: LLM proposes index configurations, HypoPG predicts impact, results feed back to the LLM, repeat until no further improvement. Deferred: needs LLM credentials, workload capture, plan-cost simulation, privacy controls, iterative orchestration, validation against over-indexing, and cross-engine parity (HypoPG is PostgreSQL-only).

If revisited: ship deterministic workload/top-query analysis and advisory index recommendations first (TODO Phase 9). LLM-driven tuning behind explicit opt-in and a separate security review.

Reference: https://github.com/crystaldba/postgres-mcp
