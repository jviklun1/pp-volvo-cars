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

The support/knowledge GraphQL adapter is intentionally not implemented. The endpoint and fields in
the historical session report are unverified hypotheses, not a contract. Work remains gated on:

1. approval for the authentication and credential-distribution mechanism; and
2. permission to introspect the live schema or access an authoritative captured schema.

After approval, capture the real schema privately, compare it with prior assumptions, implement a
rate-limited client, and map a sanitized real response fixture into the existing domain types. Do
not hand-write response structures from the historical report and do not run printing-press or
other discovery generators against the private endpoint.

## Multilingual retrieval

The `Matcher` interface exists, but the default implementation adds no cross-language matches.
Selecting local embeddings, an internal service, or deterministic translation remains an approval
decision. Citations must always retain the original source-language excerpt regardless of the
retrieval mechanism.

## Freshness

Stable support/manual content uses a seven-day threshold. Volatile market pages, offers, pricing,
and campaigns use a 24-hour threshold. Results retain capture time, source identity, market, and
language so callers can judge evidence freshness.
