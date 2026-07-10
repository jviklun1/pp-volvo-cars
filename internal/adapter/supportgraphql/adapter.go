package supportgraphql

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	baseadapter "example.com/volvo/internal/adapter"
	"example.com/volvo/internal/domain"
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
            jsonContent { body jsonContentSchemaVersion }
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
              jsonContent { body jsonContentSchemaVersion }
              applicableTo { title values }
            }
          }
        }
      }
    }
  }
}`

const marketsQuery = `query { markets { id caption } }`

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
	for _, value := range data.Markets {
		if value.ID == upstream {
			found = true
			break
		}
	}
	if !found {
		return baseadapter.SyncResult{}, fmt.Errorf("Volvo support market %q was not found", upstream)
	}
	snapshot := adapter.recordSnapshot(canonicalMarket(market), "availability")
	return baseadapter.SyncResult{Snapshot: snapshot}, nil
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
	for _, result := range response.Results {
		item, err := mapDocument(result.Document, options.Market, capturedAt, result.MatchingParagraph)
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
	var data documentData
	variables := map[string]any{
		"market": upstreamMarket(reference.Market), "documentID": reference.DocumentID,
		"languages": []string{string(reference.Language)},
	}
	if err := adapter.client.query(ctx, documentQuery, variables, &data); err != nil {
		return domain.KnowledgeItem{}, err
	}
	if data.Market == nil || data.Market.Document == nil {
		return domain.KnowledgeItem{}, errors.New("support document was not found")
	}
	return mapDocument(*data.Market.Document, reference.Market, adapter.now().UTC(), nil)
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
