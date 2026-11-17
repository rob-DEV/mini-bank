package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres error code for a CHECK constraint violation
// (our balance_non_negative constraint).
const checkViolation = "23514"

type transferRequest struct {
	From        string `json:"from"`
	To          string `json:"to"`
	AmountMinor int64  `json:"amount_minor"`
}

func (s *server) transfer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10) // cap body at 1 KiB

	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.From == "" || req.To == "" || req.From == req.To || req.AmountMinor <= 0 {
		http.Error(w, "invalid transfer", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		internalError(w, "begin tx", err)
		return
	}
	// If we return early for any reason, the transaction is rolled back.
	// After a successful Commit this is a harmless no-op.
	defer tx.Rollback(ctx)

	// Take both locks at once
	rows, err := tx.Query(ctx,
		`SELECT id FROM accounts WHERE id IN ($1, $2) ORDER BY id FOR UPDATE`,
		req.From, req.To,
	)
	if err != nil {
		internalError(w, "lock accounts", err)
		return
	}
	locked := 0
	for rows.Next() {
		locked++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		internalError(w, "lock accounts", err)
		return
	}
	if locked != 2 {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}

	// Debit. The CHECK constraint rejects it if the balance would go negative.
	tag, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_minor = balance_minor - $1 WHERE id = $2`,
		req.AmountMinor, req.From,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == checkViolation {
			http.Error(w, "insufficient funds", http.StatusUnprocessableEntity)
			return
		}
		internalError(w, "debit "+req.From, err)
		return
	}
	if tag.RowsAffected() != 1 {
		http.Error(w, "account not found: "+req.From, http.StatusNotFound)
		return
	}

	// Credit.
	tag, err = tx.Exec(ctx,
		`UPDATE accounts SET balance_minor = balance_minor + $1 WHERE id = $2`,
		req.AmountMinor, req.To,
	)
	if err != nil {
		internalError(w, "credit "+req.To, err)
		return
	}
	if tag.RowsAffected() != 1 {
		http.Error(w, "account not found: "+req.To, http.StatusNotFound)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		internalError(w, "commit", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
