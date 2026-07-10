package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

func TestStoreBootstrapFTSAndIdempotency(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "knowledge.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	model := domain.ModelSlug("ex30")
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	item := domain.KnowledgeItem{
		SourceType: domain.SourceTypeSupportArticle,
		SourceID:   "charging-123",
		Market:     domain.Market("se"),
		Language:   domain.Language("sv"),
		Title:      "Ladda bilen",
		Body:       "Guide till snabb charging hemma",
		CapturedAt: now,
		Applicability: []domain.Applicability{
			{ModelSlug: &model},
		},
		Citations: []domain.Citation{
			{SourceURL: "https://www.volvocars.com/se/support/charging", Excerpt: "Guide till laddning", Language: "sv", CapturedAt: now},
		},
	}
	item.ID = domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	item.ContentHash = domain.DeriveContentHash(item.Body)

	for range 2 {
		if err := store.UpsertItem(ctx, item); err != nil {
			t.Fatalf("UpsertItem() error = %v", err)
		}
	}

	results, err := store.SearchFTS(ctx, "charging", item.Market, nil, domain.ApplicabilityFilter{}, 10)
	if err != nil {
		t.Fatalf("SearchFTS() error = %v", err)
	}
	if len(results) != 1 || results[0].ItemID != item.ID {
		t.Fatalf("SearchFTS() = %#v, want one candidate", results)
	}

	for table, want := range map[string]int{"knowledge_items": 1, "applicability": 1, "citations": 1, "content_diffs": 0} {
		var got int
		if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s count = %d, want %d", table, got, want)
		}
	}

	item.Body = "Updated charging guidance"
	item.ContentHash = domain.DeriveContentHash(item.Body)
	if err := store.UpsertItem(ctx, item); err != nil {
		t.Fatalf("changed UpsertItem() error = %v", err)
	}
	var diffCount int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM content_diffs`).Scan(&diffCount); err != nil {
		t.Fatal(err)
	}
	if diffCount != 1 {
		t.Fatalf("content diff count = %d, want 1", diffCount)
	}

	if err := store.DeleteItem(ctx, item.ID); err != nil {
		t.Fatalf("DeleteItem() error = %v", err)
	}
	results, err = store.SearchFTS(ctx, "charging", item.Market, nil, domain.ApplicabilityFilter{}, 10)
	if err != nil {
		t.Fatalf("SearchFTS() after delete error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchFTS() after delete returned %d items", len(results))
	}
}

func TestIsStale(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "knowledge.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	stale, tier, err := store.IsStale(ctx, domain.SourceTypeOffer, domain.Market("se"), now)
	if err != nil || !stale || tier != domain.FreshnessTierVolatile {
		t.Fatalf("missing snapshot = (%v, %q, %v)", stale, tier, err)
	}

	if err := store.SaveSnapshot(ctx, domain.SourceSnapshot{
		SourceType: domain.SourceTypeOffer, Market: domain.Market("se"), FetchStatus: domain.FetchStatusOK,
		FreshnessTier: domain.FreshnessTierVolatile, LastFetchedAt: now.Add(-23 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	stale, tier, err = store.IsStale(ctx, domain.SourceTypeOffer, domain.Market("se"), now)
	if err != nil || stale || tier != domain.FreshnessTierVolatile {
		t.Fatalf("fresh snapshot = (%v, %q, %v)", stale, tier, err)
	}
}

func TestEmptyCollectionsAreNonNil(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "knowledge.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	markets, err := store.ListMarkets(ctx)
	if err != nil || markets == nil || len(markets) != 0 {
		t.Fatalf("ListMarkets() = %#v, %v; want non-nil empty slice", markets, err)
	}
	models, err := store.ListModels(ctx, "se")
	if err != nil || models == nil || len(models) != 0 {
		t.Fatalf("ListModels() = %#v, %v; want non-nil empty slice", models, err)
	}
	snapshots, err := store.ListSnapshots(ctx, "se")
	if err != nil || snapshots == nil || len(snapshots) != 0 {
		t.Fatalf("ListSnapshots() = %#v, %v; want non-nil empty slice", snapshots, err)
	}
	diffs, err := store.ListContentDiffs(ctx, "se", nil, time.Time{})
	if err != nil || diffs == nil || len(diffs) != 0 {
		t.Fatalf("ListContentDiffs() = %#v, %v; want non-nil empty slice", diffs, err)
	}
}
