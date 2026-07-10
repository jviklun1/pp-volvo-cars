package marketpages

import (
	"io"
	"time"

	"example.com/volvo/internal/domain"
)

func ParseSE(reader io.Reader, pageURL string, capturedAt time.Time) (domain.KnowledgeItem, error) {
	return mapPage(reader, pageURL, domain.Market("se"), domain.Language("sv"), capturedAt)
}
