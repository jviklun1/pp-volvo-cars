package marketpages

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

func TestParseMarketFixtures(t *testing.T) {
	capturedAt := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, fixture, rawURL, market, language, title, excerpt string
		parse                                                   func(io.Reader, string, time.Time) (domain.KnowledgeItem, error)
	}{
		{
			name: "Sweden", fixture: "testdata/se.html", rawURL: "https://www.volvocars.com/se/support/topic/?utm_source=test#top",
			market: "se", language: "sv", title: "Ladda din EX30 hemma", excerpt: "Lär dig att ladda bilen",
			parse: ParseSE,
		},
		{
			name: "United Kingdom", fixture: "testdata/gb.html", rawURL: "https://www.volvocars.com/uk/cars/ex30-electric/?campaign=spring",
			market: "gb", language: "en", title: "EX30 charging at home", excerpt: "Charge safely at home",
			parse: ParseGB,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture, err := os.ReadFile(test.fixture)
			if err != nil {
				t.Fatal(err)
			}
			first, err := test.parse(strings.NewReader(string(fixture)), test.rawURL, capturedAt)
			if err != nil {
				t.Fatal(err)
			}
			second, err := test.parse(strings.NewReader(string(fixture)), test.rawURL, capturedAt)
			if err != nil {
				t.Fatal(err)
			}
			if first.ID != second.ID || first.ContentHash != second.ContentHash {
				t.Fatalf("re-parse identity changed: %#v != %#v", first, second)
			}
			if first.Market != domain.Market(test.market) || first.Language != domain.Language(test.language) || first.Title != test.title {
				t.Fatalf("mapped item = %#v", first)
			}
			if len(first.Citations) != 1 || !strings.Contains(first.Citations[0].Excerpt, test.excerpt) || strings.Contains(first.Citations[0].SourceURL, "?") {
				t.Fatalf("citation did not preserve clean original-language evidence: %#v", first.Citations)
			}
		})
	}
}

func TestNormalizeSourceID(t *testing.T) {
	got, err := NormalizeSourceID("https://www.volvocars.com/se/support/topic/?utm_source=test#top")
	if err != nil || got != "/se/support/topic" {
		t.Fatalf("NormalizeSourceID() = %q, %v", got, err)
	}
}
