package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"log"
	"net/http"
	"strconv"
	"time"
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
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req createSegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate the segment's own fields.
	if req.City == "" || req.Country == "" {
		writeError(w, http.StatusBadRequest, "city_stayed and country_stayed are required")
		return
	}
	segStart, err := parseDate(req.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "start_date must be YYYY-MM-DD")
		return
	}
	segEnd, err := parseDate(req.EndDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "end_date must be YYYY-MM-DD")
		return
	}
	if segEnd.Before(segStart) {
		writeError(w, http.StatusBadRequest, "end_date can't be before start_date")
		return
	}

	// Look up the trip's dates so we can check the segment fits inside them.
	var tripStart, tripEnd time.Time
	err = s.pool.QueryRow(r.Context(),
		`SELECT start_date, end_date FROM trips WHERE id = $1`,
		tripID,
	).Scan(&tripStart, &tripEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "trip not found")
		return
	}
	if err != nil {
		log.Println("Error looking up trip:", err)
		writeError(w, http.StatusInternalServerError, "could not create segment")
		return
	}

	// The segment has to fit inside the trip.
	if segStart.Before(tripStart) || segEnd.After(tripEnd) {
		writeError(w, http.StatusBadRequest, "segment dates must fall within the trip")
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
		writeError(w, http.StatusNotFound, "trip not found")
		return
	}

	if err != nil {
		log.Println("Error inserting segment:", err)
		writeError(w, http.StatusInternalServerError, "Could not create segment")
		return
	}

	writeJSON(w, http.StatusCreated, createSegmentResponse{ID: segmentID, TripID: tripID, Nights: nights})
}
