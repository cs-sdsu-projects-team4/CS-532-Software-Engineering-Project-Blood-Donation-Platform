// Command api is the HTTP entrypoint for the Blood Donation backend.
//
// In Go, a directory whose package is named `main` and that has a func main()
// compiles to an executable. `go run ./cmd/api` builds and runs this file.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blood-donation/backend/internal/config"
	apihttp "github.com/blood-donation/backend/internal/http"
	"github.com/blood-donation/backend/internal/store"
)

func main() {
	// slog is Go's structured logger (standard library). JSON logs are easy to
	// ship to a log aggregator later.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Load configuration from environment variables. If something required is
	// missing we fail fast rather than starting a half-working server.
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Open the Postgres connection pool and verify it with a ping.
	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	router := apihttp.NewRouter(logger, db)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run the server in a goroutine (a lightweight concurrent task) so main()
	// can wait for a shutdown signal below.
	go func() {
		logger.Info("starting api server", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Block until we receive an interrupt or terminate signal (Ctrl+C, or
	// Docker stopping the container), then shut down gracefully.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}
