// Package search composes lexical and optional multilingual retrieval.
package search

import (
	"context"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

// Matcher optionally returns cross-language candidates for a query.
type Matcher interface {
	Match(ctx context.Context, query string, targetLanguage domain.Language) ([]domain.SearchCandidate, error)
}

// PassthroughMatcher disables multilingual augmentation without special cases.
type PassthroughMatcher struct{}

func (PassthroughMatcher) Match(context.Context, string, domain.Language) ([]domain.SearchCandidate, error) {
	return nil, nil
}
