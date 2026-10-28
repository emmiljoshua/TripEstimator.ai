package main

import (
	"context"
	"github.com/emmiljoshua/TripEstimate.ai/internal/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"os"
)

func main() {
	dbUrl := os.Getenv("DATABASE_URL")
	port := os.Getenv("PORT")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}
	if port == "" {
		port = "8080" // default port
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

	srv := httpapi.NewServer(pool)
	log.Fatal(http.ListenAndServe(":"+port, srv.Routes()))
}
