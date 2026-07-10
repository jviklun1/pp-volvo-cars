package cli

import (
	"fmt"

	"example.com/volvo/internal/adapter"
	"example.com/volvo/internal/domain"
	"github.com/spf13/cobra"
)

type searchResult struct {
	Score float64              `json:"score"`
	Item  domain.KnowledgeItem `json:"item"`
}

func newSearchCommand(app App, options *globalOptions) *cobra.Command {
	var marketInput, modelInput, powertrainInput, languageInput string
	var year, limit int
	command := &cobra.Command{
		Use:  "search <question>",
		Args: exactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			market, err := normalizeMarket(marketInput)
			if err != nil {
				return err
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

			if options.DataSource == "live" {
				var items []domain.KnowledgeItem
				for _, sourceType := range adapterTypes(app) {
					sourceAdapter := app.Adapters[sourceType]
					values, err := sourceAdapter.Search(command.Context(), args[0], searchOptions)
					if err != nil {
						return err
					}
					items = append(items, values...)
				}
				return writeValue(app, options, struct {
					Market  domain.Market          `json:"market"`
					Query   string                 `json:"query"`
					Results []domain.KnowledgeItem `json:"results"`
				}{Market: market, Query: args[0], Results: items})
			}
			if options.DataSource == "auto" {
				if err := refreshForSearch(command.Context(), app, args[0], searchOptions); err != nil {
					return err
				}
			}
			candidates, err := app.Search.Search(command.Context(), args[0], market, language, filter, limit)
			if err != nil {
				return err
			}
			results := make([]searchResult, 0, len(candidates))
			for _, candidate := range candidates {
				item, err := app.Store.GetItem(command.Context(), candidate.ItemID)
				if err != nil {
					return err
				}
				results = append(results, searchResult{Score: candidate.Score, Item: item})
			}
			if options.JSON || options.Compact {
				return writeValue(app, options, struct {
					Market  domain.Market  `json:"market"`
					Query   string         `json:"query"`
					Results []searchResult `json:"results"`
				}{Market: market, Query: args[0], Results: results})
			}
			for _, result := range results {
				fmt.Fprintf(app.Out, "%s\t%s\n", result.Item.ID, result.Item.Title)
			}
			return nil
		},
	}
	command.Flags().StringVar(&marketInput, "market", "", "market identifier")
	command.Flags().StringVar(&modelInput, "model", "", "model slug")
	command.Flags().IntVar(&year, "year", 0, "model year")
	command.Flags().StringVar(&powertrainInput, "powertrain", "", "powertrain")
	command.Flags().StringVar(&languageInput, "lang", "", "BCP-47 source language")
	command.Flags().IntVar(&limit, "limit", 20, "maximum results")
	return command
}
