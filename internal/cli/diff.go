package cli

import (
	"fmt"
	"time"

	"example.com/volvo/internal/domain"
	exiterrors "example.com/volvo/internal/errors"
	"example.com/volvo/internal/store"
	"github.com/spf13/cobra"
)

func newDiffCommand(app App, options *globalOptions) *cobra.Command {
	var marketInput, sourceInput string
	var since time.Duration
	command := &cobra.Command{
		Use:  "diff",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			market, err := normalizeMarket(marketInput)
			if err != nil {
				return err
			}
			var sourceType *domain.SourceType
			if sourceInput != "" {
				value := domain.SourceType(sourceInput)
				if !value.Valid() {
					return exiterrors.WithCode(exiterrors.CodeInvalidArguments, fmt.Errorf("invalid source type %q", sourceInput))
				}
				sourceType = &value
			}
			values, err := app.Store.ListContentDiffs(command.Context(), market, sourceType, app.Now().Add(-since))
			if err != nil {
				return err
			}
			if options.JSON || options.Compact {
				return writeValue(app, options, struct {
					Market domain.Market       `json:"market"`
					Diffs  []store.ContentDiff `json:"diffs"`
				}{Market: market, Diffs: values})
			}
			for _, diff := range values {
				fmt.Fprintf(app.Out, "%s\t%s\n", diff.ItemID, diff.DetectedAt.Format(time.RFC3339))
			}
			return nil
		},
	}
	command.Flags().StringVar(&marketInput, "market", "", "market identifier")
	command.Flags().StringVar(&sourceInput, "source", "", "source type")
	command.Flags().DurationVar(&since, "since", 30*24*time.Hour, "lookback duration")
	return command
}
