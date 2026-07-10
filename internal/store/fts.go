package store

import (
	"context"
	"fmt"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

// SearchFTS searches one explicit market and optional language.
func (store *Store) SearchFTS(
	ctx context.Context,
	query string,
	market domain.Market,
	language *domain.Language,
	filter domain.ApplicabilityFilter,
	limit int,
) ([]domain.SearchCandidate, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := store.db.QueryContext(ctx, `
        SELECT knowledge_items.id, -bm25(knowledge_items_fts)
        FROM knowledge_items_fts
        JOIN knowledge_items ON knowledge_items.rowid = knowledge_items_fts.rowid
        WHERE knowledge_items_fts MATCH ?
          AND knowledge_items.market = ?
          AND (? IS NULL OR knowledge_items.language = ?)
          AND (
              NOT EXISTS (
                  SELECT 1 FROM applicability a0 WHERE a0.knowledge_item_id = knowledge_items.id
              )
              OR EXISTS (
                  SELECT 1 FROM applicability a WHERE a.knowledge_item_id = knowledge_items.id
                    AND (? IS NULL OR a.model_slug IS NULL OR a.model_slug = ?)
                    AND (? IS NULL OR a.model_year IS NULL OR a.model_year = ?)
                    AND (? IS NULL OR a.powertrain IS NULL OR a.powertrain = ?)
              )
          )
        ORDER BY bm25(knowledge_items_fts)
        LIMIT ?`, query, market, language, language,
		filter.ModelSlug, filter.ModelSlug, filter.ModelYear, filter.ModelYear,
		filter.Powertrain, filter.Powertrain, limit)
	if err != nil {
		return nil, fmt.Errorf("search FTS: %w", err)
	}
	var results []domain.SearchCandidate
	for rows.Next() {
		var candidate domain.SearchCandidate
		if err := rows.Scan(&candidate.ItemID, &candidate.Score); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan FTS result: %w", err)
		}
		results = append(results, candidate)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	return results, nil
}
