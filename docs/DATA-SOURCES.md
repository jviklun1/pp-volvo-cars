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
availability but must not be interpreted as a full corpus export.

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
