package domain

import "time"

// Citation preserves the original-language evidence and its provenance.
type Citation struct {
	SourceURL  string
	Excerpt    string
	Language   Language
	CapturedAt time.Time
}
