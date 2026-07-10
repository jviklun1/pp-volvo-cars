# Data sources and authorization boundary

This repository is private. Only official Volvo-controlled sources may supply evidence.

## Public market pages

The implemented SE and GB code parses committed, minimal test fixtures into normalized
`KnowledgeItem` and `Citation` values. It preserves the original Swedish or English excerpt and
derives identity from the canonical page path.

No network acquisition is implemented. Adding a feed, controlled browser capture, HAR workflow,
or targeted fetcher requires an explicitly approved acquisition path. Any future fetch must be
targeted, rate-limited, reviewable, and covered by saved response fixtures; broad crawling is out
of scope. HAR files and schema captures are gitignored because they may contain sensitive data.

## Support and knowledge GraphQL

The Volvo Support Content Service endpoint is open and requires no authentication. On 2026-07-09,
targeted live introspection and real market-level and car-scoped searches verified the adapter's
field and enum contracts. A sanitized response captured from that live check is committed as a
mapping fixture; the client has bounded retry, rate-limit, response-size, and timeout controls.

This API is search-first: it does not expose a complete market-wide document enumeration. Its
cache scope is therefore query-specific. `--data-source auto` searches the live support service,
persists those normalized results, and then uses local FTS. The `sync` operation verifies market
availability and refreshes the catalog, but must not be interpreted as a full corpus export.

Catalog refresh uses the verified GraphQL fields `markets { id caption }` and
`market(id: $market) { carsByDisplayName { displayName modelSlug } }`. The latter is the
model-listing field; `carByModelSlug` remains the separate slug-input lookup used for car-scoped
search. This contract is grounded in the authoritative
`volvo-cars-support/references/graphql-schema.md` `Market` table and example query 4. A sync stores
all returned markets and the selected market's de-duplicated model slugs. `volvo markets list` and
`volvo models list` expose the last successful sync; query-scoped auto-search does not refresh
either catalog.

The public API currently exposes the United Kingdom as `uk` (including duplicate market rows).
The CLI retains canonical `gb` and translates to `uk` only at the upstream boundary. Market and
car searches use distinct GraphQL enum types—`MarketSearchType` and `CarSearchType`—despite sharing
the same enum values.

Do not run printing-press or other generation pipelines against the endpoint; the hand-written,
minimal query contract is intentional and covered by live and fixture verification.

## Multilingual retrieval

The `Matcher` interface exists, but the default implementation adds no cross-language matches.
Selecting local embeddings, an internal service, or deterministic translation remains an approval
decision. Citations must always retain the original source-language excerpt regardless of the
retrieval mechanism.

## Freshness

Stable support/manual content uses a seven-day threshold. Volatile market pages, offers, pricing,
and campaigns use a 24-hour threshold. Results retain capture time, source identity, market, and
language so callers can judge evidence freshness.

## Known limitations (v1)

- `internal/adapter/marketpages`, the `market_page`/`offer`/`pricing`/`campaign` source types, and
  volatile freshness handling are implemented and tested but are not wired into the binary. The
  composition root currently registers only support GraphQL; a market-page fetch path remains
  gated on Q3 acquisition approval.
- Runtime paths currently emit exit codes 0 (success), 1 (internal), 3 (not found), 4 (invalid
  arguments), 5 (insufficient evidence), and 7 (rate limited), plus 2 if an upstream authentication
  boundary applies. Code 5 is emitted by `coverage assess` when its aggregate result is
  `incomplete` — that is, when at least one question returned a source error.
- `coverage assess` is search-first: a `no_results` outcome is a **gap candidate, not proof of
  content absence**. Because the support API cannot enumerate a full market corpus, zero hits mean
  only that the supplied query returned nothing usable and must never be reported to a caller as
  confirmed missing knowledge. `coverage assess` always uses live query-scoped sources, so it does
  not accept `--data-source`; an explicit value is rejected as invalid arguments (exit 4).
