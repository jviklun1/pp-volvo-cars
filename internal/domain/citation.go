package domain

import "time"

// Citation preserves the original-language evidence and its provenance.
type Citation struct {
	SourceURL  string    `json:"source_url"`
	Excerpt    string    `json:"excerpt"`
	Language   Language  `json:"language"`
	CapturedAt time.Time `json:"captured_at"`
}
