package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
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
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		log.Fatal(err)
	}

	err = pool.Ping(context.Background())
		if err != nil {
    		log.Fatal(err)
		}
	log.Println("db connected")
	defer pool.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
