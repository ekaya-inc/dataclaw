# Approved Query Tool Parameter Normalization

## Problem

The approved-query tool surface uses inconsistent field names for the same concepts:

- `list_queries()` returns query objects with `id` and `sql_query`.
- `update_query()` expects `query_id` and `sql`.
- `delete_query()` expects `query_id`.
- `create_query()` also uses `sql`.

This makes the API harder to use because callers cannot take the output of `list_queries()` and pass it directly into `update_query()` or `delete_query()` without renaming fields first.

## Current Mismatch

### `list_queries()`

```json
{
  "queries": [
    {
      "id": "b21d822b-e2ac-4dbe-873d-7da50e5f097f",
      "name": "Connectivity Check",
      "description": "Use this for testing connectivity to the datasource",
      "sql_query": "SELECT true AS connected",
      "is_enabled": true
    }
  ]
}
```

### `update_query(...)`

```json
{
  "query_id": "b21d822b-e2ac-4dbe-873d-7da50e5f097f",
  "name": "Connectivity Check",
  "sql": "SELECT true AS connected"
}
```

### `delete_query(...)`

```json
{
  "query_id": "b21d822b-e2ac-4dbe-873d-7da50e5f097f"
}
```

## Why This Is a Problem

- It creates avoidable client-side translation logic.
- It increases the chance of tool-call errors.
- It makes the interface feel inconsistent and unfinished.
- It complicates future additions such as `execute_query()` because naming is already divergent.

## Proposed Normalization

Use one canonical name for each concept across the full approved-query surface:

- Query identifier: `query_id`
- SQL text: `sql`

### Canonical shapes

#### `list_queries()`

Return:

```json
{
  "queries": [
    {
      "query_id": "b21d822b-e2ac-4dbe-873d-7da50e5f097f",
      "name": "Connectivity Check",
      "description": "Use this for testing connectivity to the datasource",
      "sql": "SELECT true AS connected",
      "is_enabled": true
    }
  ]
}
```

#### `create_query(...)`

Accept:

```json
{
  "name": "Connectivity Check",
  "description": "Use this for testing connectivity to the datasource",
  "sql": "SELECT true AS connected",
  "is_enabled": true
}
```

#### `update_query(...)`

Accept:

```json
{
  "query_id": "b21d822b-e2ac-4dbe-873d-7da50e5f097f",
  "name": "Connectivity Check",
  "sql": "SELECT true AS connected",
  "is_enabled": true
}
```

#### `delete_query(...)`

Accept:

```json
{
  "query_id": "b21d822b-e2ac-4dbe-873d-7da50e5f097f"
}
```

## Compatibility Plan

To avoid breaking existing clients, normalize at the tool boundary first:

1. Continue accepting legacy input aliases where relevant:
   - `id` as an alias for `query_id`
   - `sql_query` as an alias for `sql`
2. Return canonical fields in responses:
   - prefer `query_id` over `id`
   - prefer `sql` over `sql_query`
3. Optionally include both legacy and canonical response fields for one release if compatibility is a concern.
4. Update tool docs, generated schemas, and examples to use only canonical names.
5. Remove legacy aliases only after clients have migrated.

## Suggested Implementation Notes

- Normalize request payloads immediately after decoding.
- Normalize response objects before returning them from the tool layer.
- Keep storage names internal; only the external contract needs to be consistent.
- Add regression tests that verify:
  - `list_queries()` returns canonical field names.
  - `update_query()` accepts `query_id`.
  - `delete_query()` accepts `query_id`.
  - legacy aliases still work during the migration window.

## Recommendation

Adopt `query_id` and `sql` as the external contract everywhere. They already match the mutation tools and are clearer than mixing `id` with `sql_query`.
