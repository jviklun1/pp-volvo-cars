package store

import (
	"context"
	"database/sql"
	"errors"
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

func TestReplaceSourceMarketPrunesOnlyTheCompletedSlice(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "knowledge.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	keep := replacementTestItem(domain.SourceTypeMarketPage, "keep", "se", "sv", "keep original")
	removed := replacementTestItem(domain.SourceTypeMarketPage, "removed", "se", "sv", "obsolete page text")
	otherMarket := replacementTestItem(domain.SourceTypeMarketPage, "other-market", "gb", "en", "other market")
	otherSource := replacementTestItem(domain.SourceTypeSupportArticle, "other-source", "se", "sv", "other source")
	for _, item := range []domain.KnowledgeItem{keep, removed, otherMarket, otherSource} {
		if err := database.UpsertItem(ctx, item); err != nil {
			t.Fatalf("UpsertItem(%q) error = %v", item.SourceID, err)
		}
	}

	changed := keep
	changed.Body = "keep changed"
	changed.ContentHash = domain.DeriveContentHash(changed.Body)
	added := replacementTestItem(domain.SourceTypeMarketPage, "added", "se", "sv", "new page")
	if err := database.ReplaceSourceMarket(
		ctx,
		domain.SourceTypeMarketPage,
		"se",
		[]domain.KnowledgeItem{changed, added},
	); err != nil {
		t.Fatalf("ReplaceSourceMarket() error = %v", err)
	}

	for _, want := range []domain.KnowledgeItem{changed, added, otherMarket, otherSource} {
		got, err := database.GetItem(ctx, want.ID)
		if err != nil {
			t.Fatalf("GetItem(%q) error = %v", want.SourceID, err)
		}
		if got.Body != want.Body {
			t.Errorf("GetItem(%q).Body = %q, want %q", want.SourceID, got.Body, want.Body)
		}
	}
	if _, err := database.GetItem(ctx, removed.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetItem(removed) error = %v, want sql.ErrNoRows", err)
	}

	results, err := database.SearchFTS(ctx, "obsolete", "se", nil, domain.ApplicabilityFilter{}, 10)
	if err != nil {
		t.Fatalf("SearchFTS() error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchFTS() returned pruned item: %#v", results)
	}

	for _, table := range []string{"citations", "applicability"} {
		var count int
		if err := database.db.QueryRowContext(
			ctx,
			`SELECT count(*) FROM `+table+` WHERE knowledge_item_id = ?`,
			removed.ID,
		).Scan(&count); err != nil {
			t.Fatalf("count pruned %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("pruned %s count = %d, want 0", table, count)
		}
	}

	var diffCount int
	if err := database.db.QueryRowContext(ctx, `SELECT count(*) FROM content_diffs`).Scan(&diffCount); err != nil {
		t.Fatal(err)
	}
	if diffCount != 1 {
		t.Fatalf("content diff count = %d, want one changed survivor and no deletion diff", diffCount)
	}
	var diffItemID string
	if err := database.db.QueryRowContext(ctx, `SELECT knowledge_item_id FROM content_diffs`).Scan(&diffItemID); err != nil {
		t.Fatal(err)
	}
	if diffItemID != keep.ID {
		t.Errorf("content diff item = %q, want changed survivor %q", diffItemID, keep.ID)
	}
}

func TestReplaceSourceMarketRollsBackOnMidTransactionFailure(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "knowledge.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	original := replacementTestItem(domain.SourceTypeMarketPage, "original", "se", "sv", "original body")
	if err := database.UpsertItem(ctx, original); err != nil {
		t.Fatal(err)
	}

	changed := original
	changed.Body = "changed body"
	changed.ContentHash = domain.DeriveContentHash(changed.Body)
	invalid := replacementTestItem(domain.SourceTypeMarketPage, "invalid", "se", "sv", "invalid body")
	invalid.ContentHash = "not-the-derived-hash"
	if err := database.ReplaceSourceMarket(
		ctx,
		domain.SourceTypeMarketPage,
		"se",
		[]domain.KnowledgeItem{changed, invalid},
	); err == nil {
		t.Fatal("ReplaceSourceMarket() error = nil, want invalid content hash")
	}

	got, err := database.GetItem(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != original.Body || got.ContentHash != original.ContentHash {
		t.Fatalf("rolled-back item = (%q, %q), want (%q, %q)", got.Body, got.ContentHash, original.Body, original.ContentHash)
	}
	if _, err := database.GetItem(ctx, invalid.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetItem(invalid) error = %v, want sql.ErrNoRows", err)
	}
	var diffCount int
	if err := database.db.QueryRowContext(ctx, `SELECT count(*) FROM content_diffs`).Scan(&diffCount); err != nil {
		t.Fatal(err)
	}
	if diffCount != 0 {
		t.Fatalf("content diff count after rollback = %d, want 0", diffCount)
	}
}

func TestReplaceSourceMarketRejectsMismatchedItemScope(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "knowledge.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	item := replacementTestItem(domain.SourceTypeMarketPage, "gb-page", "gb", "en", "wrong market")
	if err := database.ReplaceSourceMarket(
		ctx,
		domain.SourceTypeMarketPage,
		"se",
		[]domain.KnowledgeItem{item},
	); err == nil {
		t.Fatal("ReplaceSourceMarket() error = nil, want market mismatch")
	}
	if _, err := database.GetItem(ctx, item.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetItem(mismatched) error = %v, want sql.ErrNoRows", err)
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

func replacementTestItem(
	sourceType domain.SourceType,
	sourceID string,
	market domain.Market,
	language domain.Language,
	body string,
) domain.KnowledgeItem {
	capturedAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	model := domain.ModelSlug("ex30")
	item := domain.KnowledgeItem{
		SourceType: sourceType,
		SourceID:   sourceID,
		Market:     market,
		Language:   language,
		Title:      sourceID,
		Body:       body,
		CapturedAt: capturedAt,
		Applicability: []domain.Applicability{
			{ModelSlug: &model},
		},
		Citations: []domain.Citation{
			{
				SourceURL:  "https://www.volvocars.com/" + string(market) + "/" + sourceID,
				Excerpt:    body,
				Language:   language,
				CapturedAt: capturedAt,
			},
		},
	}
	item.ID = domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	item.ContentHash = domain.DeriveContentHash(item.Body)
	return item
}
