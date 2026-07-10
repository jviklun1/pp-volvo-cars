package cli

import (
	"fmt"

	"github.com/jviklun1/pp-volvo-cars/internal/domain"
	"github.com/spf13/cobra"
)

func newSourcesCommand(app App, options *globalOptions) *cobra.Command {
	var marketInput string
	sources := &cobra.Command{Use: "sources", Short: "Inspect source freshness"}
	status := &cobra.Command{
		Use:  "status",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			market, err := normalizeMarket(marketInput)
			if err != nil {
				return err
			}
			values, err := app.Store.ListSnapshots(command.Context(), market)
			if err != nil {
				return err
			}
			if options.JSON || options.Compact {
				return writeValue(app, options, struct {
					Market  domain.Market           `json:"market"`
					Sources []domain.SourceSnapshot `json:"sources"`
				}{Market: market, Sources: values})
			}
			for _, snapshot := range values {
				fmt.Fprintf(app.Out, "%s\t%s\t%s\n", snapshot.SourceType, snapshot.FetchStatus, snapshot.LastFetchedAt.Format("2006-01-02T15:04:05Z07:00"))
			}
			return nil
		},
	}
	status.Flags().StringVar(&marketInput, "market", "", "market identifier")
	sources.AddCommand(status)
	return sources
}
