package adapter

import (
	"context"
	"testing"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

type stubAdapter struct{}

type marketScopedStub struct {
	stubAdapter
	supported map[domain.Market]bool
}

func (stub marketScopedStub) SupportsMarket(market domain.Market) bool {
	return stub.supported[market]
}

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

func TestMarketSupporterIsOptional(t *testing.T) {
	var unscoped Adapter = stubAdapter{}
	if _, ok := unscoped.(MarketSupporter); ok {
		t.Fatal("plain adapters must not be treated as market-scoped")
	}

	var scoped Adapter = marketScopedStub{
		supported: map[domain.Market]bool{"se": true},
	}
	supporter, ok := scoped.(MarketSupporter)
	if !ok {
		t.Fatal("market-scoped adapter does not expose MarketSupporter")
	}
	if !supporter.SupportsMarket("se") {
		t.Error("expected se to be supported")
	}
	if supporter.SupportsMarket("us") {
		t.Error("expected us to be unsupported")
	}
}
