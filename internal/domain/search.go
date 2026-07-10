package domain

// SearchCandidate identifies a normalized item and its retrieval score.
type SearchCandidate struct {
	ItemID string
	Score  float64
}
