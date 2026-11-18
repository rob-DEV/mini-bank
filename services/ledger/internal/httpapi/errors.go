package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"mini-bank/services/ledger/internal/store"
	"net/http"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *Handler) storeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrInvalidTransfer):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, store.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", err.Error())
	case errors.Is(err, store.ErrInsufficientFunds):
		writeError(w, http.StatusUnprocessableEntity, "insufficient_funds", err.Error())
	case errors.Is(err, store.ErrCurrencyMismatch):
		writeError(w, http.StatusUnprocessableEntity, "currency_mismatch", err.Error())
	case errors.Is(err, store.ErrAccountNotOpen):
		writeError(w, http.StatusUnprocessableEntity, "account_not_open", err.Error())
	case errors.Is(err, context.Canceled):
		// Client went away; nobody is listening for a response.
		return
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "timeout", "request timed out")
	default:
		// Unexpected: log the real cause, never leak it to the client.
		h.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status) // headers can't change after this line
	_ = json.NewEncoder(w).Encode(v)
}
