package store

import (
	"context"
	"fmt"

	"example.com/volvo/internal/domain"
)

// FTSResult is one hydrated lexical match and its SQLite BM25 rank.
type FTSResult struct {
	Item domain.KnowledgeItem
	Rank float64
}

// SearchFTS searches one explicit market and optional language.
func (store *Store) SearchFTS(
	ctx context.Context,
	query string,
	market domain.Market,
	language *domain.Language,
	limit int,
) ([]FTSResult, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := store.db.QueryContext(ctx, `
        SELECT knowledge_items.id, bm25(knowledge_items_fts)
        FROM knowledge_items_fts
        JOIN knowledge_items ON knowledge_items.rowid = knowledge_items_fts.rowid
        WHERE knowledge_items_fts MATCH ?
          AND knowledge_items.market = ?
          AND (? IS NULL OR knowledge_items.language = ?)
        ORDER BY bm25(knowledge_items_fts)
        LIMIT ?`, query, market, language, language, limit)
	if err != nil {
		return nil, fmt.Errorf("search FTS: %w", err)
	}
	type match struct {
		id   string
		rank float64
	}
	var matches []match
	for rows.Next() {
		var value match
		if err := rows.Scan(&value.id, &value.rank); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan FTS result: %w", err)
		}
		matches = append(matches, value)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	results := make([]FTSResult, 0, len(matches))
	for _, match := range matches {
		item, err := store.GetItem(ctx, match.id)
		if err != nil {
			return nil, fmt.Errorf("hydrate FTS result: %w", err)
		}
		results = append(results, FTSResult{Item: item, Rank: match.rank})
	}
	return results, nil
}
