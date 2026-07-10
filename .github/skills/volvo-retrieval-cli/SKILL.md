---
name: volvo-retrieval-cli
description: >
  Build and review the Volvo Cars evidence-retrieval CLI safely, with deterministic
  local storage, adapter boundaries, GraphQL contract validation, and agent-facing
  command behavior. Use when modifying the CLI, its SQLite store, source adapters,
  or retrieval commands.
---

# Volvo retrieval CLI engineering

## Keep package direction one-way

- Put shared transport values in the leaf domain package; do not return a sibling
  infrastructure package's type from another sibling.
- Keep freshness checks pure in storage. The command layer owns stale → sync → read
  orchestration so storage never imports an adapter.
- Distinguish market-wide and query-scoped cache capabilities. A source that cannot
  enumerate a full market must never be marked market-fresh after a narrow search.
- When a sync carries parent records and dependent records, persist parents first and
  keep sync-only catalog refresh separate from query capture.

## Preserve deterministic retrieval

- Audit greenfield plans for internal consistency: validate that the model, schema,
  and dependency direction can support the promised command surface before code exists.
- Use unambiguous component encoding for stable hashed identifiers, and lock a known
  result in a test. For web resources, derive identity from a normalized canonical
  URL path rather than volatile query, fragment, or content values.
- Sort map-derived identifiers before observable work or output.
- For external-content SQLite FTS, define insert, update, and delete synchronization
  triggers and test both indexing and deletion.
- Treat SQLite connection settings as connection-scoped: apply them per connection or
  deliberately constrain the pool.

## Validate external and CLI contracts

- Verify GraphQL argument types at each nested field; similar enum values do not prove
  type compatibility. Exercise a live query with typed variables in addition to fixture mapping.
- Normalize framework-generated flag and positional-argument failures to the CLI's
  documented invalid-argument error. Test unknown-command and bare-root behavior when
  customizing Cobra's root validation.
