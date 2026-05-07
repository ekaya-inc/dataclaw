# BRAINSTORM: MCP Client UX Improvements

Captured from a non-destructive smoke test on 2026-05-07 driving every modeler tool from an MCP client. Unvetted suggestions — vet before promoting to a DESIGN/PLAN.

- **Validate-without-persist for approved queries.** `create_query` / `update_query` only fail at submit. A `validate_query` (or dry-run flag) that runs placeholder/parameter parity checks, dialect anti-pattern checks, and read-only/DML-shape checks without writing the catalog would shorten the iteration loop.
- **Deduplicate the approved-query template rules in the tool schema.** The ~40-line "Approved query template rules" block currently appears verbatim in `create_query`, `update_query`, and the `sql_query` field description. One canonical statement plus short pointers would shrink the schema substantially without losing the guidance.
- **Validate `output_columns` on create/update.** The existing connectivity query has a stray `{"name":"","type":""}` entry; the catalog accepted it. Reject empty entries, or strip them before persisting.
- **Promote schema-explorer gaps onto the roadmap.** TODO Phase 2 already opts row counts behind explicit cost. Keys / indexes / foreign keys are acknowledged in the live `limitations` array but not owned by any phase — clients reach for raw `information_schema` queries to fill the gap.
- **Worked CAST examples in parameter docs.** The "add `CAST({{x}} AS …)` only when the operator needs a different type" rule is correct but easy to forget for timestamp math and numeric coercion. One or two concrete examples in the `sql_query` description would prevent the common mistakes.
