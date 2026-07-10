package marketpages

import (
	"io"
	"time"
	"unicode/utf8"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

func mapPage(reader io.Reader, rawURL string, market domain.Market, language domain.Language, capturedAt time.Time) (domain.KnowledgeItem, error) {
	page, err := extract(reader, rawURL)
	if err != nil {
		return domain.KnowledgeItem{}, err
	}
	item := domain.KnowledgeItem{
		SourceType: domain.SourceTypeMarketPage,
		SourceID:   page.SourceID,
		Market:     market,
		Language:   language,
		Title:      page.Title,
		Body:       page.Body,
		CapturedAt: capturedAt.UTC(),
		Citations: []domain.Citation{{
			SourceURL: page.CanonicalURL, Excerpt: excerpt(page.Body, 280), Language: language, CapturedAt: capturedAt.UTC(),
		}},
	}
	item.ID = domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	item.ContentHash = domain.DeriveContentHash(item.Body)
	return item, nil
}

func excerpt(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "…"
}
