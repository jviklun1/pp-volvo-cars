package cli

import (
	"database/sql"
	"errors"
	"fmt"

	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
	"github.com/spf13/cobra"
)

func newGetCommand(app App, options *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:  "get <item-id>",
		Args: exactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if options.DataSource == "live" {
				return exiterrors.WithCode(exiterrors.CodeInvalidArguments,
					errors.New("get resolves stored IDs; run sync or use search --data-source live"))
			}
			item, err := app.Store.GetItem(command.Context(), args[0])
			if errors.Is(err, sql.ErrNoRows) {
				return exiterrors.WithCode(exiterrors.CodeNotFound, fmt.Errorf("item %q not found", args[0]))
			}
			if err != nil {
				return err
			}
			if options.JSON || options.Compact {
				return writeValue(app, options, item)
			}
			fmt.Fprintf(app.Out, "%s\n\n%s\n", item.Title, item.Body)
			return nil
		},
	}
}
