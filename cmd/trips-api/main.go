package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"strconv"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type healthResponse struct {
	Status string `json:"status"`
}

type createTripRequest struct {
	TripName    string `json:"trip_name"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	PartySize   int    `json:"party_size"`
	OriginCity  string `json:"origin_city"`
	OriginCountry string `json:"origin_country"`
}

type tripResponse struct {
	ID int64 `json:"id"`
	TripName    string `json:"trip_name"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	PartySize   int    `json:"party_size"`
	OriginCity  string `json:"origin_city"`
	OriginCountry string `json:"origin_country"`
}

type createSegmentRequest struct {
	City  string `json:"city_stayed"`
	Country string `json:"country_stayed"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
}

type createSegmentResponse struct {
	ID int64 `json:"id"`
	TripID int64 `json:"trip_id"`
	Nights int `json:"nights"`
}

type createTripResponse struct {
	ID int64 `json:"id"`
}

type server struct {
	pool *pgxpool.Pool
}

func (s *server) createTrip(w http.ResponseWriter, r *http.Request) {
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

func (s *server) getTrip(w http.ResponseWriter, r *http.Request) {
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

func (s *server) createSegment(w http.ResponseWriter, r *http.Request) {
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

	srv := &server{pool: pool}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.healthHandler)
	mux.HandleFunc("POST /trips", srv.createTrip)
	mux.HandleFunc("GET /trips/{id}", srv.getTrip)
	mux.HandleFunc("POST /trips/{id}/segments", srv.createSegment)
	log.Fatal(http.ListenAndServe(":" + port, mux))
}
