package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jviklun1/pp-volvo-cars/internal/adapter"
	"github.com/jviklun1/pp-volvo-cars/internal/domain"
	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
	"github.com/jviklun1/pp-volvo-cars/internal/search"
	"github.com/jviklun1/pp-volvo-cars/internal/store"
)

var coverageNow = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

type queryOutcome struct {
	items []domain.KnowledgeItem
	err   error
}

type queryAdapter struct {
	now     time.Time
	byQuery map[string]queryOutcome
	calls   *[]string
}

func (queryAdapter) CacheScope() adapter.CacheScope { return adapter.CacheScopeQuery }

func (queryAdapter) Sync(context.Context, domain.Market, adapter.SyncOptions) (adapter.SyncResult, error) {
	return adapter.SyncResult{}, nil
}

func (source queryAdapter) Search(_ context.Context, query string, _ adapter.SearchOptions) ([]domain.KnowledgeItem, error) {
	if source.calls != nil {
		*source.calls = append(*source.calls, query)
	}
	outcome := source.byQuery[query]
	return outcome.items, outcome.err
}

func (queryAdapter) Get(context.Context, string) (domain.KnowledgeItem, error) {
	return domain.KnowledgeItem{}, nil
}

func (source queryAdapter) Status(_ context.Context, market domain.Market) (domain.SourceSnapshot, error) {
	return domain.SourceSnapshot{
		SourceType: domain.SourceTypeSupportArticle, Market: market, VersionOrHash: "v1",
		FetchStatus: domain.FetchStatusOK, FreshnessTier: domain.FreshnessTierStable, LastFetchedAt: source.now,
	}, nil
}

func coverageItem(sourceID, body string) domain.KnowledgeItem {
	item := domain.KnowledgeItem{
		SourceType: domain.SourceTypeSupportArticle, SourceID: sourceID, Market: "gb", Language: "en",
		Title: "Home charging", Body: body, CapturedAt: coverageNow,
	}
	item.ID = domain.DeriveID(item.SourceType, item.SourceID, item.Market, item.Language)
	item.ContentHash = domain.DeriveContentHash(item.Body)
	return item
}

func newCoverageApp(t *testing.T, output *bytes.Buffer, source adapter.Adapter) App {
	t.Helper()
	database, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "cli.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return App{
		Store:    database,
		Search:   search.New(search.NewLexical(database), nil),
		Adapters: map[domain.SourceType]adapter.Adapter{domain.SourceTypeSupportArticle: source},
		Out:      output,
		Now:      func() time.Time { return coverageNow },
	}
}

func TestCoverageAllEvidenceFoundPersistsAndCompletes(t *testing.T) {
	ctx := context.Background()
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{
		"Why is my car not charging?": {items: []domain.KnowledgeItem{coverageItem("doc-1", "Plug in the cable.")}},
	}}
	var output bytes.Buffer
	app := newCoverageApp(t, &output, source)
	root := NewRoot(app)
	root.SetArgs([]string{"coverage", "assess", "--topic", "Home charging problems",
		"--question", "Why is my car not charging?", "--market", "gb", "--json"})
	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var report coverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, output.String())
	}
	if report.Status != statusComplete || len(report.Questions) != 1 || report.Questions[0].Outcome != outcomeEvidenceFound {
		t.Fatalf("unexpected report: %#v", report)
	}
	stored, err := app.Store.GetItem(ctx, coverageItem("doc-1", "Plug in the cable.").ID)
	if err != nil {
		t.Fatalf("evidence not persisted: %v", err)
	}
	if stored.SourceID != "doc-1" {
		t.Fatalf("unexpected persisted item: %#v", stored)
	}
	snapshots, err := app.Store.ListSnapshots(ctx, "gb")
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("snapshot not persisted: %v (%#v)", err, snapshots)
	}
}

func TestCoverageAllNoResultsCompletesWithoutPersistence(t *testing.T) {
	ctx := context.Background()
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{
		"Why did charging stop?": {items: nil},
	}}
	var output bytes.Buffer
	app := newCoverageApp(t, &output, source)
	root := NewRoot(app)
	root.SetArgs([]string{"coverage", "assess", "--topic", "Home charging problems",
		"--question", "Why did charging stop?", "--market", "gb", "--json"})
	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var report coverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, output.String())
	}
	if report.Status != statusComplete || report.Questions[0].Outcome != outcomeNoResults || len(report.Questions[0].Results) != 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if !strings.Contains(output.String(), `"results": []`) {
		t.Fatalf("empty results must serialize as []: %s", output.String())
	}
	snapshots, err := app.Store.ListSnapshots(ctx, "gb")
	if err != nil || len(snapshots) != 0 {
		t.Fatalf("no snapshot expected on no_results: %v (%#v)", err, snapshots)
	}
}

func TestCoverageSourceErrorContinuesAndExitsFive(t *testing.T) {
	ctx := context.Background()
	var calls []string
	source := queryAdapter{now: coverageNow, calls: &calls, byQuery: map[string]queryOutcome{
		"first":  {err: errors.New("support document was not found")},
		"second": {items: []domain.KnowledgeItem{coverageItem("doc-2", "Later question still runs.")}},
	}}
	var output bytes.Buffer
	app := newCoverageApp(t, &output, source)
	root := NewRoot(app)
	root.SetArgs([]string{"coverage", "assess", "--topic", "t",
		"--question", "first", "--question", "second", "--market", "gb", "--json"})
	err := root.ExecuteContext(ctx)
	if exiterrors.CodeOf(err) != exiterrors.CodeInsufficientEvidence {
		t.Fatalf("exit code = %d, want 5; err = %v", exiterrors.CodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "1 of 2 questions returned a source error") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if len(calls) != 2 || calls[1] != "second" {
		t.Fatalf("evaluation did not continue past the source error: %#v", calls)
	}
	var report coverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("report must still be written; invalid JSON: %v\n%s", err, output.String())
	}
	if report.Status != statusIncomplete {
		t.Fatalf("status = %q, want incomplete", report.Status)
	}
	if report.Questions[0].Outcome != outcomeSourceError || report.Questions[0].ErrorCode != int(exiterrors.CodeInternal) {
		t.Fatalf("plain error must classify as CodeInternal (1): %#v", report.Questions[0])
	}
	if report.Questions[1].Outcome != outcomeEvidenceFound {
		t.Fatalf("second question should still be evaluated: %#v", report.Questions[1])
	}
}

func TestCoverageRateLimitErrorClassifiesAsSeven(t *testing.T) {
	ctx := context.Background()
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{
		"q": {err: fmt.Errorf("support rate limited: %w", exiterrors.ErrRateLimited)},
	}}
	var output bytes.Buffer
	app := newCoverageApp(t, &output, source)
	root := NewRoot(app)
	root.SetArgs([]string{"coverage", "assess", "--topic", "t", "--question", "q", "--market", "gb", "--json"})
	_ = root.ExecuteContext(ctx)
	var report coverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.Questions[0].ErrorCode != int(exiterrors.CodeRateLimited) {
		t.Fatalf("rate-limit error must classify as 7: %#v", report.Questions[0])
	}
}

func TestCoverageTooManyQuestions(t *testing.T) {
	args := []string{"coverage", "assess", "--topic", "t", "--market", "gb"}
	for i := 0; i < 21; i++ {
		args = append(args, "--question", fmt.Sprintf("q%d", i))
	}
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{}}
	root := NewRoot(newCoverageApp(t, &bytes.Buffer{}, source))
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments || !strings.Contains(err.Error(), "too many questions") {
		t.Fatalf("error = %v, code = %d", err, exiterrors.CodeOf(err))
	}
}

func TestCoverageBlankTopic(t *testing.T) {
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{}}
	root := NewRoot(newCoverageApp(t, &bytes.Buffer{}, source))
	root.SetArgs([]string{"coverage", "assess", "--topic", "   ", "--question", "q", "--market", "gb"})
	err := root.ExecuteContext(context.Background())
	if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments || !strings.Contains(err.Error(), "coverage topic is required") {
		t.Fatalf("error = %v, code = %d", err, exiterrors.CodeOf(err))
	}
}

func TestCoverageBlankQuestion(t *testing.T) {
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{}}
	root := NewRoot(newCoverageApp(t, &bytes.Buffer{}, source))
	root.SetArgs([]string{"coverage", "assess", "--topic", "t", "--question", "  ", "--market", "gb"})
	err := root.ExecuteContext(context.Background())
	if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments || !strings.Contains(err.Error(), "at least one non-blank --question is required") {
		t.Fatalf("error = %v, code = %d", err, exiterrors.CodeOf(err))
	}
}

func TestCoverageQuestionWithCommaIsSingleQuestion(t *testing.T) {
	ctx := context.Background()
	question := "Why did charging stop, and how do I restart it?"
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{question: {items: nil}}}
	var output bytes.Buffer
	root := NewRoot(newCoverageApp(t, &output, source))
	root.SetArgs([]string{"coverage", "assess", "--topic", "t", "--question", question, "--market", "gb", "--json"})
	if err := root.ExecuteContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var report coverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(report.Questions) != 1 || report.Questions[0].Question != question {
		t.Fatalf("comma question was split (StringSliceVar footgun): %#v", report.Questions)
	}
}

func TestCoverageNoQueryScopedAdapter(t *testing.T) {
	database, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "cli.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	app := App{
		Store:    database,
		Adapters: map[domain.SourceType]adapter.Adapter{domain.SourceTypeSupportArticle: catalogAdapter{}},
		Out:      &bytes.Buffer{},
		Now:      func() time.Time { return coverageNow },
	}
	root := NewRoot(app)
	root.SetArgs([]string{"coverage", "assess", "--topic", "t", "--question", "q", "--market", "gb"})
	err = root.ExecuteContext(context.Background())
	if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments || !strings.Contains(err.Error(), "no query-scoped (GraphQL) source adapters") {
		t.Fatalf("error = %v, code = %d", err, exiterrors.CodeOf(err))
	}
}

func TestCoverageJSONGolden(t *testing.T) {
	ctx := context.Background()
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{
		"Why is my car not charging?": {items: []domain.KnowledgeItem{coverageItem("doc-1", "Plug in the cable.")}},
		"Why did charging stop?":      {items: nil},
		"Why is the app broken?":      {err: errors.New("support document was not found")},
	}}
	var output bytes.Buffer
	root := NewRoot(newCoverageApp(t, &output, source))
	root.SetArgs([]string{"coverage", "assess", "--topic", "Home charging problems",
		"--question", "Why is my car not charging?",
		"--question", "Why did charging stop?",
		"--question", "Why is the app broken?",
		"--market", "gb", "--json"})
	_ = root.ExecuteContext(ctx)
	want, err := os.ReadFile("testdata/coverage.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != string(want) {
		t.Fatalf("coverage JSON:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestCoverageRejectsExplicitDataSource(t *testing.T) {
	for _, value := range []string{"local", "live"} {
		t.Run(value, func(t *testing.T) {
			var calls []string
			source := queryAdapter{now: coverageNow, calls: &calls, byQuery: map[string]queryOutcome{
				"q": {items: []domain.KnowledgeItem{coverageItem("doc-1", "Plug in the cable.")}},
			}}
			var output bytes.Buffer
			root := NewRoot(newCoverageApp(t, &output, source))
			root.SetArgs([]string{"coverage", "assess", "--topic", "t", "--question", "q",
				"--market", "gb", "--data-source", value})
			err := root.ExecuteContext(context.Background())
			if exiterrors.CodeOf(err) != exiterrors.CodeInvalidArguments ||
				!strings.Contains(err.Error(), "does not accept --data-source") {
				t.Fatalf("error = %v, code = %d", err, exiterrors.CodeOf(err))
			}
			if len(calls) != 0 {
				t.Fatalf("no adapter search may run when --data-source is rejected: %#v", calls)
			}
		})
	}
}

func TestCoverageWritesReportBeforePersistenceError(t *testing.T) {
	ctx := context.Background()
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{
		"Why is my car not charging?": {items: []domain.KnowledgeItem{coverageItem("doc-1", "Plug in the cable.")}},
	}}
	var output bytes.Buffer
	app := newCoverageApp(t, &output, source)
	if err := app.Store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	root := NewRoot(app)
	root.SetArgs([]string{"coverage", "assess", "--topic", "Home charging problems",
		"--question", "Why is my car not charging?", "--market", "gb", "--json"})
	err := root.ExecuteContext(ctx)
	if exiterrors.CodeOf(err) != exiterrors.CodeInternal {
		t.Fatalf("persistence failure should classify as internal (1): code = %d, err = %v", exiterrors.CodeOf(err), err)
	}
	var report coverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("report must be written before the persistence error; invalid JSON: %v\n%s", err, output.String())
	}
	if report.Status != statusComplete || len(report.Questions) != 1 || report.Questions[0].Outcome != outcomeEvidenceFound {
		t.Fatalf("retrieved evidence must be preserved in the report: %#v", report)
	}
	if len(report.Questions[0].Results) != 1 {
		t.Fatalf("evidence results must survive the persistence failure: %#v", report.Questions[0])
	}
}

func TestCoveragePersistenceErrorWinsOverSourceError(t *testing.T) {
	ctx := context.Background()
	source := queryAdapter{now: coverageNow, byQuery: map[string]queryOutcome{
		"good": {items: []domain.KnowledgeItem{coverageItem("doc-1", "Plug in the cable.")}},
		"bad":  {err: errors.New("support document was not found")},
	}}
	var output bytes.Buffer
	app := newCoverageApp(t, &output, source)
	if err := app.Store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	root := NewRoot(app)
	root.SetArgs([]string{"coverage", "assess", "--topic", "t",
		"--question", "good", "--question", "bad", "--market", "gb", "--json"})
	err := root.ExecuteContext(ctx)
	if exiterrors.CodeOf(err) != exiterrors.CodeInternal {
		t.Fatalf("persistence failure must win over source-error exit 5: code = %d, err = %v", exiterrors.CodeOf(err), err)
	}
	var report coverageReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("report must still be written: %v\n%s", err, output.String())
	}
	if report.Status != statusIncomplete {
		t.Fatalf("status = %q, want incomplete (source diagnostics retained)", report.Status)
	}
	if report.Questions[1].Outcome != outcomeSourceError {
		t.Fatalf("per-question source diagnostic must remain in the report: %#v", report.Questions[1])
	}
}
