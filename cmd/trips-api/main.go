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

type server struct {
	pool *pgxpool.Pool
}

func (s *server) healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")

    _, err := s.pool.Exec(r.Context(), "SELECT 1")
    if err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(healthResponse{Status: "db unavailable"})
        return
    }

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

	srv := &server{pool: pool}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.healthHandler)
	log.Fatal(http.ListenAndServe(":8080", mux))
}
