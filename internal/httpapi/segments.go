package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"log"
	"net/http"
	"strconv"
)

type createSegmentRequest struct {
	City      string `json:"city_stayed"`
	Country   string `json:"country_stayed"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type createSegmentResponse struct {
	ID     int64 `json:"id"`
	TripID int64 `json:"trip_id"`
	Nights int   `json:"nights"`
}

// createSegment handles POST /trips/{id}/segments.
// It adds a city stay to the trip and returns the segment id, trip id, and
// nights (computed by Postgres from the dates) with 201. Returns 404 if the
// trip doesn't exist (foreign key violation, SQLSTATE 23503), 400 for a bad
// id or bad JSON, and 500 on any other database error.
func (s *Server) createSegment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	tripID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req createSegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var segmentID int64
	var nights int
	err = s.pool.QueryRow(r.Context(),
		`INSERT INTO trip_segments (trip_id, city_stayed, country_stayed, start_date, end_date) VALUES ($1, $2, $3, $4, $5) RETURNING id, nights`,
		tripID, req.City, req.Country, req.StartDate, req.EndDate,
	).Scan(&segmentID, &nights)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}

	if err != nil {
		log.Println("Error inserting segment:", err)
		http.Error(w, "Could not create segment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createSegmentResponse{ID: segmentID, TripID: tripID, Nights: nights})
}
