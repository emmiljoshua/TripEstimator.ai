package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
