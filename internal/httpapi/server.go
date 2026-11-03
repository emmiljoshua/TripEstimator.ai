// Package httpapi holds the HTTP handlers for the trips API.
package httpapi

import (
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

// Server holds what the handlers share, like the database pool.
type Server struct {
	pool *pgxpool.Pool
}

// NewServer returns a Server that uses pool for database access.
func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{pool: pool}
}

// writeJSON writes the given value as JSON to the response writer with the specified status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an error message as JSON to the response writer with the specified status code.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// parseDate turns "2026-11-15" into a time.Time, or returns an error
// if the text isn't a real date in YYYY-MM-DD form.
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
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
