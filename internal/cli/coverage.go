package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jviklun1/pp-volvo-cars/internal/adapter"
	"github.com/jviklun1/pp-volvo-cars/internal/domain"
	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
	"github.com/spf13/cobra"
)

const (
	outcomeEvidenceFound = "evidence_found"
	outcomeNoResults     = "no_results"
	outcomeSourceError   = "source_error"
)

const (
	statusComplete   = "complete"
	statusIncomplete = "incomplete"
)

const maxCoverageQuestions = 20

type coverageQuestionResult struct {
	Question  string                 `json:"question"`
	Outcome   string                 `json:"outcome"`
	Results   []domain.KnowledgeItem `json:"results"`
	Error     string                 `json:"error,omitempty"`
	ErrorCode int                    `json:"error_code,omitempty"`
}

type coverageReport struct {
	Topic     string                   `json:"topic"`
	Market    domain.Market            `json:"market"`
	Status    string                   `json:"status"`
	Questions []coverageQuestionResult `json:"questions"`
}

func newCoverageCommand(app App, options *globalOptions) *cobra.Command {
	coverage := &cobra.Command{Use: "coverage", Short: "Assess topic coverage"}
	var topicInput, marketInput, modelInput, powertrainInput, languageInput string
	var questions []string
	var year, limit int
	assess := &cobra.Command{
		Use:  "assess",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			topic := strings.TrimSpace(topicInput)
			if topic == "" {
				return exiterrors.WithCode(exiterrors.CodeInvalidArguments, errors.New("coverage topic is required"))
			}
			trimmed := make([]string, 0, len(questions))
			for _, question := range questions {
				value := strings.TrimSpace(question)
				if value == "" {
					return exiterrors.WithCode(exiterrors.CodeInvalidArguments, errors.New("at least one non-blank --question is required"))
				}
				trimmed = append(trimmed, value)
			}
			if len(trimmed) == 0 {
				return exiterrors.WithCode(exiterrors.CodeInvalidArguments, errors.New("at least one non-blank --question is required"))
			}
			if len(trimmed) > maxCoverageQuestions {
				return exiterrors.WithCode(exiterrors.CodeInvalidArguments, fmt.Errorf("too many questions (max %d per invocation)", maxCoverageQuestions))
			}
			market, err := normalizeMarket(marketInput)
			if err != nil {
				return err
			}
			questionSources := queryScopedAdapters(app)
			if len(questionSources) == 0 {
				return exiterrors.WithCode(exiterrors.CodeInvalidArguments, errors.New("no query-scoped (GraphQL) source adapters are configured"))
			}

			filter := domain.ApplicabilityFilter{}
			if modelInput != "" {
				value := domain.ModelSlug(modelInput)
				filter.ModelSlug = &value
			}
			if year != 0 {
				value := domain.ModelYear(year)
				filter.ModelYear = &value
			}
			if powertrainInput != "" {
				value := domain.Powertrain(powertrainInput)
				filter.Powertrain = &value
			}
			var language *domain.Language
			if languageInput != "" {
				value := domain.Language(languageInput)
				language = &value
			}
			searchOptions := adapter.SearchOptions{
				Market: market, Language: language, Applicability: filter, Limit: limit,
			}

			report := coverageReport{
				Topic: topic, Market: market, Status: statusComplete,
				Questions: make([]coverageQuestionResult, 0, len(trimmed)),
			}
			anyIncomplete := 0

			for _, question := range trimmed {
				items := make([]domain.KnowledgeItem, 0)
				var failure error
				searched := make([]domain.SourceType, 0, len(questionSources))
				for _, sourceType := range questionSources {
					values, err := app.Adapters[sourceType].Search(command.Context(), question, searchOptions)
					if err != nil {
						failure = err
						break
					}
					searched = append(searched, sourceType)
					items = append(items, values...)
				}

				result := coverageQuestionResult{Question: question, Results: items}
				switch {
				case failure != nil:
					result.Outcome = outcomeSourceError
					result.Results = make([]domain.KnowledgeItem, 0)
					result.Error = failure.Error()
					result.ErrorCode = int(exiterrors.CodeOf(failure))
					anyIncomplete++
				case len(items) == 0:
					result.Outcome = outcomeNoResults
				default:
					result.Outcome = outcomeEvidenceFound
					if err := persistCoverageEvidence(command.Context(), app, searched, market, items); err != nil {
						return err
					}
				}
				report.Questions = append(report.Questions, result)
			}

			if anyIncomplete > 0 {
				report.Status = statusIncomplete
			}

			if err := writeCoverageReport(app, options, report); err != nil {
				return err
			}

			if anyIncomplete > 0 {
				return exiterrors.WithCode(
					exiterrors.CodeInsufficientEvidence,
					fmt.Errorf("coverage assessment incomplete: %d of %d questions returned a source error", anyIncomplete, len(trimmed)),
				)
			}
			return nil
		},
	}
	assess.Flags().StringVar(&topicInput, "topic", "", "coverage topic")
	assess.Flags().StringArrayVar(&questions, "question", nil, "representative customer question (repeatable)")
	assess.Flags().StringVar(&marketInput, "market", "", "market identifier")
	assess.Flags().StringVar(&modelInput, "model", "", "model slug")
	assess.Flags().IntVar(&year, "year", 0, "model year")
	assess.Flags().StringVar(&powertrainInput, "powertrain", "", "powertrain")
	assess.Flags().StringVar(&languageInput, "lang", "", "BCP-47 source language")
	assess.Flags().IntVar(&limit, "limit", 20, "maximum results per question")
	coverage.AddCommand(assess)
	return coverage
}

func queryScopedAdapters(app App) []domain.SourceType {
	types := make([]domain.SourceType, 0, len(app.Adapters))
	for _, sourceType := range adapterTypes(app) {
		if app.Adapters[sourceType].CacheScope() == adapter.CacheScopeQuery {
			types = append(types, sourceType)
		}
	}
	return types
}

func persistCoverageEvidence(ctx context.Context, app App, searched []domain.SourceType, market domain.Market, items []domain.KnowledgeItem) error {
	for _, item := range items {
		if err := app.Store.UpsertItem(ctx, item); err != nil {
			return err
		}
	}
	for _, sourceType := range searched {
		snapshot, err := app.Adapters[sourceType].Status(ctx, market)
		if err != nil {
			return err
		}
		if err := app.Store.SaveSnapshot(ctx, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func writeCoverageReport(app App, options *globalOptions, report coverageReport) error {
	if options.JSON || options.Compact {
		return writeValue(app, options, report)
	}
	fmt.Fprintf(app.Out, "%s\t%s\t%s\n", report.Topic, report.Market, report.Status)
	for _, question := range report.Questions {
		fmt.Fprintf(app.Out, "%s\t%s\t%d\n", question.Question, question.Outcome, len(question.Results))
	}
	return nil
}
