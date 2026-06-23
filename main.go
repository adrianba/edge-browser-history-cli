package main

import (
	"context"
	"os"
	"time"

	"github.com/adrianba/edge-browser-history-cli/edge"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	os.Exit(edge.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
