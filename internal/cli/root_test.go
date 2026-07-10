package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/volvo/internal/adapter"
	"example.com/volvo/internal/domain"
	exiterrors "example.com/volvo/internal/errors"
	"example.com/volvo/internal/search"
	"example.com/volvo/internal/store"
)

func TestMarketsJSONGoldenAndSearchShape(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "cli.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.SaveMarket(ctx, store.MarketRecord{
		ID: "se", Caption: "Sweden", SourceTypes: []domain.SourceType{domain.SourceTypeSupportArticle},
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	item := domain.KnowledgeItem{
		SourceType: domain.SourceTypeSupportArticle, SourceID: "x", Market: "se", Language: "sv",
		Title: "X", Body: "x evidence", CapturedAt: now,
	}
	item.RawJSON = []byte(`{"internal":"payload"}`)
	item.ID = domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	item.ContentHash = domain.DeriveContentHash(item.Body)
	if err := database.UpsertItem(ctx, item); err != nil {
		t.Fatal(err)
	}

	app := func(output *bytes.Buffer) App {
		return App{Store: database, Search: search.New(search.NewLexical(database), nil), Out: output, Now: func() time.Time { return now }}
	}

	var output bytes.Buffer
	root := NewRoot(app(&output))
	root.SetArgs([]string{"markets", "list", "--json"})
	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/markets.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != string(want) {
		t.Fatalf("markets JSON:\n%s\nwant:\n%s", output.String(), want)
	}

	output.Reset()
	root = NewRoot(app(&output))
	root.SetArgs([]string{"search", "x", "--market", "se", "--data-source", "local", "--json"})
	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Market  string `json:"market"`
		Query   string `json:"query"`
		Results []struct {
			Score float64 `json:"score"`
			Item  struct {
				ID string `json:"id"`
			} `json:"item"`
		} `json:"results"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("invalid search JSON: %v\n%s", err, output.String())
	}
	if strings.Contains(output.String(), "raw_json") || strings.Contains(output.String(), "payload") {
		t.Fatalf("search JSON leaked internal raw payload: %s", output.String())
	}
	if response.Market != "se" || response.Query != "x" || len(response.Results) != 1 || response.Results[0].Item.ID != item.ID {
		t.Fatalf("unexpected search response: %#v", response)
	}
}

func TestGetRejectsLiveDataSource(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "cli.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	root := NewRoot(App{Store: database, Search: search.New(search.NewLexical(database), nil), Out: &bytes.Buffer{}})
	root.SetArgs([]string{"get", "id", "--data-source", "live"})
	err = root.ExecuteContext(ctx)
	if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments || !strings.Contains(err.Error(), "stored IDs") {
		t.Fatalf("get live error = %v, code = %d", err, exiterrors.CodeOf(err))
	}
}

func TestRootUnknownCommandUsesInvalidArguments(t *testing.T) {
	root := NewRoot(App{Out: &bytes.Buffer{}})
	root.SetArgs([]string{"frobnicate"})
	err := root.ExecuteContext(context.Background())
	if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unknown command error = %v, code = %d", err, exiterrors.CodeOf(err))
	}
}

func TestBareRootShowsHelp(t *testing.T) {
	var output bytes.Buffer
	root := NewRoot(App{Out: &output})
	root.SetArgs([]string{})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Usage:") {
		t.Fatalf("bare root output = %q, want help", output.String())
	}
}

func TestSearchRejectsBlankQuery(t *testing.T) {
	root := NewRoot(App{Out: &bytes.Buffer{}})
	root.SetArgs([]string{"search", "   ", "--market", "se", "--data-source", "local"})
	err := root.ExecuteContext(context.Background())
	if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments || !strings.Contains(err.Error(), "search query is required") {
		t.Fatalf("blank search error = %v, code = %d", err, exiterrors.CodeOf(err))
	}
}

func TestSyncPersistsCatalog(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "cli.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	sourceType := domain.SourceTypeSupportArticle
	source := catalogAdapter{result: adapter.SyncResult{
		Markets: []domain.MarketRef{{ID: "se", Caption: "Sweden"}},
		Models:  []domain.ModelRef{{Market: "se", Slug: "xc60", DisplayName: "XC60"}},
		Snapshot: domain.SourceSnapshot{
			SourceType: sourceType, Market: "se", FetchStatus: domain.FetchStatusOK,
			FreshnessTier: domain.FreshnessTierStable, LastFetchedAt: now,
		},
	}}
	root := NewRoot(App{
		Store: database, Adapters: map[domain.SourceType]adapter.Adapter{sourceType: source},
		Out: &bytes.Buffer{}, Now: func() time.Time { return now },
	})
	root.SetArgs([]string{"sync", "--market", "se", "--json"})
	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}

	markets, err := database.ListMarkets(ctx)
	if err != nil || len(markets) != 1 || markets[0].ID != "se" || markets[0].Caption != "Sweden" || len(markets[0].SourceTypes) != 1 || markets[0].SourceTypes[0] != sourceType {
		t.Fatalf("markets = %#v, error = %v", markets, err)
	}
	models, err := database.ListModels(ctx, "se")
	if err != nil || len(models) != 1 || models[0] != (store.ModelRecord{MarketID: "se", Slug: "xc60", DisplayName: "XC60"}) {
		t.Fatalf("models = %#v, error = %v", models, err)
	}
}

type catalogAdapter struct {
	result adapter.SyncResult
}

func (source catalogAdapter) CacheScope() adapter.CacheScope { return adapter.CacheScopeMarket }

func (source catalogAdapter) Sync(context.Context, domain.Market, adapter.SyncOptions) (adapter.SyncResult, error) {
	return source.result, nil
}

func (catalogAdapter) Search(context.Context, string, adapter.SearchOptions) ([]domain.KnowledgeItem, error) {
	return nil, nil
}

func (catalogAdapter) Get(context.Context, string) (domain.KnowledgeItem, error) {
	return domain.KnowledgeItem{}, nil
}

func (catalogAdapter) Status(context.Context, domain.Market) (domain.SourceSnapshot, error) {
	return domain.SourceSnapshot{}, nil
}
