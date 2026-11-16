package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

import _ "net/http/pprof"

type balanceResponse struct {
	Account  string `json:"account"`
	Balance  int64  `json:"balance"`
	Currency string `json:"currency"`
}

type server struct {
	pool *pgxpool.Pool
}

func (s *server) getBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var resp balanceResponse
	err := s.pool.QueryRow(r.Context(),
		`SELECT id, balance_minor, currency
		   FROM accounts
		  WHERE id = $1`,
		id,
	).Scan(&resp.Account, &resp.Balance, &resp.Currency)

	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("get balance %s: %v", id, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	config.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	s := &server{pool: pool} // 1. put the pool in a struct

	http.HandleFunc("GET /balance/{id}", s.getBalance) // 2. register the method

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
