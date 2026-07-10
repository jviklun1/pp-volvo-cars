package cli

import (
	"errors"

	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
	"github.com/spf13/cobra"
)

func newSyncCommand(app App, options *globalOptions) *cobra.Command {
	var marketInput string
	var dryRun bool
	command := &cobra.Command{
		Use:  "sync",
		Args: noArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			market, err := normalizeMarket(marketInput)
			if err != nil {
				return err
			}
			if len(app.Adapters) == 0 {
				return exiterrors.WithCode(exiterrors.CodeInvalidArguments, errors.New("no source adapters are configured"))
			}
			if dryRun {
				return writeValue(app, options, map[string]any{"market": market, "dry_run": true})
			}
			if err := refresh(command.Context(), app, market, true); err != nil {
				return err
			}
			return writeValue(app, options, map[string]any{"market": market, "status": "ok"})
		},
	}
	command.Flags().StringVar(&marketInput, "market", "", "market identifier")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be refreshed")
	return command
}
