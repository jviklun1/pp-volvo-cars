package search_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
	"github.com/jviklun1/pp-volvo-cars/internal/search"
	"github.com/jviklun1/pp-volvo-cars/internal/store"
)

func TestLexicalSearchAndPassthroughMatcher(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(ctx, filepath.Join(t.TempDir(), "search.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	item := domain.KnowledgeItem{
		SourceType: domain.SourceTypeSupportArticle, SourceID: "charging", Market: "se", Language: "sv",
		Title: "Charging", Body: "charging the car at home", CapturedAt: time.Now().UTC(),
	}
	item.ID = domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	item.ContentHash = domain.DeriveContentHash(item.Body)
	if err := database.UpsertItem(ctx, item); err != nil {
		t.Fatal(err)
	}

	engine := search.New(search.NewLexical(database), search.PassthroughMatcher{})
	language := domain.Language("sv")
	results, err := engine.Search(ctx, "charging", "se", &language, domain.ApplicabilityFilter{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ItemID != item.ID {
		t.Fatalf("Search() = %#v, want seeded item", results)
	}
}

func TestMergeRanksAndLimitsDeterministically(t *testing.T) {
	lexical := []domain.SearchCandidate{{ItemID: "a", Score: 1}, {ItemID: "b", Score: 2}}
	multilingual := []domain.SearchCandidate{{ItemID: "a", Score: 3}, {ItemID: "c", Score: 1}}
	got := search.Merge(2, lexical, multilingual)
	if len(got) != 2 || got[0].ItemID != "a" || got[0].Score != 4 || got[1].ItemID != "b" {
		t.Fatalf("Merge() = %#v", got)
	}
}
