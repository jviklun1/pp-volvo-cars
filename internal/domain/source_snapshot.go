package domain

import "time"

// FreshnessTier controls the maximum age allowed before auto-refresh.
type FreshnessTier string

const (
	FreshnessTierStable   FreshnessTier = "stable"
	FreshnessTierVolatile FreshnessTier = "volatile"
)

// Valid reports whether the freshness tier is part of the v1 contract.
func (t FreshnessTier) Valid() bool {
	return t == FreshnessTierStable || t == FreshnessTierVolatile
}

// FetchStatus describes the last adapter fetch for a source slice.
type FetchStatus string

const (
	FetchStatusOK      FetchStatus = "ok"
	FetchStatusPartial FetchStatus = "partial"
	FetchStatusFailed  FetchStatus = "failed"
	FetchStatusStale   FetchStatus = "stale"
)

// SourceSnapshot records freshness and fetch state for one source and market.
type SourceSnapshot struct {
	SourceType    SourceType    `json:"source_type"`
	Market        Market        `json:"market"`
	VersionOrHash string        `json:"version_or_hash"`
	FetchStatus   FetchStatus   `json:"fetch_status"`
	FreshnessTier FreshnessTier `json:"freshness_tier"`
	LastFetchedAt time.Time     `json:"last_fetched_at"`
}
