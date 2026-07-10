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
