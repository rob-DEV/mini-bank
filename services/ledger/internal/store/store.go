package store

import (
	"bytes"
	"context"
	"crypto/sha256"
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

type TransferRequest struct {
	InitiatedBy    uuid.UUID
	IdempotencyKey string
	From           uuid.UUID
	To             uuid.UUID
	AmountMinor    int64
	Currency       string
}

type TransferResult struct {
	TransactionID uuid.UUID
	Replayed      bool
}

func (r TransferRequest) validate() error {
	switch {
	case r.InitiatedBy == uuid.Nil:
		return fmt.Errorf("%w: initiated_by is required", ErrInvalidTransfer)
	case r.IdempotencyKey == "" || len(r.IdempotencyKey) > 255:
		return fmt.Errorf("%w: idempotency key must be 1-255 characters", ErrInvalidTransfer)
	case r.From == uuid.Nil || r.To == uuid.Nil:
		return fmt.Errorf("%w: from and to are required", ErrInvalidTransfer)
	case r.From == r.To:
		return fmt.Errorf("%w: cannot transfer to the same account", ErrInvalidTransfer)
	case r.AmountMinor <= 0:
		return fmt.Errorf("%w: amount must be positive", ErrInvalidTransfer)
	case len(r.Currency) != 3:
		return fmt.Errorf("%w: currency must be a 3-letter code", ErrInvalidTransfer)
	}
	return nil
}

func (r TransferRequest) hash() []byte {
	h := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%d|%s", r.From, r.To, r.AmountMinor, r.Currency))
	return h[:]
}

func (s *Store) Transfer(ctx context.Context, req TransferRequest) (TransferResult, error) {
	if err := req.validate(); err != nil {
		return TransferResult{}, err
	}

	newID, err := uuid.NewV7()
	if err != nil {
		return TransferResult{}, fmt.Errorf("generate id: %w", err)
	}
	reqHash := req.hash()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TransferResult{}, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit; undoes everything on any early return

	// 1. Claim the idempotency key. If another request with the same key is
	//    in flight, this blocks until it commits or rolls back.
	var txnID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions (id, kind, initiated_by, idempotency_key, request_hash)
		 VALUES ($1, 'transfer', $2, $3, $4)
		 ON CONFLICT (initiated_by, idempotency_key) DO NOTHING
		 RETURNING id`,
		newID, req.InitiatedBy, req.IdempotencyKey, reqHash,
	).Scan(&txnID)

	if errors.Is(err, pgx.ErrNoRows) {
		// Key already used: return the original result, or reject if the
		// request differs. Nothing was written, so the deferred rollback is fine.
		return s.replay(ctx, tx, req, reqHash)
	}
	if err != nil {
		return TransferResult{}, fmt.Errorf("claim idempotency key: %w", err)
	}

	// 2. Lock the sender so two debits can't both pass the balance check.
	//    FOR NO KEY UPDATE (not FOR UPDATE) so it doesn't conflict with the
	//    FOR KEY SHARE lock that inserting an entry's foreign key takes on the
	//    *receiver* — otherwise opposite-direction transfers deadlock.
	var (
		fromCurrency, fromStatus string
		allowNegative            bool
	)
	err = tx.QueryRow(ctx,
		`SELECT currency, status, allow_negative
		   FROM ledger_accounts
		  WHERE id = $1
		    FOR NO KEY UPDATE`,
		req.From,
	).Scan(&fromCurrency, &fromStatus, &allowNegative)
	if errors.Is(err, pgx.ErrNoRows) {
		return TransferResult{}, fmt.Errorf("%w: from %s", ErrNotFound, req.From)
	}
	if err != nil {
		return TransferResult{}, fmt.Errorf("lock sender: %w", err)
	}

	// 3. Receiver: read only. Crediting never needs a balance check.
	var toCurrency, toStatus string
	err = tx.QueryRow(ctx,
		`SELECT currency, status FROM ledger_accounts WHERE id = $1`,
		req.To,
	).Scan(&toCurrency, &toStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return TransferResult{}, fmt.Errorf("%w: to %s", ErrNotFound, req.To)
	}
	if err != nil {
		return TransferResult{}, fmt.Errorf("read receiver: %w", err)
	}

	if fromStatus != "open" || toStatus != "open" {
		return TransferResult{}, ErrAccountNotOpen
	}
	if fromCurrency != req.Currency || toCurrency != req.Currency {
		return TransferResult{}, ErrCurrencyMismatch
	}

	// 4. Funds check, safe because we hold the sender's lock.
	if !allowNegative {
		var balance int64
		err = tx.QueryRow(ctx,
			`SELECT COALESCE(sum(amount_minor), 0) FROM entries WHERE account_id = $1`,
			req.From,
		).Scan(&balance)
		if err != nil {
			return TransferResult{}, fmt.Errorf("read sender balance: %w", err)
		}
		if balance < req.AmountMinor {
			return TransferResult{}, ErrInsufficientFunds
		}
	}

	// 5. Two legs that sum to zero.
	_, err = tx.Exec(ctx,
		`INSERT INTO entries (transaction_id, account_id, amount_minor, currency)
		 VALUES ($1, $2, $3, $5),
		        ($1, $4, -$3, $5)`,
		txnID, req.To, req.AmountMinor, req.From, req.Currency,
	)
	if err != nil {
		return TransferResult{}, fmt.Errorf("insert entries: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return TransferResult{}, fmt.Errorf("commit: %w", err)
	}
	return TransferResult{TransactionID: txnID}, nil
}

func (s *Store) replay(ctx context.Context, tx pgx.Tx, req TransferRequest, reqHash []byte) (TransferResult, error) {
	var (
		existingID   uuid.UUID
		existingHash []byte
	)
	err := tx.QueryRow(ctx,
		`SELECT id, request_hash
		   FROM transactions
		  WHERE initiated_by = $1 AND idempotency_key = $2`,
		req.InitiatedBy, req.IdempotencyKey,
	).Scan(&existingID, &existingHash)
	if err != nil {
		return TransferResult{}, fmt.Errorf("load existing transaction: %w", err)
	}
	if !bytes.Equal(existingHash, reqHash) {
		return TransferResult{}, ErrIdempotencyConflict
	}
	return TransferResult{TransactionID: existingID, Replayed: true}, nil
}
