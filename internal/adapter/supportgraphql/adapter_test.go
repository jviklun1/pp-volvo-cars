package supportgraphql

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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
	var captured struct {
		Data searchData `json:"data"`
	}
	if err := json.Unmarshal(fixture, &captured); err != nil {
		t.Fatal(err)
	}
	documentFixture, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"market": map[string]any{
				"document": captured.Data.Market.Search.Results[0].Document,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload graphqlRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		if strings.Contains(payload.Query, "query Document") {
			if payload.Variables["market"] != "uk" || payload.Variables["documentID"] != "32baedb1923875bac0a801517b21e068" {
				t.Errorf("document variables = %#v", payload.Variables)
			}
			_, _ = writer.Write(documentFixture)
			return
		}
		if payload.Variables["market"] != "uk" || payload.Variables["query"] != "charging" {
			t.Errorf("search variables = %#v", payload.Variables)
		}
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
	if len(item.Citations) != 1 || item.Citations[0].SourceURL != "https://www.volvocars.com/uk/support/topic/32baedb1923875bac0a801517b21e068/" {
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
	if strings.Contains(marketSearchQuery, "jsonContent") || strings.Contains(carSearchQuery, "jsonContent") {
		t.Fatal("search queries must not request nested jsonContent")
	}
	if !strings.Contains(documentQuery, "jsonContent { body jsonContentSchemaVersion }") {
		t.Fatalf("full document query lost jsonContent: %s", documentQuery)
	}
}

func TestSearchDiscoversThenHydratesUniqueDocuments(t *testing.T) {
	documents := map[string]document{
		"doc-b": {
			DocumentID: "doc-b", Language: "en-GB", DocumentType: "SUPPORT_ARTICLE",
			StringContent: stringContent{Title: "Document B", Description: "Description B"},
			JSONContent:   documentJSON{Body: json.RawMessage(`{"type":"segment","children":["Full body B"]}`)},
		},
		"doc-a": {
			DocumentID: "doc-a", Language: "en-GB", DocumentType: "USER_MANUAL",
			StringContent: stringContent{Title: "Document A", Description: "Description A"},
			JSONContent:   documentJSON{Body: json.RawMessage(`{"type":"segment","children":["Full body A"]}`)},
		},
	}
	searchFixture := []byte(`{
      "data": {"market": {"search": {"results": [
        {"matchingParagraph":["first B"],"document":{"documentId":"doc-b","language":"en-GB","documentType":"SUPPORT_ARTICLE","stringContent":{"title":"Document B","description":"Description B"},"applicableTo":[]}},
        {"matchingParagraph":["A match"],"document":{"documentId":"doc-a","language":"en-GB","documentType":"USER_MANUAL","stringContent":{"title":"Document A","description":"Description A"},"applicableTo":[]}},
        {"matchingParagraph":["second B"],"document":{"documentId":"doc-b","language":"en-GB","documentType":"SUPPORT_ARTICLE","stringContent":{"title":"Document B","description":"Description B"},"applicableTo":[]}}
      ]},"carByModelSlug":null}}
    }`)

	var mu sync.Mutex
	hydrations := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload graphqlRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if strings.Contains(payload.Query, "query Search(") {
			if strings.Contains(payload.Query, "jsonContent") {
				t.Error("discovery query requested jsonContent")
			}
			if payload.Variables["market"] != "uk" || payload.Variables["maxResults"] != float64(20) {
				t.Errorf("search variables = %#v", payload.Variables)
			}
			include, ok := payload.Variables["include"].([]any)
			if !ok || len(include) != len(searchTypes) {
				t.Errorf("search include = %#v", payload.Variables["include"])
			}
			_, _ = writer.Write(searchFixture)
			return
		}
		if !strings.Contains(payload.Query, "query Document") || !strings.Contains(payload.Query, "jsonContent") {
			t.Errorf("unexpected hydration query: %s", payload.Query)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		documentID, _ := payload.Variables["documentID"].(string)
		languages, ok := payload.Variables["languages"].([]any)
		if payload.Variables["market"] != "uk" || !ok || len(languages) != 1 || languages[0] != "en-GB" {
			t.Errorf("document variables = %#v", payload.Variables)
		}
		value, ok := documents[documentID]
		if !ok {
			t.Errorf("unknown document %q", documentID)
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		mu.Lock()
		hydrations[documentID]++
		mu.Unlock()
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"data": map[string]any{"market": map[string]any{"document": value}},
		})
	}))
	defer server.Close()

	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	adapter := newAdapter(newClient(server.URL, server.Client(), 0, 1), func() time.Time { return now })
	items, err := adapter.Search(context.Background(), "charging", baseadapter.SearchOptions{Market: "gb"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %#v", items)
	}
	wantOrder := []string{"doc-b", "doc-a", "doc-b"}
	wantBodies := []string{"Full body B", "Full body A", "Full body B"}
	wantExcerpts := []string{"first B", "A match", "second B"}
	for index := range items {
		if items[index].SourceID != wantOrder[index] || items[index].Body != wantBodies[index] {
			t.Errorf("items[%d] = (%q, %q), want (%q, %q)", index, items[index].SourceID, items[index].Body, wantOrder[index], wantBodies[index])
		}
		if len(items[index].Citations) != 1 || items[index].Citations[0].Excerpt != wantExcerpts[index] {
			t.Errorf("items[%d].Citations = %#v, want excerpt %q", index, items[index].Citations, wantExcerpts[index])
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if hydrations["doc-a"] != 1 || hydrations["doc-b"] != 1 || len(hydrations) != 2 {
		t.Fatalf("hydrations = %#v, want one per unique document/language", hydrations)
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

func TestPublicTopicCitationsPreserveMarketAndDocumentIdentity(t *testing.T) {
	for _, tc := range []struct{ market, language, locale string }{
		{"gb", "en-GB", "uk"}, {"de", "de-DE", "de"},
		{"be", "en-GB", "nl-be"}, {"be", "fr-FR", "fr-be"},
		{"ca", "en-US", "en-ca"}, {"ca", "fr-CA", "fr-ca"},
		{"ch", "de-DE", "de-ch"}, {"ch", "it-IT", "it-ch"},
	} {
		t.Run(tc.market+tc.language, func(t *testing.T) {
			doc := document{DocumentID: "abc-123", FullDocumentID: "parent_abc-123", DocumentType: "LATEST_INFO", Language: tc.language}
			want := "https://www.volvocars.com/" + tc.locale + "/support/topic/abc-123/"
			if got := supportDocumentURL(doc, domain.Market(tc.market)); got != want {
				t.Fatalf("citation = %q, want %q", got, want)
			}
		})
	}
	for _, kind := range []string{"USER_MANUAL", "QUALITY_BULLETIN", "UNKNOWN"} {
		doc := document{DocumentID: "abc-123", DocumentType: kind}
		if got := supportDocumentURL(doc, "gb"); got != "volvo-support://document/abc-123" {
			t.Fatalf("%s invented a public route: %s", kind, got)
		}
	}
	for _, id := range []string{"", "../other", "id?redirect=elsewhere", "id/child"} {
		doc := document{DocumentID: id, DocumentType: "LATEST_INFO"}
		if got := supportDocumentURL(doc, "gb"); strings.HasPrefix(got, "https://") {
			t.Fatalf("invalid document ID generated a public URL: %s", got)
		}
	}
}
