package search

import (
	"context"
	"sort"

	"example.com/volvo/internal/domain"
)

// Engine composes lexical retrieval with an optional multilingual matcher.
type Engine struct {
	lexical *Lexical
	matcher Matcher
}

func New(lexical *Lexical, matcher Matcher) *Engine {
	if matcher == nil {
		matcher = PassthroughMatcher{}
	}
	return &Engine{lexical: lexical, matcher: matcher}
}

func (engine *Engine) Search(
	ctx context.Context,
	query string,
	market domain.Market,
	language *domain.Language,
	filter domain.ApplicabilityFilter,
	limit int,
) ([]domain.SearchCandidate, error) {
	lexical, err := engine.lexical.Search(ctx, query, market, language, filter, limit)
	if err != nil {
		return nil, err
	}
	var multilingual []domain.SearchCandidate
	if language != nil {
		multilingual, err = engine.matcher.Match(ctx, query, *language)
		if err != nil {
			return nil, err
		}
	}
	return Merge(limit, lexical, multilingual), nil
}

// Merge sums candidate scores and applies deterministic ordering.
func Merge(limit int, groups ...[]domain.SearchCandidate) []domain.SearchCandidate {
	scores := make(map[string]float64)
	for _, group := range groups {
		for _, candidate := range group {
			scores[candidate.ItemID] += candidate.Score
		}
	}
	result := make([]domain.SearchCandidate, 0, len(scores))
	for itemID, score := range scores {
		result = append(result, domain.SearchCandidate{ItemID: itemID, Score: score})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return result[i].ItemID < result[j].ItemID
		}
		return result[i].Score > result[j].Score
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}
