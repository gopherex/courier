// Command courier runs the notification API, delivery relay, or schema migrations.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopherex/courier/internal/app"
)

func run() int {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	config, err := app.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	migrateOnly := len(os.Args) == commandArgCount && os.Args[1] == "migrate"
	if len(os.Args) > 1 && !migrateOnly {
		slog.Error("usage: courier [migrate]")
		return 1
	}

	if err = app.Run(ctx, config, migrateOnly); err != nil {
		slog.Error("courier stopped", "error", err)
		return 1
	}

	return 0
}

const commandArgCount = 2

func main() { os.Exit(run()) }
