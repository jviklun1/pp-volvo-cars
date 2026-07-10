package adapter

import (
	"context"
	"testing"

	"example.com/volvo/internal/domain"
)

type stubAdapter struct{}

func (stubAdapter) CacheScope() CacheScope { return CacheScopeMarket }

func (stubAdapter) Sync(context.Context, domain.Market, SyncOptions) (SyncResult, error) {
	return SyncResult{}, nil
}

func (stubAdapter) Search(context.Context, string, SearchOptions) ([]domain.KnowledgeItem, error) {
	return nil, nil
}

func (stubAdapter) Get(context.Context, string) (domain.KnowledgeItem, error) {
	return domain.KnowledgeItem{}, nil
}

func (stubAdapter) Status(context.Context, domain.Market) (domain.SourceSnapshot, error) {
	return domain.SourceSnapshot{}, nil
}

func TestAdapterContract(t *testing.T) {
	var _ Adapter = stubAdapter{}
}
