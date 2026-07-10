package store

import (
	"context"
	"fmt"
	"time"

	"example.com/volvo/internal/domain"
)

type ContentDiff struct {
	ItemID         string            `json:"item_id"`
	SourceType     domain.SourceType `json:"source_type"`
	Market         domain.Market     `json:"market"`
	OldContentHash string            `json:"old_content_hash"`
	NewContentHash string            `json:"new_content_hash"`
	DetectedAt     time.Time         `json:"detected_at"`
}

func (store *Store) ListContentDiffs(
	ctx context.Context,
	market domain.Market,
	sourceType *domain.SourceType,
	since time.Time,
) ([]ContentDiff, error) {
	rows, err := store.db.QueryContext(ctx, `
        SELECT d.knowledge_item_id, k.source_type, k.market,
               d.old_content_hash, d.new_content_hash, d.detected_at
        FROM content_diffs d
        JOIN knowledge_items k ON k.id = d.knowledge_item_id
        WHERE k.market = ? AND (? IS NULL OR k.source_type = ?) AND d.detected_at >= ?
        ORDER BY d.detected_at DESC, d.id DESC`, market, sourceType, sourceType, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	diffs := make([]ContentDiff, 0)
	for rows.Next() {
		var diff ContentDiff
		var detectedAt string
		if err := rows.Scan(&diff.ItemID, &diff.SourceType, &diff.Market, &diff.OldContentHash,
			&diff.NewContentHash, &detectedAt); err != nil {
			return nil, err
		}
		diff.DetectedAt, err = time.Parse(time.RFC3339Nano, detectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse content diff time: %w", err)
		}
		diffs = append(diffs, diff)
	}
	return diffs, rows.Err()
}
