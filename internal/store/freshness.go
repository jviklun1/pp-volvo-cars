package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

const (
	StableMaxAge   = 7 * 24 * time.Hour
	VolatileMaxAge = 24 * time.Hour
)

// FreshnessTierForSource returns the configured freshness tier for a source type.
func FreshnessTierForSource(sourceType domain.SourceType) domain.FreshnessTier {
	switch sourceType {
	case domain.SourceTypeMarketPage, domain.SourceTypeOffer, domain.SourceTypePricing, domain.SourceTypeCampaign:
		return domain.FreshnessTierVolatile
	default:
		return domain.FreshnessTierStable
	}
}

// SaveSnapshot records the latest fetch status for a source-market slice.
func (store *Store) SaveSnapshot(ctx context.Context, snapshot domain.SourceSnapshot) error {
	_, err := store.db.ExecContext(ctx, `
        INSERT INTO source_snapshots(
            source_type, market, version_or_hash, fetch_status, freshness_tier, last_fetched_at
        ) VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(source_type, market) DO UPDATE SET
            version_or_hash = excluded.version_or_hash,
            fetch_status = excluded.fetch_status,
            freshness_tier = excluded.freshness_tier,
            last_fetched_at = excluded.last_fetched_at`,
		snapshot.SourceType, snapshot.Market, snapshot.VersionOrHash, snapshot.FetchStatus,
		snapshot.FreshnessTier, snapshot.LastFetchedAt.UTC().Format(time.RFC3339Nano))
	return err
}

// IsStale reports freshness without importing or invoking an adapter.
func (store *Store) IsStale(
	ctx context.Context,
	sourceType domain.SourceType,
	market domain.Market,
	now time.Time,
) (bool, domain.FreshnessTier, error) {
	tier := FreshnessTierForSource(sourceType)
	var storedTier, fetchedAt string
	err := store.db.QueryRowContext(ctx, `
        SELECT freshness_tier, last_fetched_at FROM source_snapshots
        WHERE source_type = ? AND market = ?`, sourceType, market).Scan(&storedTier, &fetchedAt)
	if err == sql.ErrNoRows {
		return true, tier, nil
	}
	if err != nil {
		return false, tier, fmt.Errorf("read source freshness: %w", err)
	}
	tier = domain.FreshnessTier(storedTier)
	lastFetched, err := time.Parse(time.RFC3339Nano, fetchedAt)
	if err != nil {
		return false, tier, fmt.Errorf("parse source freshness: %w", err)
	}
	maxAge := StableMaxAge
	if tier == domain.FreshnessTierVolatile {
		maxAge = VolatileMaxAge
	}
	return now.Sub(lastFetched) > maxAge, tier, nil
}
