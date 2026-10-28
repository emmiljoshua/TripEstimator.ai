package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"log"
	"net/http"
	"strconv"
)

type createTripRequest struct {
	TripName      string `json:"trip_name"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	PartySize     int    `json:"party_size"`
	OriginCity    string `json:"origin_city"`
	OriginCountry string `json:"origin_country"`
}

type createTripResponse struct {
	ID int64 `json:"id"`
}

type tripResponse struct {
	ID            int64  `json:"id"`
	TripName      string `json:"trip_name"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	PartySize     int    `json:"party_size"`
	OriginCity    string `json:"origin_city"`
	OriginCountry string `json:"origin_country"`
}

// createTrip handles POST /trips.
// It decodes a createTripRequest from the body, inserts the trip, and returns
// the new id with 201. Bad JSON returns 400. A database error returns 500.
func (s *Server) createTrip(w http.ResponseWriter, r *http.Request) {
	var req createTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Process the request...
	var id int64
	err := s.pool.QueryRow(r.Context(),
		`INSERT INTO trips (trip_name, start_date, end_date, party_size, origin_city, origin_country) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		req.TripName, req.StartDate, req.EndDate, req.PartySize, req.OriginCity, req.OriginCountry,
	).Scan(&id)

	if err != nil {
		log.Println("Error inserting trip:", err)
		http.Error(w, "Could not create trip", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createTripResponse{ID: id})
}

// getTrip handles GET /trips/{id}.
// It returns the trip as JSON with 200, 404 if no trip has that id,
// 400 if the id isn't a number, and 500 on a database error.
func (s *Server) getTrip(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var t tripResponse
	err = s.pool.QueryRow(r.Context(),
		`SELECT id, trip_name, start_date::text, end_date::text, party_size, origin_city, origin_country FROM trips WHERE id=$1`,
		id,
	).Scan(&t.ID, &t.TripName, &t.StartDate, &t.EndDate, &t.PartySize, &t.OriginCity, &t.OriginCountry)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println("Error fetching trip:", err)
		http.Error(w, "could not fetch trip", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(t)
}
