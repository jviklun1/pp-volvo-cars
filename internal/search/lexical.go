package search

import (
	"context"
	"errors"
	"strings"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

// LexicalStore is the minimal FTS capability required by lexical search.
type LexicalStore interface {
	SearchFTS(
		context.Context,
		string,
		domain.Market,
		*domain.Language,
		domain.ApplicabilityFilter,
		int,
	) ([]domain.SearchCandidate, error)
}

// Lexical performs safely quoted FTS searches against the normalized store.
type Lexical struct {
	store LexicalStore
}

func NewLexical(store LexicalStore) *Lexical {
	return &Lexical{store: store}
}

func (lexical *Lexical) Search(
	ctx context.Context,
	query string,
	market domain.Market,
	language *domain.Language,
	filter domain.ApplicabilityFilter,
	limit int,
) ([]domain.SearchCandidate, error) {
	ftsQuery, err := quoteFTSQuery(query)
	if err != nil {
		return nil, err
	}
	return lexical.store.SearchFTS(ctx, ftsQuery, market, language, filter, limit)
}

func quoteFTSQuery(query string) (string, error) {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return "", errors.New("search query is required")
	}
	for index, field := range fields {
		fields[index] = `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return strings.Join(fields, " "), nil
}
