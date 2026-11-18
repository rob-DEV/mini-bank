package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("account not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrCurrencyMismatch    = errors.New("currency mismatch")
	ErrAccountNotOpen      = errors.New("account is not open")
	ErrIdempotencyConflict = errors.New("idempotency key reused with a different request")
	ErrInvalidTransfer     = errors.New("invalid transfer")
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type Balance struct {
	AccountID    uuid.UUID
	Currency     string
	BalanceMinor int64
}

// Balance returns the sum of an account's entries. An account with no
// entries has a balance of 0; an unknown account is ErrNotFound.
func (s *Store) Balance(ctx context.Context, accountID uuid.UUID) (Balance, error) {
	b := Balance{AccountID: accountID}
	err := s.pool.QueryRow(ctx,
		`SELECT a.currency,
		        COALESCE((SELECT sum(e.amount_minor) FROM entries e WHERE e.account_id = a.id), 0)
		   FROM ledger_accounts a
		  WHERE a.id = $1`,
		accountID,
	).Scan(&b.Currency, &b.BalanceMinor)
	if errors.Is(err, pgx.ErrNoRows) {
		return Balance{}, ErrNotFound
	}
	if err != nil {
		return Balance{}, fmt.Errorf("query balance: %w", err)
	}
	return b, nil
}
