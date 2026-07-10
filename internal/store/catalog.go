package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

type MarketRecord struct {
	ID          domain.Market       `json:"id"`
	Caption     string              `json:"caption"`
	SourceTypes []domain.SourceType `json:"source_types"`
}

type ModelRecord struct {
	MarketID    domain.Market    `json:"market"`
	Slug        domain.ModelSlug `json:"slug"`
	DisplayName string           `json:"display_name"`
}

func (store *Store) SaveMarket(ctx context.Context, market MarketRecord) error {
	sources, err := json.Marshal(market.SourceTypes)
	if err != nil {
		return err
	}
	_, err = store.db.ExecContext(ctx, `
        INSERT INTO markets(id, caption, source_types) VALUES (?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET caption = excluded.caption, source_types = excluded.source_types`,
		market.ID, market.Caption, string(sources))
	return err
}

func (store *Store) ListMarkets(ctx context.Context) ([]MarketRecord, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT id, caption, source_types FROM markets ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	markets := make([]MarketRecord, 0)
	for rows.Next() {
		var market MarketRecord
		var sources string
		if err := rows.Scan(&market.ID, &market.Caption, &sources); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(sources), &market.SourceTypes); err != nil {
			return nil, fmt.Errorf("decode market source types: %w", err)
		}
		markets = append(markets, market)
	}
	return markets, rows.Err()
}

func (store *Store) SaveModel(ctx context.Context, model ModelRecord) error {
	_, err := store.db.ExecContext(ctx, `
        INSERT INTO models(market_id, slug, display_name) VALUES (?, ?, ?)
        ON CONFLICT(market_id, slug) DO UPDATE SET display_name = excluded.display_name`,
		model.MarketID, model.Slug, model.DisplayName)
	return err
}

func (store *Store) ListModels(ctx context.Context, market domain.Market) ([]ModelRecord, error) {
	rows, err := store.db.QueryContext(ctx, `
        SELECT market_id, slug, display_name FROM models WHERE market_id = ? ORDER BY slug`, market)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := make([]ModelRecord, 0)
	for rows.Next() {
		var model ModelRecord
		if err := rows.Scan(&model.MarketID, &model.Slug, &model.DisplayName); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func (store *Store) ListSnapshots(ctx context.Context, market domain.Market) ([]domain.SourceSnapshot, error) {
	rows, err := store.db.QueryContext(ctx, `
        SELECT source_type, market, version_or_hash, fetch_status, freshness_tier, last_fetched_at
        FROM source_snapshots WHERE market = ? ORDER BY source_type`, market)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	snapshots := make([]domain.SourceSnapshot, 0)
	for rows.Next() {
		var snapshot domain.SourceSnapshot
		var fetchedAt string
		if err := rows.Scan(&snapshot.SourceType, &snapshot.Market, &snapshot.VersionOrHash,
			&snapshot.FetchStatus, &snapshot.FreshnessTier, &fetchedAt); err != nil {
			return nil, err
		}
		snapshot.LastFetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
		if err != nil {
			return nil, fmt.Errorf("parse snapshot time: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}
