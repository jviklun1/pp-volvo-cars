// Package adapter defines the source boundary for official Volvo content.
package adapter

import (
	"context"

	"example.com/volvo/internal/domain"
)

// SyncOptions controls an adapter refresh without leaking source-specific flags.
type SyncOptions struct {
	DryRun bool
}

// SyncResult is the normalized output of refreshing one source-market slice.
type SyncResult struct {
	Items    []domain.KnowledgeItem
	Snapshot domain.SourceSnapshot
}

// SearchOptions carries the explicit market and optional applicability context.
type SearchOptions struct {
	Market        domain.Market
	Language      *domain.Language
	Applicability domain.ApplicabilityFilter
	Limit         int
}

// Adapter retrieves one official source while exposing only normalized types.
type Adapter interface {
	Sync(ctx context.Context, market domain.Market, options SyncOptions) (SyncResult, error)
	Search(ctx context.Context, query string, options SearchOptions) ([]domain.KnowledgeItem, error)
	Get(ctx context.Context, itemID string) (domain.KnowledgeItem, error)
	Status(ctx context.Context, market domain.Market) (domain.SourceSnapshot, error)
}
