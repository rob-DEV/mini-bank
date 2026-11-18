package httpapi

import (
	"context"
	"log/slog"
	"mini-bank/services/ledger/internal/store"
	"net/http"

	"github.com/google/uuid"
)

type Ledger interface {
	Balance(ctx context.Context, accountID uuid.UUID) (store.Balance, error)
}

type Handler struct {
	ledger Ledger
	log    *slog.Logger
}

func New(ledger Ledger, log *slog.Logger) *Handler {
	return &Handler{ledger: ledger, log: log}
}

// Routes returns the HTTP handler for all ledger endpoints.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /accounts/{id}/balance", h.getBalance)
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

	writeJSON(w, http.StatusOK, balanceResponse{
		AccountID:    b.AccountID,
		BalanceMinor: b.BalanceMinor,
		Currency:     b.Currency,
	})
}
