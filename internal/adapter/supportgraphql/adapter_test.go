package supportgraphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	baseadapter "github.com/jviklun1/pp-volvo-cars/internal/adapter"
	"github.com/jviklun1/pp-volvo-cars/internal/domain"
)

func TestSearchMapsLiveCapturedResponse(t *testing.T) {
	fixture, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload graphqlRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.Variables["market"] != "uk" || payload.Variables["query"] != "charging" {
			t.Errorf("variables = %#v", payload.Variables)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(fixture)
	}))
	defer server.Close()

	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	adapter := newAdapter(newClient(server.URL, server.Client(), 0, 1), func() time.Time { return now })
	items, err := adapter.Search(context.Background(), "charging", baseadapter.SearchOptions{Market: "gb", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
	item := items[0]
	if item.Market != domain.Market("gb") || item.SourceID != "32baedb1923875bac0a801517b21e068" || item.Title != "Plug & Charge" {
		t.Fatalf("mapped item = %#v", item)
	}
	if item.SourceType != domain.SourceTypeSupportArticle || item.Language != domain.Language("en-GB") {
		t.Fatalf("mapped type/language = %q/%q", item.SourceType, item.Language)
	}
	if item.Body != "Note Even if Plug & Charge is included in the charging view, it may not be available in your country. You can find charging stations that support Plug & Charge through Google Maps in the centre display. Applicable models EX30 , model year 2026 and later" {
		t.Fatalf("body = %q", item.Body)
	}
	if len(item.Citations) != 1 || item.Citations[0].SourceURL != "volvo-support://document/32baedb1923875bac0a801517b21e068" {
		t.Fatalf("citations = %#v", item.Citations)
	}
	wantID := domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	if item.ID != wantID || item.ContentHash != domain.DeriveContentHash(item.Body) {
		t.Fatalf("identity/hash were not stable: %#v", item)
	}
	if len(item.Applicability) != 1 || item.Applicability[0].ModelSlug == nil || *item.Applicability[0].ModelSlug != domain.ModelSlug("ex30") {
		t.Fatalf("applicability = %#v", item.Applicability)
	}
}

func TestSearchQueriesUseFieldSpecificEnumTypes(t *testing.T) {
	if !strings.Contains(marketSearchQuery, "$include: [MarketSearchType!]!") {
		t.Fatalf("market search enum contract drifted: %s", marketSearchQuery)
	}
	if !strings.Contains(carSearchQuery, "$include: [CarSearchType!]!") {
		t.Fatalf("car search enum contract drifted: %s", carSearchQuery)
	}
}

func TestSyncReturnsMarketAndModelCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload graphqlRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(payload.Query, "markets {"):
			_, _ = writer.Write([]byte(`{"data":{"markets":[{"id":"se","caption":"Sweden"},{"id":"uk","caption":"United Kingdom"}]}}`))
		case strings.Contains(payload.Query, "carsByDisplayName"):
			if payload.Variables["market"] != "uk" {
				t.Errorf("variables = %#v", payload.Variables)
			}
			_, _ = writer.Write([]byte(`{"data":{"market":{"carsByDisplayName":[{"displayName":"XC60","modelSlug":"xc60"},{"displayName":"XC60 duplicate","modelSlug":"xc60"},{"displayName":"EX30","modelSlug":"ex30"}]}}}`))
		default:
			t.Errorf("unexpected query: %s", payload.Query)
			writer.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	adapter := newAdapter(newClient(server.URL, server.Client(), 0, 1), func() time.Time { return now })
	result, err := adapter.Sync(context.Background(), "gb", baseadapter.SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Markets) != 2 || result.Markets[0] != (domain.MarketRef{ID: "se", Caption: "Sweden"}) || result.Markets[1] != (domain.MarketRef{ID: "gb", Caption: "United Kingdom"}) {
		t.Fatalf("markets = %#v", result.Markets)
	}
	wantModels := []domain.ModelRef{
		{Market: "gb", Slug: "xc60", DisplayName: "XC60"},
		{Market: "gb", Slug: "ex30", DisplayName: "EX30"},
	}
	if len(result.Models) != len(wantModels) {
		t.Fatalf("models = %#v", result.Models)
	}
	for index := range wantModels {
		if result.Models[index] != wantModels[index] {
			t.Fatalf("models[%d] = %#v, want %#v", index, result.Models[index], wantModels[index])
		}
	}
}
