package main

import (
	"encoding/json"
	"log"
	"net/http"
)

import _ "net/http/pprof"

var balances = map[string]int64{
	"alice": 1000,
	"bob":   500,
	"carol": 200,
}

type balanceResponse struct {
	Account string `json:"account"`
	Balance int64  `json:"balance"`
}

func getBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	bal, ok := balances[id]
	if !ok {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balanceResponse{Account: id, Balance: bal})
}

func main() {
	http.HandleFunc("GET /balance/{id}", getBalance)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
