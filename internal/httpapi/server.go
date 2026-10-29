// Package httpapi holds the HTTP handlers for the trips API.
package httpapi

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
)

// Server holds what the handlers share, like the database pool.
type Server struct {
	pool *pgxpool.Pool
}

// NewServer returns a Server that uses pool for database access.
func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{pool: pool}
}

// Routes returns a handler with every endpoint registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.healthHandler)
	mux.HandleFunc("POST /trips", s.createTrip)
	mux.HandleFunc("GET /trips/{id}", s.getTrip)
	mux.HandleFunc("POST /trips/{id}/segments", s.createSegment)
	mux.HandleFunc("POST /segments/{id}/expenses", s.createExpense)
	mux.HandleFunc("GET /rates", s.getRates)
	return mux
}
