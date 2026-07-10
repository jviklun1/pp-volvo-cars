# Volvo CLI Package - Session Report for OpenAI Codex

Date: 2026-05-19

## Purpose

This document summarizes a discussion about whether `mvanhorn/cli-printing-press` can be used to create or improve a CLI package for querying Volvo Cars public/support/market knowledge. It is intended as a handoff brief for a future OpenAI Codex coding session.

## Repositories and Sources Discussed

Primary generator candidate:

- `https://github.com/mvanhorn/cli-printing-press`

Volvo public market pages initially mentioned:

- `https://www.volvocars.com/us/`
- `https://www.volvocars.com/se/`

Existing local Hermes skill/API reference:

- `~/.hermes/skills/openclaw-imports/volvo-cars-support/`

Known Volvo support GraphQL endpoint used by the existing skill:

- `https://support-content-service.weu-prod.ecpaz.volvocars.biz/api/graphql`

Important sensitivity note:

- The Volvo support API is a working knowledge API, but it is not officially published.
- John knows about it through work.
- John intends to ask permission before open-sourcing any CLI that exposes or wraps it.
- Until permission is granted, any generated CLI, schema snapshot, HAR capture, endpoint details, and discovery artifacts should be treated as private/internal.

## Existing Volvo Knowledge API Capability

The existing local skill already confirms useful GraphQL access to Volvo support/manual/knowledge content. It can query markets, models, support articles, user manual documents, knowledge pages, PDFs/manuals, and documents by ID.

Example GraphQL endpoint usage from the local skill:

```bash
cd ~/.hermes/skills/openclaw-imports/volvo-cars-support
./scripts/graphql-query.sh '{ markets { id caption } }'
```

The script posts to:

```bash
https://support-content-service.weu-prod.ecpaz.volvocars.biz/api/graphql
```

Common market IDs include:

- `us` - United States
- `se` - Sweden
- `gb` - United Kingdom
- `de` - Germany
- `no` - Norway
- `fr` - France
- `nl` - Netherlands

Search types supported by the API include:

- `LATEST_INFO`
- `SUPPORT_ARTICLE`
- `USER_MANUAL`
- `SOFTWARE_RELEASE_NOTES`
- `QUALITY_BULLETIN`
- `KNOWLEDGE`

Representative GraphQL pattern:

```graphql
{
  market(id: "us") {
    carByModelSlug(modelSlug: "xc60") {
      displayName
      modelYear
      search(
        q: "tire pressure",
        include: [USER_MANUAL, SUPPORT_ARTICLE],
        language: "en",
        maxResults: 5
      ) {
        pageInfo { resultCount }
        results {
          score
          ... on DocumentSearchResult {
            matchingParagraph
            document {
              documentId
              documentType
              stringContent {
                title
                description
              }
            }
          }
        }
      }
    }
  }
}
```

Representative full-document pattern:

```graphql
{
  market(id: "se") {
    document(documentId: "DOCUMENT_ID_HERE", language: ["en"]) {
      stringContent { title description }
      jsonContent { body }
      children { documentId stringContent { title } }
    }
  }
}
```

## Initial Assessment of CLI Printing Press

`cli-printing-press` appears suitable for this project because it is designed to generate production-ready Go CLIs and MCP servers from APIs, including REST, GraphQL, browser-sniffed traffic, HAR files, or documentation pages.

Important capabilities from `cli-printing-press`:

- Generates Cobra-style Go CLI projects.
- Generates matching MCP servers.
- Supports GraphQL specs.
- Supports website/browser-sniffing mode when no official API spec exists.
- Supports HAR import from browser DevTools.
- Produces local SQLite sync/search/export patterns.
- Encourages agent-native CLI UX: JSON output, compact output, sync, search, SQL, and compound commands.
- Is designed to go beyond endpoint wrappers by adding domain-specific commands and local analytics.

Conclusion from the session:

- Yes, `cli-printing-press` can be used to build or improve a Volvo CLI.
- For the support API, the best input is the GraphQL schema/API, not just the `.com` pages.
- For `volvocars.com/us` and `volvocars.com/se`, the best input is likely browser/HAR discovery plus targeted page extraction.

## Refined Product Direction

The original question was whether the CLI could query knowledge from Volvo `.com` pages. After discussion, the better product framing is broader than a support-search wrapper.

Recommended product framing:

- `volvo-content`
- `volvo-market-intel`
- `volvo-public-content`
- `volvo-support` only if scoped narrowly to support/manual content

Recommended conceptual scope:

> A private Volvo Cars public-content and support CLI that unifies structured support/manual knowledge with market-specific `.com` content such as pricing, offers, campaigns, model pages, configurator/pricing surfaces, and local communication.

The support GraphQL API should be treated as the structured backbone. The `.com` pages should be treated as market/local content surfaces.

## Why `.com` Pages Matter

John clarified that the reason for asking about `volvocars.com/us` and `volvocars.com/se` was that there is other local market content outside the support knowledge API, such as:

- Pricing
- Local offers
- Campaigns
- Local communication
- Market-specific model pages
- Configurator/order flows
- Leasing/subscription copy
- Incentive/disclaimer language
- Dealer/local offer pages where relevant and permitted

This means the CLI should not only wrap the support GraphQL API. It should also be able to inspect, extract, normalize, cache, and diff public market pages.

## Recommended Architecture

One CLI with source-specific namespaces is preferable to separate CLIs.

Possible top-level command shape:

```bash
volvo markets list
volvo models list --market us

volvo support search "tire pressure" --market us --model xc60
volvo support get <document-id> --market us --lang en
volvo support sync --markets us,se --models xc60,ex90,ex30 --langs en,sv

volvo manuals list --market us --model xc60 --year 2024
volvo knowledge tree --market se --lang sv

volvo pages fetch /us/cars/ex90-electric/
volvo pages inspect --url https://www.volvocars.com/us/cars/ex90-electric/
volvo pages extract --market us --type model-page
volvo pages diff --markets us,se --path cars/ex90-electric

volvo offers list --market us
volvo offers watch --markets us,se --model ex30
volvo pricing get --market us --model ex90
volvo campaigns list --market se

volvo answer "How do I reset tire pressure monitoring?" --market us --model xc60 --citations
volvo buying-check --model s90 --year 2017 --engine t6 --market us
volvo compare markets us se --topic "EX90 charging"
volvo brief market --market se --since 7d
```

## Recommended MCP Tools

The MCP surface should map to real user/agent workflows, not only raw GraphQL operations.

Potential tools:

- `volvo_search_support`
- `volvo_get_document`
- `volvo_get_manual_section`
- `volvo_list_models`
- `volvo_find_current_offers`
- `volvo_extract_page_facts`
- `volvo_compare_market_pages`
- `volvo_answer_owner_question`
- `volvo_check_used_buying_context`
- `volvo_detect_content_changes`
- `volvo_generate_market_brief`

## Local SQLite Store Recommendation

A local store is strongly recommended. This is one of the most valuable ways `cli-printing-press` can enhance the existing work.

Suggested tables/entities:

- `markets`
- `models`
- `model_years`
- `documents`
- `document_versions`
- `manuals`
- `knowledge_nodes`
- `software_release_notes`
- `support_search_index`
- `market_pages`
- `page_snapshots`
- `offers`
- `prices`
- `campaigns`
- `dealer_pages` if in scope/permitted
- `source_fetches`
- `citations`
- `content_diffs`

Each fetched or derived item should store provenance:

- Source URL or API endpoint/source type
- Market
- Language
- Model/model year if known
- Source type: support API, manual, knowledge, `.com` model page, offer page, dealer page, app store, third-party, etc.
- Captured timestamp
- Nearby disclaimer/legal copy where relevant
- Raw JSON/body when allowed
- Normalized text/body for search
- Hash for diffing

## Recommended CLI Design Principles

1. Prefer domain commands over raw endpoint wrappers.
2. Market, language, model, and model year should be first-class filters.
3. Every answer should include citations/provenance.
4. Support structured output: `--json`, `--compact`, and citation-rich markdown.
5. Support local/offline search after sync.
6. Rate limit politely and avoid aggressive crawling.
7. Keep generated discovery artifacts private until permission is granted.
8. Keep raw HAR captures and schema snapshots out of public repos unless explicitly approved.
9. Design for agent usage: deterministic commands, stable schemas, compact output, low ambiguity.
10. Treat `.com` pages as changing market content that must be snapshotted and diffed.

## Suggested Printing Press Build Path

Recommended private spike:

1. Create a private repo/worktree for the CLI.
2. Snapshot or export the GraphQL schema for the support API.
3. Feed the GraphQL schema/API into `cli-printing-press`.
4. Capture HAR files for selected public `.com` flows:
   - `https://www.volvocars.com/us/`
   - `https://www.volvocars.com/se/`
   - One US model page, e.g. EX90/EX30/XC60.
   - One SE model page for the same model.
   - One offers/pricing page if available.
   - One support/manual flow if useful for comparison.
5. Feed HAR captures into `cli-printing-press` discovery mode.
6. Generate CLI + MCP server.
7. Keep only the useful generated scaffolding.
8. Add custom compound commands manually.
9. Add SQLite sync/search/diff.
10. Dogfood against real Volvo owner/purchase questions.

Possible Printing Press modes:

- GraphQL schema/spec mode for the support API.
- HAR mode for `.com` and dealer/local offer pages.
- Website/browser-sniff mode for exploratory discovery, but HAR is preferred for control and auditability.

HAR mode is preferred because:

- It is controlled.
- It captures exactly the pages/actions relevant to the CLI.
- It avoids broad crawling.
- It is easier to review before generation.
- It preserves discovery provenance.
- It helps avoid accidentally wrapping sensitive/internal-looking endpoints.

## Social Research: Last 30 Days Summary

The `last30days` skill was used to investigate what kinds of Volvo-related questions people ask on social/public platforms. The run had partial source coverage:

- Reddit returned useful material in the initial run.
- Web/grounding returned useful public pages.
- YouTube returned limited signal.
- X returned HTTP 403.
- Later Reddit follow-up attempts hit rate limiting/402 responses.
- TikTok and Instagram did not return useful Volvo-specific results in this run.

Despite imperfect coverage, the directional signal was clear.

### Key Social/Consumer Question Patterns

1. Warranty, service, and escalation questions

Example surfaced:

- `r/Volvo`: “Volvo’s 8-Month Warranty Nightmare: My XC60 T8 PHEV is now just a heavy, gas-guzzling SUV, and Volvo Sweden refuses to help. Has anyone else experienced this?”
- URL: `https://www.reddit.com/r/Volvo/comments/1teo0nz/volvos_8month_warranty_nightmare_my_xc60_t8_phev/`

Implication:

- The CLI should answer questions that depend on model, year, drivetrain, market, warranty terms, service process, parts availability, and escalation path.

Potential command:

```bash
volvo answer "XC60 T8 high voltage battery warranty delay Sweden" \
  --market se \
  --model xc60 \
  --powertrain t8 \
  --citations
```

2. Used-car due diligence and ownership-confidence questions

Example surfaced:

- `r/Volvo`: “Thoughts on a 2017 S90 T6 inscription”
- URL: `https://www.reddit.com/r/Volvo/comments/1tb61jh/thoughts_on_a_2017_s90_t6_inscription/`

Implication:

- The CLI should help with ownership due diligence, not only support search.
- It should combine manuals, maintenance schedules, warranty context, known support topics, service intervals, and market/pricing context where available.

Potential command:

```bash
volvo buying-check --model s90 --year 2017 --engine t6 --market us
```

3. EV battery, charging, software, and resale-value questions

Example surfaced:

- Recharged article: “How to Sell a Volvo EX30 for the Best Price”
- URL: `https://recharged.com/articles/how-to-sell-volvo-ex30`

Themes in this result:

- EX30 depreciation
- Battery state of health
- OTA software state
- Warning lights
- Incentives and lease effects
- Documentation needed for resale
- Charging habits and buyer confidence

Implication:

- EV topics should be first-class in the CLI.
- Commands should group charging, range, battery health, preconditioning, OTA/software, app setup, incentives, and resale context.

Potential commands:

```bash
volvo ev charging --model ex30 --market us
volvo ev battery-health-docs --model ex30
volvo ev resale-check --model ex30 --market us
```

4. Pricing, leasing, offers, and local-market caveats

Example surfaced:

- Volvo Cars Ramsey EX30 Cross Country lease page
- URL: `https://www.volvocarsramsey.com/volvo-models/electric/ex30-cross-country-mahwah-nj.htm`

Themes in this result:

- Local lease language
- Federal lease pass-through language
- NJ incentives
- Dealer-specific terms
- Trim/pricing claims
- Reservation/preorder language
- Local service-area copy

Implication:

- Pricing/offers are not fully represented in the support API.
- The CLI needs `.com` and possibly dealer/local offer page ingestion, with disclaimers and timestamps.
- Offer extraction must be market-aware and time-aware.

Potential commands:

```bash
volvo offers list --market us --model ex30
volvo pricing get --market us --model ex30
volvo offers diff --markets us,se --model ex30
volvo watch offers --markets us,se --model ex30
```

5. Market-aware answerability is the product gap

Users do not think in source boundaries. They ask blended questions like:

- “What applies to my exact car?”
- “What applies in Sweden vs the US?”
- “Does this warranty/offer/charging guidance apply to my model year?”
- “What changed on the local website?”
- “Is this deal real after incentives/disclaimers?”
- “Which manual/support page answers this issue?”

Implication:

- The CLI should be designed around answer workflows that can retrieve from multiple source types and return source-specific citations.

## Recommended Intent Categories

Add normalized intent categories to the CLI/store. These can help classify queries, route searches, and design MCP tool schemas.

Suggested categories:

- `troubleshooting`
- `warranty_claim`
- `service_maintenance`
- `manual_lookup`
- `software_app`
- `ev_charging_range`
- `battery_health`
- `buying_advice`
- `used_car_due_diligence`
- `pricing_offer`
- `lease_finance`
- `market_comparison`
- `content_change_detection`

## Example High-Value Workflows

### Support answer with citations

```bash
volvo answer "How do I reset tire pressure monitoring?" \
  --market us \
  --model xc60 \
  --year 2024 \
  --citations
```

Expected behavior:

- Search support/manual docs.
- Fetch top documents.
- Return concise answer.
- Include citations with document IDs and source URLs if available.

### Compare market content

```bash
volvo compare markets us se --topic "EX90 charging"
```

Expected behavior:

- Search support and market pages in both markets.
- Normalize language/content.
- Identify differences, missing docs, or divergent claims.
- Include citations and captured timestamps.

### Local offer monitoring

```bash
volvo offers watch --markets us,se --model ex30 --since 7d
```

Expected behavior:

- Check synced page snapshots.
- Detect changes in price, lease, incentive, disclaimers, or availability language.
- Return diff and source URLs.

### Used-car buying context

```bash
volvo buying-check --model s90 --year 2017 --engine t6 --market us
```

Expected behavior:

- Retrieve manuals/maintenance/service docs.
- Surface common maintenance schedule items.
- Retrieve warranty/extended warranty public guidance.
- Optionally include market pricing pages or third-party references if in scope.

### EV resale/ownership check

```bash
volvo ev resale-check --model ex30 --market us
```

Expected behavior:

- Pull EV battery/charging/software support content.
- Include documentation checklist: software updated, warning lights, battery health evidence, charging cable, service records.
- Cite source docs/pages.

## Recommended Private Spike Acceptance Criteria

A useful first private spike should demonstrate:

1. `volvo support search` works for `us` and `se`.
2. `volvo support get` fetches full document content by ID.
3. `volvo models list --market us/se` works.
4. `volvo pages inspect` can extract useful structured facts from at least one US and one SE model page.
5. `volvo offers list` or `volvo pricing get` can extract at least one market/local offer page where permitted.
6. `volvo sync` stores support docs and page snapshots in SQLite.
7. `volvo search-local` works against synced content.
8. `volvo compare markets us se --topic <topic>` returns a useful cited diff.
9. MCP server exposes at least 4 useful tools:
   - support search
   - get document/manual section
   - extract page facts
   - compare market content
10. All generated/discovered sensitive artifacts remain private.

## Security, Governance, and Publication Notes

Until permission is granted:

- Do not publish the support API endpoint in a public repo.
- Do not publish schema snapshots.
- Do not publish HAR captures.
- Do not publish generated clients that reveal non-public endpoints.
- Do not publish discovery manifests or traffic-analysis artifacts.
- Keep the repo private.
- Use conservative rate limits.
- Avoid broad crawling.
- Prefer explicit HAR captures and targeted URLs.
- Keep source provenance and captured timestamps.

If open-sourcing later, possible approaches:

1. Open-source only the CLI framework and require users to provide their own GraphQL endpoint/spec.
2. Include only `.com` page extraction for clearly public pages.
3. Keep support API integration as a private plugin/adapter.
4. Strip all discovery artifacts from the public repo.
5. Provide mock fixtures instead of live endpoint references.

## Bottom-Line Recommendation

Use `cli-printing-press` to accelerate this, but do not rely on generated endpoint wrappers alone.

Best direction:

> Build a private `volvo-content` or `volvo-market-intel` CLI that uses the existing Volvo support GraphQL API as the structured backbone, then adds HAR/browser-discovered `.com` market content extraction for pricing, offers, campaigns, and local communication. Use SQLite for sync/search/diff and expose a curated MCP surface for real owner, buyer, and market-comparison workflows.

The social research supports this direction: people ask blended, local, model-specific, situation-specific questions. The CLI should therefore be market-aware, model-aware, source-aware, citation-heavy, and designed around compound answers rather than raw API operations.
