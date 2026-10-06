package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jviklun1/pp-volvo-cars/internal/adapter/supportgraphql"
	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
)

type supportHTTPTransport struct {
	handler http.HandlerFunc
}

func (transport supportHTTPTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response := httptest.NewRecorder()
	transport.handler.ServeHTTP(response, request)
	return response.Result(), nil
}

// Only the upstream HTTP boundary is replaced; the CLI, adapter and store run normally.
func runSupportCoverage(t *testing.T, handler http.HandlerFunc) (coverageReport, error) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = supportHTTPTransport{handler: handler}
	t.Cleanup(func() { http.DefaultTransport = previous })
	var output bytes.Buffer
	root := NewRoot(newCoverageApp(t, &output, supportgraphql.New()))
	root.SetArgs([]string{"coverage", "assess", "--topic", "Flat tyre",
		"--question", "How do I repair a flat tyre?", "--market", "se", "--json"})
	err := root.ExecuteContext(context.Background())
	var report coverageReport
	if decodeErr := json.Unmarshal(output.Bytes(), &report); decodeErr != nil {
		t.Fatalf("invalid JSON: %v\n%s", decodeErr, output.String())
	}
	if len(report.Questions) != 1 {
		t.Fatalf("questions = %#v, want one", report.Questions)
	}
	return report, err
}

func TestCoverageMissingDocumentIsNoResults(t *testing.T) {
	report, err := runSupportCoverage(t, func(writer http.ResponseWriter, request *http.Request) {
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(body), "query Search(") {
			_, _ = writer.Write([]byte(`{"data":{"market":{"search":{"results":[{"document":{"documentId":"missing","language":"en-GB"}}]}}}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"data":{"market":{"document":null}}}`))
	})
	if err != nil {
		t.Fatalf("missing document must exit 0: %v", err)
	}
	question := report.Questions[0]
	if report.Status != "complete" || question.Outcome != "no_results" || len(question.Results) != 0 || question.Error != "" {
		t.Fatalf("missing document must be a completed gap: %#v", report)
	}
}

func TestCoverageUpstreamHTTP503RemainsSourceError(t *testing.T) {
	report, err := runSupportCoverage(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte("upstream unavailable"))
	})
	if exiterrors.CodeOf(err) != exiterrors.CodeInsufficientEvidence {
		t.Fatalf("HTTP failure must exit 5: %v", err)
	}
	question := report.Questions[0]
	if report.Status != "incomplete" || question.Outcome != "source_error" || !strings.Contains(question.Error, "HTTP 503: upstream unavailable") {
		t.Fatalf("HTTP failure must retain its diagnostic: %#v", report)
	}
}

func TestCoverageDefaultLanguageDocumentHasOfficialCitation(t *testing.T) {
	report, err := runSupportCoverage(t, func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if decodeErr := json.NewDecoder(request.Body).Decode(&payload); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if strings.Contains(payload.Query, "query Search(") {
			_, _ = writer.Write([]byte(`{"data":{"market":{"search":{"results":[{"document":{"documentId":"flat-tyre","language":"en-GB"}}]}}}}`))
			return
		}
		if languages, _ := payload.Variables["languages"].([]any); len(languages) > 0 {
			_, _ = writer.Write([]byte(`{"data":{"market":{"document":null}}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"data":{"market":{"document":{"documentId":"flat-tyre","language":"sv-SE","documentType":"SUPPORT_CONTENT","stringContent":{"title":"Laga punktering","description":"Använd däckreparationssatsen."}}}}}`))
	})
	if err != nil {
		t.Fatalf("default-language page must exit 0: %v", err)
	}
	question := report.Questions[0]
	if report.Status != "complete" || question.Outcome != "evidence_found" || len(question.Results) != 1 {
		t.Fatalf("default-language page must be evidence: %#v", report)
	}
	item := question.Results[0]
	if item.Language != "sv-SE" || item.Title != "Laga punktering" || len(item.Citations) != 1 || item.Citations[0].SourceURL != "https://www.volvocars.com/se/support/topic/flat-tyre/" {
		t.Fatalf("default-language evidence must retain its public link and language: %#v", item)
	}
}
