package marketpages

import (
	"io"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

func ParseSE(reader io.Reader, pageURL string, capturedAt time.Time) (domain.KnowledgeItem, error) {
	return mapPage(reader, pageURL, domain.Market("se"), domain.Language("sv"), capturedAt)
}
