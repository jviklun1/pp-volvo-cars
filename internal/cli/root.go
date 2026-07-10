// Package cli defines the agent-facing Volvo knowledge command surface.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"example.com/volvo/internal/adapter"
	"example.com/volvo/internal/domain"
	exiterrors "example.com/volvo/internal/errors"
	"example.com/volvo/internal/search"
	"example.com/volvo/internal/store"
	"github.com/spf13/cobra"
)

type App struct {
	Store    *store.Store
	Search   *search.Engine
	Adapters map[domain.SourceType]adapter.Adapter
	Out      io.Writer
	ErrOut   io.Writer
	Now      func() time.Time
}

type globalOptions struct {
	JSON       bool
	Compact    bool
	DataSource string
}

func NewRoot(app App) *cobra.Command {
	if app.Now == nil {
		app.Now = time.Now
	}
	if app.Out == nil {
		app.Out = io.Discard
	}
	if app.ErrOut == nil {
		app.ErrOut = io.Discard
	}
	if app.Adapters == nil {
		app.Adapters = map[domain.SourceType]adapter.Adapter{}
	}
	options := &globalOptions{DataSource: "auto"}
	root := &cobra.Command{
		Use:           "volvo",
		Short:         "Retrieve market-scoped evidence from official Volvo sources",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			switch options.DataSource {
			case "auto", "local", "live":
				return nil
			default:
				return exiterrors.WithCode(exiterrors.CodeInvalidArguments, fmt.Errorf("invalid data source %q", options.DataSource))
			}
		},
	}
	root.SetOut(app.Out)
	root.SetErr(app.ErrOut)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return exiterrors.WithCode(exiterrors.CodeInvalidArguments, err)
	})
	root.PersistentFlags().BoolVar(&options.JSON, "json", false, "emit stable JSON")
	root.PersistentFlags().BoolVar(&options.Compact, "compact", false, "emit compact JSON")
	root.PersistentFlags().StringVar(&options.DataSource, "data-source", "auto", "data source: auto, local, or live")

	root.AddCommand(newMarketsCommand(app, options))
	root.AddCommand(newModelsCommand(app, options))
	root.AddCommand(newGetCommand(app, options))
	root.AddCommand(newSourcesCommand(app, options))
	root.AddCommand(newSearchCommand(app, options))
	root.AddCommand(newDiffCommand(app, options))
	root.AddCommand(newSyncCommand(app, options))
	return root
}

func normalizeMarket(value string) (domain.Market, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "uk" {
		value = "gb"
	}
	if value == "" {
		return "", exiterrors.WithCode(exiterrors.CodeInvalidArguments, errors.New("market is required"))
	}
	return domain.Market(value), nil
}

func writeValue(app App, options *globalOptions, value any) error {
	encoder := json.NewEncoder(app.Out)
	if !options.Compact {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(value)
}

func noArgs(command *cobra.Command, args []string) error {
	if err := cobra.NoArgs(command, args); err != nil {
		return exiterrors.WithCode(exiterrors.CodeInvalidArguments, err)
	}
	return nil
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(count)(command, args); err != nil {
			return exiterrors.WithCode(exiterrors.CodeInvalidArguments, err)
		}
		return nil
	}
}

func adapterTypes(app App) []domain.SourceType {
	types := make([]domain.SourceType, 0, len(app.Adapters))
	for sourceType := range app.Adapters {
		types = append(types, sourceType)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}

func refresh(ctx context.Context, app App, market domain.Market, force bool) error {
	for _, sourceType := range adapterTypes(app) {
		sourceAdapter := app.Adapters[sourceType]
		if !force {
			stale, _, err := app.Store.IsStale(ctx, sourceType, market, app.Now().UTC())
			if err != nil {
				return err
			}
			if !stale {
				continue
			}
		}
		result, err := sourceAdapter.Sync(ctx, market, adapter.SyncOptions{})
		if err != nil {
			return err
		}
		for _, item := range result.Items {
			if err := app.Store.UpsertItem(ctx, item); err != nil {
				return err
			}
		}
		if err := app.Store.SaveSnapshot(ctx, result.Snapshot); err != nil {
			return err
		}
	}
	return nil
}
