package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"mini-bank/services/ledger/internal/store"
	"net/http"

	"github.com/google/uuid"
)

type Ledger interface {
	Balance(ctx context.Context, accountID uuid.UUID) (store.Balance, error)
	Transfer(ctx context.Context, req store.TransferRequest) (store.TransferResult, error)
}

type Handler struct {
	ledger Ledger
	log    *slog.Logger
}

func New(ledger Ledger, log *slog.Logger) *Handler {
	return &Handler{ledger: ledger, log: log}
}

type route struct {
	pattern string
	handler http.HandlerFunc
}

func (h *Handler) routes() []route {
	return []route{
		{"GET /accounts/{id}/balance", h.getBalance},
		{"POST /transfers", h.postTransfer},
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	for _, rt := range h.routes() {
		mux.HandleFunc(rt.pattern, rt.handler)
		h.log.Info("route registered", "route", rt.pattern)
	}
	return mux
}

// --- GET /accounts/{id}/balance ---

type balanceResponse struct {
	AccountID    uuid.UUID `json:"account_id"`
	BalanceMinor int64     `json:"balance_minor"`
	Currency     string    `json:"currency"`
}

func (h *Handler) getBalance(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "account id must be a UUID")
		return
	}

	b, err := h.ledger.Balance(r.Context(), id)
	if err != nil {
		h.storeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, balanceResponse{
		AccountID:    b.AccountID,
		BalanceMinor: b.BalanceMinor,
		Currency:     b.Currency,
	})
}

// --- POST /transfers ---
//
// Headers:
//   Idempotency-Key: <client-chosen string>   (required)
//   X-Customer-Id:   <uuid>                   (required; set by the gateway once auth exists)

type transferRequest struct {
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	AmountMinor   int64  `json:"amount_minor"`
	Currency      string `json:"currency"`
}

type transferResponse struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	Replayed      bool      `json:"replayed"`
}

func (h *Handler) postTransfer(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Idempotency-Key header is required")
		return
	}
	initiatedBy, err := uuid.Parse(r.Header.Get("X-Customer-Id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "X-Customer-Id header must be a UUID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4<<10) // 4 KiB is plenty
	var body transferRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // reject typos like "ammount_minor"
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body: "+err.Error())
		return
	}

	from, err1 := uuid.Parse(body.FromAccountID)
	to, err2 := uuid.Parse(body.ToAccountID)
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "from_account_id and to_account_id must be UUIDs")
		return
	}

	res, err := h.ledger.Transfer(r.Context(), store.TransferRequest{
		InitiatedBy:    initiatedBy,
		IdempotencyKey: key,
		From:           from,
		To:             to,
		AmountMinor:    body.AmountMinor,
		Currency:       body.Currency,
	})
	if err != nil {
		h.storeError(w, r, err)
		return
	}

	// 201 for a new transfer, 200 for a replay — same body either way, so the
	// client can't tell the difference unless it wants to (Stripe does the same).
	status := http.StatusCreated
	if res.Replayed {
		status = http.StatusOK
		w.Header().Set("Idempotent-Replayed", "true")
	}
	writeJSON(w, status, transferResponse{TransactionID: res.TransactionID, Replayed: res.Replayed})
}
