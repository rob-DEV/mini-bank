package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
)

type balanceResponse struct {
	Account  string `json:"account"`
	Balance  int64  `json:"balance_minor"`
	Currency string `json:"currency"`
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
		internalError(w, "get balance "+id, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
