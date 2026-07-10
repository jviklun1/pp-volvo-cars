package main

import (
	"context"
	"fmt"
	"os"

	"example.com/volvo/internal/adapter"
	"example.com/volvo/internal/adapter/supportgraphql"
	"example.com/volvo/internal/cli"
	"example.com/volvo/internal/domain"
	exiterrors "example.com/volvo/internal/errors"
	"example.com/volvo/internal/search"
	"example.com/volvo/internal/store"
)

func main() {
	path := os.Getenv("VOLVO_DB")
	if path == "" {
		path = "volvo.sqlite"
	}
	database, err := store.Open(context.Background(), path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(exiterrors.CodeInternal))
	}
	defer database.Close()
	support := supportgraphql.New()

	root := cli.NewRoot(cli.App{
		Store: database, Search: search.New(search.NewLexical(database), search.PassthroughMatcher{}),
		Adapters: map[domain.SourceType]adapter.Adapter{domain.SourceTypeSupportArticle: support},
		Out:      os.Stdout, ErrOut: os.Stderr,
	})
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(int(exiterrors.CodeOf(err)))
	}
}
