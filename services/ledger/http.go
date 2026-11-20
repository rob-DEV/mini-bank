package main

import (
	"log/slog"
	"mini-bank/services/ledger/internal/httpapi"
	"mini-bank/services/ledger/internal/store"
	"net/http"
	"time"
)

func newHTTPServer(addr string, st *store.Store, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           httpapi.LoggingMiddleware(httpapi.New(st, log).Routes()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second, // keep-alive connections
	}
}
