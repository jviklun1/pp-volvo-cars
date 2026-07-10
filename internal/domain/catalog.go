package domain

// MarketRef is a compact catalog entry for a canonical Volvo market.
type MarketRef struct {
	ID      Market `json:"id"`
	Caption string `json:"caption"`
}

// ModelRef is a compact market-scoped Volvo model catalog entry.
type ModelRef struct {
	Market      Market    `json:"market"`
	Slug        ModelSlug `json:"slug"`
	DisplayName string    `json:"display_name"`
}
