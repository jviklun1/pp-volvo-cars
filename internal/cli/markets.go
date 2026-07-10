package cli

import (
	"fmt"

	"example.com/volvo/internal/store"
	"github.com/spf13/cobra"
)

func newMarketsCommand(app App, options *globalOptions) *cobra.Command {
	markets := &cobra.Command{Use: "markets", Short: "List available markets"}
	markets.AddCommand(&cobra.Command{
		Use:  "list",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			values, err := app.Store.ListMarkets(command.Context())
			if err != nil {
				return err
			}
			if options.JSON || options.Compact {
				return writeValue(app, options, struct {
					Markets []store.MarketRecord `json:"markets"`
				}{Markets: values})
			}
			for _, market := range values {
				fmt.Fprintf(app.Out, "%s\t%s\n", market.ID, market.Caption)
			}
			return nil
		},
	})
	return markets
}

func newModelsCommand(app App, options *globalOptions) *cobra.Command {
	var marketInput string
	models := &cobra.Command{Use: "models", Short: "List market models"}
	list := &cobra.Command{
		Use:  "list",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			market, err := normalizeMarket(marketInput)
			if err != nil {
				return err
			}
			values, err := app.Store.ListModels(command.Context(), market)
			if err != nil {
				return err
			}
			if options.JSON || options.Compact {
				return writeValue(app, options, struct {
					Market string              `json:"market"`
					Models []store.ModelRecord `json:"models"`
				}{Market: string(market), Models: values})
			}
			for _, model := range values {
				fmt.Fprintf(app.Out, "%s\t%s\n", model.Slug, model.DisplayName)
			}
			return nil
		},
	}
	list.Flags().StringVar(&marketInput, "market", "", "market identifier")
	models.AddCommand(list)
	return models
}
