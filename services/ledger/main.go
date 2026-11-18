package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mini-bank/services/ledger/internal/store"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const shutdownTimeout = 10 * time.Second

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("ledger stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL not set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := connectDB(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	srv := newHTTPServer(":8080", store.New(pool), log)

	errCh := make(chan error, 1)
	go func() {
		log.Info("http listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err) // failed on its own, e.g. port in use
	case <-ctx.Done():
		stop()
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx) // waits for in-flight requests
}

func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
