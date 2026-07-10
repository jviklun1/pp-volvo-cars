# Volvo Cars Knowledge CLI architecture

The CLI is a private evidence-retrieval tool for Volvo employees and approved internal agents. It
returns deterministic, market-scoped evidence; it does not generate answers.

## Dependency direction

Production packages follow one direction:

```text
domain
  ↑       ↑        ↑
store   adapter  search
  ↑       ↑        ↑
  └────── cli ─────┘
```

- `internal/domain` owns normalized items, citations, applicability, snapshots, and search
  candidate values. It has no internal dependencies.
- `internal/store` owns SQLite persistence, FTS5, content diffs, and freshness predicates. It
  imports only `domain` internally.
- `internal/adapter` defines the source contract and cache scope. `marketpages` provides
  deterministic SE/GB fixture parsing only; it does not fetch pages. `supportgraphql` queries the
  open Volvo Support Content Service with bounded retries and rate limiting, then maps live
  documents into the normalized domain.
- `internal/search` quotes lexical queries, calls a minimal store interface, and merges optional
  multilingual candidate scores. The default matcher is intentionally a no-op until an approved
  multilingual mechanism is selected.
- `internal/cli` is the composition layer. It owns data-source selection and the
  stale → sync → local-read sequence, preventing a `store → adapter` cycle.

## Identity and applicability

A knowledge item ID is SHA-256 over an unambiguously delimited sequence of source type, stable
source ID, market, and language. Market-page source IDs use the canonical URL path without query
parameters, fragments, or a trailing slash. Content hashes are SHA-256 over the normalized body.

Applicability is one-to-many. Zero rows means an item applies to all vehicles in its market; a
NULL field in an applicability row is a wildcard for that dimension. Search applies market and
optional language/model/year/powertrain filters before returning candidate IDs.

## SQLite and FTS5

`internal/store/schema.sql` is the schema source of truth. External-content FTS5 uses INSERT,
UPDATE, and DELETE triggers tied to `knowledge_items`; FTS rowids join back to the base table's
implicit rowid. Item writes are transactional: existing citation/applicability children are
deleted, the parent is upserted, and replacement children are inserted. A content diff is written
only when the stored and incoming content hashes differ.

The local store deliberately uses one database connection so connection-scoped SQLite settings,
including foreign-key enforcement, apply consistently.

## Command behavior

The binary is `volvo`. Market is mandatory for retrieval and `uk` normalizes to canonical `gb`.
`--data-source` accepts `local`, `auto`, or `live`. Auto refresh is orchestrated by the CLI using
7-day stable and 24-hour volatile thresholds. Market-scoped adapters refresh a whole slice when
stale; the support API is query-scoped, so auto mode captures the current query before local FTS.
`get` rejects live mode because its synthetic item ID exists only after capture. JSON field names
and process exit codes are stable agent contracts.

`volvo sync --market <market>` also refreshes the local market/model catalog. The support adapter
returns all entries from `markets { id caption }` plus the selected market's entries from
`market(id: $market) { carsByDisplayName { displayName modelSlug } }`; the CLI persists those
normalized values before updating the source snapshot. `volvo markets list` and `volvo models
list` therefore reflect the last successful sync. Query-scoped auto-search remains lean and does
not populate the catalog.

`volvo coverage assess` evaluates one `--topic` against one or more repeatable `--question` flags
using only query-scoped (GraphQL) adapters, retrieving each question independently and never
reading local FTS first. Querying live removes the local-cache ambiguity that a stale sync would
introduce, but `no_results` means only that the supplied query returned no usable hits: it is a
gap candidate, not proof of corpus or content absence. This API is search-first and cannot
enumerate a full market, so a Foundry agent must not convert `no_results` into a confirmed
knowledge gap. The command inherits the persistent `--data-source` flag but always uses live
query-scoped sources, so an explicitly supplied `--data-source` is rejected with exit `4` before
any adapter call. Each question reports `evidence_found`, `no_results`, or `source_error`; a source
error on one question does not abort the others. The full report is always written before any error
is returned, including an internal evidence-persistence failure; when a persistence failure and a
source error coexist, the process returns the internal failure (exit `1`) while retaining every
per-question source diagnostic in the report. Questions that find evidence are persisted (item
upsert plus source snapshot); the aggregate `status` is `incomplete` and the process exits `5` when
any question source-errors.

The module path is temporarily `github.com/jviklun1/pp-volvo-cars` until the owning internal Git organization is
selected.
