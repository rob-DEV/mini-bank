package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// shared state
type server struct {
	pool *pgxpool.Pool
}

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, url) // parses config only, doesn't connect yet
	if err != nil {
		log.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil { // fail at startup, not on first request
		log.Fatalf("ping db: %v", err)
	}

	s := &server{pool: pool}

	http.HandleFunc("GET /balance/{id}", s.getBalance)
	http.HandleFunc("POST /transfer", s.transfer)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// internalError logs the real cause and gives the client a generic 500,
// so database details never leak in responses.
func internalError(w http.ResponseWriter, op string, err error) {
	log.Printf("%s: %v", op, err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
