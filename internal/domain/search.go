package domain

// SearchCandidate identifies a normalized item and its retrieval score.
type SearchCandidate struct {
	ItemID string  `json:"item_id"`
	Score  float64 `json:"score"`
}
