package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jviklun1/pp-volvo-cars/internal/adapter"
	"github.com/jviklun1/pp-volvo-cars/internal/adapter/supportgraphql"
	"github.com/jviklun1/pp-volvo-cars/internal/cli"
	"github.com/jviklun1/pp-volvo-cars/internal/domain"
	exiterrors "github.com/jviklun1/pp-volvo-cars/internal/errors"
	"github.com/jviklun1/pp-volvo-cars/internal/search"
	"github.com/jviklun1/pp-volvo-cars/internal/store"
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
