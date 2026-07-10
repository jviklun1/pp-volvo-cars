// Package store owns the local normalized SQLite knowledge store.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Store is a local SQLite knowledge store.
type Store struct {
	db *sql.DB
}

// Open opens a store and idempotently applies the current schema.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply sqlite schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (store *Store) Close() error {
	return store.db.Close()
}

// UpsertItem transactionally replaces an item and its child evidence rows.
func (store *Store) UpsertItem(ctx context.Context, item domain.KnowledgeItem) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin item upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := upsertItemTx(ctx, tx, item); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit item upsert: %w", err)
	}
	return nil
}

// ReplaceSourceMarket atomically replaces one complete source-market slice.
func (store *Store) ReplaceSourceMarket(
	ctx context.Context,
	sourceType domain.SourceType,
	market domain.Market,
	items []domain.KnowledgeItem,
) error {
	if !sourceType.Valid() {
		return fmt.Errorf("invalid replacement source type %q", sourceType)
	}
	if strings.TrimSpace(string(market)) == "" {
		return fmt.Errorf("replacement market is required")
	}

	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin source-market replacement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	retained := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.SourceType != sourceType || item.Market != market {
			return fmt.Errorf(
				"replacement item %q belongs to source %q market %q, want source %q market %q",
				item.ID, item.SourceType, item.Market, sourceType, market,
			)
		}
		if err := upsertItemTx(ctx, tx, item); err != nil {
			return err
		}
		retained[item.ID] = struct{}{}
	}

	rows, err := tx.QueryContext(ctx, `
        SELECT id FROM knowledge_items
        WHERE source_type = ? AND market = ?`, sourceType, market)
	if err != nil {
		return fmt.Errorf("query obsolete source-market items: %w", err)
	}
	var obsolete []string
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan obsolete source-market item: %w", err)
		}
		if _, ok := retained[itemID]; !ok {
			obsolete = append(obsolete, itemID)
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close obsolete source-market items: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate obsolete source-market items: %w", err)
	}
	for _, itemID := range obsolete {
		if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_items WHERE id = ?`, itemID); err != nil {
			return fmt.Errorf("delete obsolete source-market item %q: %w", itemID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit source-market replacement: %w", err)
	}
	return nil
}

func upsertItemTx(ctx context.Context, tx *sql.Tx, item domain.KnowledgeItem) error {
	if err := item.Valid(); err != nil {
		return err
	}
	if want := domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language); item.ID != want {
		return fmt.Errorf("knowledge item ID does not match its identity fields")
	}
	if want := domain.DeriveContentHash(item.Body); item.ContentHash != want {
		return fmt.Errorf("knowledge item content hash does not match its body")
	}

	var oldHash string
	err := tx.QueryRowContext(ctx, `SELECT content_hash FROM knowledge_items WHERE id = ?`, item.ID).Scan(&oldHash)
	if err != nil && !stderrors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read existing content hash: %w", err)
	}
	if err == nil && oldHash != item.ContentHash {
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO content_diffs(knowledge_item_id, old_content_hash, new_content_hash, detected_at)
            VALUES (?, ?, ?, ?)`, item.ID, oldHash, item.ContentHash, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record content diff: %w", err)
		}
	}

	for _, table := range []string{"citations", "applicability"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE knowledge_item_id = ?`, item.ID); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
        INSERT INTO knowledge_items(
            id, source_type, source_id, market, language, title, body, raw_json, captured_at, content_hash
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            source_type = excluded.source_type,
            source_id = excluded.source_id,
            market = excluded.market,
            language = excluded.language,
            title = excluded.title,
            body = excluded.body,
            raw_json = excluded.raw_json,
            captured_at = excluded.captured_at,
            content_hash = excluded.content_hash`,
		item.ID, item.SourceType, item.SourceID, item.Market, item.Language, item.Title, item.Body,
		item.RawJSON, item.CapturedAt.UTC().Format(time.RFC3339Nano), item.ContentHash); err != nil {
		return fmt.Errorf("upsert knowledge item: %w", err)
	}

	for _, applicability := range item.Applicability {
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO applicability(knowledge_item_id, model_slug, model_year, powertrain)
            VALUES (?, ?, ?, ?)`, item.ID, applicability.ModelSlug, applicability.ModelYear, applicability.Powertrain); err != nil {
			return fmt.Errorf("insert applicability: %w", err)
		}
	}
	for _, citation := range item.Citations {
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO citations(knowledge_item_id, source_url, excerpt, language, captured_at)
            VALUES (?, ?, ?, ?, ?)`, item.ID, citation.SourceURL, citation.Excerpt, citation.Language,
			citation.CapturedAt.UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("insert citation: %w", err)
		}
	}

	return nil
}

// GetItem returns a fully hydrated item by synthetic ID.
func (store *Store) GetItem(ctx context.Context, itemID string) (domain.KnowledgeItem, error) {
	var item domain.KnowledgeItem
	var sourceType, market, language, capturedAt string
	err := store.db.QueryRowContext(ctx, `
        SELECT id, source_type, source_id, market, language, title, body, raw_json, captured_at, content_hash
        FROM knowledge_items WHERE id = ?`, itemID).Scan(
		&item.ID, &sourceType, &item.SourceID, &market, &language, &item.Title, &item.Body,
		&item.RawJSON, &capturedAt, &item.ContentHash,
	)
	if err != nil {
		return domain.KnowledgeItem{}, err
	}
	item.SourceType = domain.SourceType(sourceType)
	item.Market = domain.Market(market)
	item.Language = domain.Language(language)
	item.CapturedAt, err = time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		return domain.KnowledgeItem{}, fmt.Errorf("parse captured time: %w", err)
	}

	item.Applicability, err = store.loadApplicability(ctx, itemID)
	if err != nil {
		return domain.KnowledgeItem{}, err
	}
	item.Citations, err = store.loadCitations(ctx, itemID)
	if err != nil {
		return domain.KnowledgeItem{}, err
	}
	return item, nil
}

func (store *Store) loadApplicability(ctx context.Context, itemID string) ([]domain.Applicability, error) {
	rows, err := store.db.QueryContext(ctx, `
        SELECT model_slug, model_year, powertrain FROM applicability
        WHERE knowledge_item_id = ? ORDER BY id`, itemID)
	if err != nil {
		return nil, fmt.Errorf("query applicability: %w", err)
	}
	defer rows.Close()
	var result []domain.Applicability
	for rows.Next() {
		var model, powertrain sql.NullString
		var year sql.NullInt64
		if err := rows.Scan(&model, &year, &powertrain); err != nil {
			return nil, fmt.Errorf("scan applicability: %w", err)
		}
		var value domain.Applicability
		if model.Valid {
			modelSlug := domain.ModelSlug(model.String)
			value.ModelSlug = &modelSlug
		}
		if year.Valid {
			modelYear := domain.ModelYear(year.Int64)
			value.ModelYear = &modelYear
		}
		if powertrain.Valid {
			powertrainValue := domain.Powertrain(powertrain.String)
			value.Powertrain = &powertrainValue
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (store *Store) loadCitations(ctx context.Context, itemID string) ([]domain.Citation, error) {
	rows, err := store.db.QueryContext(ctx, `
        SELECT source_url, excerpt, language, captured_at FROM citations
        WHERE knowledge_item_id = ? ORDER BY id`, itemID)
	if err != nil {
		return nil, fmt.Errorf("query citations: %w", err)
	}
	defer rows.Close()
	var result []domain.Citation
	for rows.Next() {
		var citation domain.Citation
		var language, capturedAt string
		if err := rows.Scan(&citation.SourceURL, &citation.Excerpt, &language, &capturedAt); err != nil {
			return nil, fmt.Errorf("scan citation: %w", err)
		}
		citation.Language = domain.Language(language)
		citation.CapturedAt, err = time.Parse(time.RFC3339Nano, capturedAt)
		if err != nil {
			return nil, fmt.Errorf("parse citation captured time: %w", err)
		}
		result = append(result, citation)
	}
	return result, rows.Err()
}

// DeleteItem deletes an item and its child rows; the FTS delete trigger follows it.
func (store *Store) DeleteItem(ctx context.Context, itemID string) error {
	_, err := store.db.ExecContext(ctx, `DELETE FROM knowledge_items WHERE id = ?`, itemID)
	return err
}
