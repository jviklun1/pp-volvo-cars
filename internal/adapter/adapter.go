// Package adapter defines the source boundary for official Volvo content.
package adapter

import (
	"context"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

// SyncOptions controls an adapter refresh without leaking source-specific flags.
type SyncOptions struct {
	DryRun bool
}

// SyncResult is the normalized output of refreshing one source-market slice.
type SyncResult struct {
	Items    []domain.KnowledgeItem
	Markets  []domain.MarketRef
	Models   []domain.ModelRef
	Snapshot domain.SourceSnapshot
}

// SearchOptions carries the explicit market and optional applicability context.
type SearchOptions struct {
	Market        domain.Market
	Language      *domain.Language
	Applicability domain.ApplicabilityFilter
	Limit         int
}

// CacheScope describes whether Sync can populate a whole market or whether
// content can only be captured in response to a specific search query.
type CacheScope string

const (
	CacheScopeMarket CacheScope = "market"
	CacheScopeQuery  CacheScope = "query"
)

// Adapter retrieves one official source while exposing only normalized types.
type Adapter interface {
	CacheScope() CacheScope
	Sync(ctx context.Context, market domain.Market, options SyncOptions) (SyncResult, error)
	Search(ctx context.Context, query string, options SearchOptions) ([]domain.KnowledgeItem, error)
	Get(ctx context.Context, itemID string) (domain.KnowledgeItem, error)
	Status(ctx context.Context, market domain.Market) (domain.SourceSnapshot, error)
}
