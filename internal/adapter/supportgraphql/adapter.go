package supportgraphql

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	baseadapter "github.com/jviklun1/pp-volvo-cars/internal/adapter"
	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

const marketSearchQuery = `
query Search($market: ID!, $query: String!, $include: [MarketSearchType!]!, $language: String!, $maxResults: Int!) {
  market(id: $market) {
    search(q: $query, include: $include, language: $language, maxResults: $maxResults) {
      results {
        score
        ... on DocumentSearchResult {
          matchingParagraph
          document {
            documentId externalId fullDocumentId language documentType lastUpdate
            stringContent { title description }
            applicableTo { title values }
          }
        }
      }
    }
  }
}`

const carSearchQuery = `
query SearchCar($market: ID!, $modelSlug: String!, $query: String!, $include: [CarSearchType!]!, $language: String!, $maxResults: Int!) {
  market(id: $market) {
    carByModelSlug(modelSlug: $modelSlug) {
      search(q: $query, include: $include, language: $language, maxResults: $maxResults) {
        results {
          score
          ... on DocumentSearchResult {
            matchingParagraph
            document {
              documentId externalId fullDocumentId language documentType lastUpdate
              stringContent { title description }
              applicableTo { title values }
            }
          }
        }
      }
    }
  }
}`

const marketsQuery = `query { markets { id caption } }`

const modelsQuery = `
query Models($market: ID!) {
  market(id: $market) {
    carsByDisplayName { displayName modelSlug }
  }
}`

const documentQuery = `
query Document($market: ID!, $documentID: String!, $languages: [String!]) {
  market(id: $market) {
    document(documentId: $documentID, language: $languages) {
      documentId externalId fullDocumentId language documentType lastUpdate
      stringContent { title description }
      jsonContent { body jsonContentSchemaVersion }
      applicableTo { title values }
    }
  }
}`

var errDocumentNotFound = errors.New("support document was not found")

var searchTypes = []string{
	"LATEST_INFO", "SUPPORT_ARTICLE", "USER_MANUAL", "SOFTWARE_RELEASE_NOTES", "QUALITY_BULLETIN", "KNOWLEDGE",
}

type documentRef struct {
	Market     domain.Market
	DocumentID string
	Language   domain.Language
}

type Adapter struct {
	client *Client
	now    func() time.Time

	mu         sync.RWMutex
	snapshots  map[domain.Market]domain.SourceSnapshot
	references map[string]documentRef
}

func New() *Adapter {
	return newAdapter(NewClient(), time.Now)
}

func newAdapter(client *Client, now func() time.Time) *Adapter {
	return &Adapter{
		client: client, now: now, snapshots: make(map[domain.Market]domain.SourceSnapshot), references: make(map[string]documentRef),
	}
}

func (*Adapter) CacheScope() baseadapter.CacheScope { return baseadapter.CacheScopeQuery }

func (adapter *Adapter) Sync(ctx context.Context, market domain.Market, _ baseadapter.SyncOptions) (baseadapter.SyncResult, error) {
	var data marketsData
	if err := adapter.client.query(ctx, marketsQuery, nil, &data); err != nil {
		return baseadapter.SyncResult{}, err
	}
	upstream := upstreamMarket(market)
	found := false
	markets := make([]domain.MarketRef, 0, len(data.Markets))
	for _, value := range data.Markets {
		markets = append(markets, domain.MarketRef{
			ID:      canonicalMarket(domain.Market(value.ID)),
			Caption: value.Caption,
		})
		if value.ID == upstream {
			found = true
		}
	}
	if !found {
		return baseadapter.SyncResult{}, fmt.Errorf("Volvo support market %q was not found", upstream)
	}
	var modelData modelsData
	if err := adapter.client.query(ctx, modelsQuery, map[string]any{"market": upstream}, &modelData); err != nil {
		return baseadapter.SyncResult{}, err
	}
	if modelData.Market == nil {
		return baseadapter.SyncResult{}, fmt.Errorf("Volvo support market %q was not found", upstream)
	}
	canonical := canonicalMarket(market)
	models := make([]domain.ModelRef, 0, len(modelData.Market.CarsByDisplayName))
	seenSlugs := make(map[domain.ModelSlug]struct{}, len(modelData.Market.CarsByDisplayName))
	for _, value := range modelData.Market.CarsByDisplayName {
		slug := domain.ModelSlug(value.ModelSlug)
		if _, seen := seenSlugs[slug]; seen {
			continue
		}
		seenSlugs[slug] = struct{}{}
		models = append(models, domain.ModelRef{
			Market:      canonical,
			Slug:        slug,
			DisplayName: value.DisplayName,
		})
	}
	snapshot := adapter.recordSnapshot(canonicalMarket(market), "availability")
	return baseadapter.SyncResult{Markets: markets, Models: models, Snapshot: snapshot}, nil
}

func (adapter *Adapter) Search(ctx context.Context, query string, options baseadapter.SearchOptions) ([]domain.KnowledgeItem, error) {
	if query == "" {
		return nil, errors.New("support search query is required")
	}
	language := "en"
	if options.Language != nil && *options.Language != "" {
		language = string(*options.Language)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	variables := map[string]any{
		"market": upstreamMarket(options.Market), "query": query, "include": searchTypes,
		"language": language, "maxResults": limit,
	}
	queryText := marketSearchQuery
	if options.Applicability.ModelSlug != nil {
		queryText = carSearchQuery
		variables["modelSlug"] = string(*options.Applicability.ModelSlug)
	}
	var data searchData
	if err := adapter.client.query(ctx, queryText, variables, &data); err != nil {
		return nil, err
	}
	if data.Market == nil {
		return nil, fmt.Errorf("Volvo support market %q was not found", variables["market"])
	}
	response := data.Market.Search
	if options.Applicability.ModelSlug != nil {
		if data.Market.CarByModelSlug == nil {
			return nil, fmt.Errorf("Volvo support model %q was not found", *options.Applicability.ModelSlug)
		}
		response = data.Market.CarByModelSlug.Search
	}
	capturedAt := adapter.now().UTC()
	items := make([]domain.KnowledgeItem, 0, len(response.Results))
	type hydrationKey struct {
		documentID string
		language   domain.Language
	}
	hydrated := make(map[hydrationKey]document, len(response.Results))
	for _, result := range response.Results {
		key := hydrationKey{
			documentID: result.Document.DocumentID,
			language:   domain.Language(result.Document.Language),
		}
		fullDocument, ok := hydrated[key]
		if !ok {
			var err error
			fullDocument, err = adapter.fetchDocument(ctx, options.Market, key.documentID, key.language)
			if errors.Is(err, errDocumentNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			hydrated[key] = fullDocument
		}
		item, err := mapDocument(fullDocument, options.Market, capturedAt, result.MatchingParagraph)
		if err != nil {
			return nil, err
		}
		if len(item.Applicability) == 0 && options.Applicability.ModelSlug != nil {
			model := *options.Applicability.ModelSlug
			item.Applicability = []domain.Applicability{{ModelSlug: &model}}
		}
		items = append(items, item)
		adapter.mu.Lock()
		adapter.references[item.ID] = documentRef{Market: item.Market, DocumentID: item.SourceID, Language: item.Language}
		adapter.mu.Unlock()
	}
	adapter.recordSnapshot(canonicalMarket(options.Market), resultVersion(query, items))
	return items, nil
}

func (adapter *Adapter) Get(ctx context.Context, itemID string) (domain.KnowledgeItem, error) {
	adapter.mu.RLock()
	reference, ok := adapter.references[itemID]
	adapter.mu.RUnlock()
	if !ok {
		return domain.KnowledgeItem{}, errors.New("support item must be discovered by search before Get")
	}
	value, err := adapter.fetchDocument(ctx, reference.Market, reference.DocumentID, reference.Language)
	if err != nil {
		return domain.KnowledgeItem{}, err
	}
	return mapDocument(value, reference.Market, adapter.now().UTC(), nil)
}

func (adapter *Adapter) fetchDocument(
	ctx context.Context,
	market domain.Market,
	documentID string,
	language domain.Language,
) (document, error) {
	var data documentData
	variables := map[string]any{
		"market": upstreamMarket(market), "documentID": documentID,
		"languages": []string{string(language)},
	}
	if err := adapter.client.query(ctx, documentQuery, variables, &data); err != nil {
		return document{}, err
	}
	if data.Market == nil {
		return document{}, fmt.Errorf("Volvo support market %q was not found", variables["market"])
	}
	if data.Market.Document == nil && language != "" {
		// An omitted language asks the source for the market's default document.
		delete(variables, "languages")
		if err := adapter.client.query(ctx, documentQuery, variables, &data); err != nil {
			return document{}, err
		}
		if data.Market == nil {
			return document{}, fmt.Errorf("Volvo support market %q was not found", variables["market"])
		}
	}
	if data.Market.Document == nil {
		return document{}, errDocumentNotFound
	}
	return *data.Market.Document, nil
}

func (adapter *Adapter) Status(_ context.Context, market domain.Market) (domain.SourceSnapshot, error) {
	market = canonicalMarket(market)
	adapter.mu.RLock()
	snapshot, ok := adapter.snapshots[market]
	adapter.mu.RUnlock()
	if ok {
		return snapshot, nil
	}
	return domain.SourceSnapshot{
		SourceType: domain.SourceTypeSupportArticle, Market: market,
		FetchStatus: domain.FetchStatusStale, FreshnessTier: domain.FreshnessTierStable,
	}, nil
}

func (adapter *Adapter) recordSnapshot(market domain.Market, version string) domain.SourceSnapshot {
	snapshot := domain.SourceSnapshot{
		SourceType: domain.SourceTypeSupportArticle, Market: market, VersionOrHash: version,
		FetchStatus: domain.FetchStatusOK, FreshnessTier: domain.FreshnessTierStable, LastFetchedAt: adapter.now().UTC(),
	}
	adapter.mu.Lock()
	adapter.snapshots[market] = snapshot
	adapter.mu.Unlock()
	return snapshot
}

var _ baseadapter.Adapter = (*Adapter)(nil)
