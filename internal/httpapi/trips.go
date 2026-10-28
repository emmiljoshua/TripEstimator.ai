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
	ID            int64             `json:"id"`
	TripName      string            `json:"trip_name"`
	StartDate     string            `json:"start_date"`
	EndDate       string            `json:"end_date"`
	PartySize     int               `json:"party_size"`
	OriginCity    string            `json:"origin_city"`
	OriginCountry string            `json:"origin_country"`
	Segments      []segmentResponse `json:"segments"`
}

type categoryTotal struct {
	Category string      `json:"category"`
	TotalUSD json.Number `json:"total_usd"`
}

type segmentResponse struct {
	ID            int64           `json:"id"`
	CityStayed    string          `json:"city_stayed"`
	CountryStayed string          `json:"country_stayed"`
	StartDate     string          `json:"start_date"`
	EndDate       string          `json:"end_date"`
	Nights        int             `json:"nights"`
	TotalUSD      json.Number     `json:"total_usd"`
	Categories    []categoryTotal `json:"categories"`
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

	// Load the trip's segments, each with its USD total.
	// Query (not QueryRow) because this returns many rows.
	rows, err := s.pool.Query(r.Context(),
		`SELECT s.id, s.city_stayed, s.country_stayed, s.start_date::text, s.end_date::text, s.nights,
		        COALESCE(SUM(e.amount_in_usd), 0)::text
		 FROM trip_segments s
		 LEFT JOIN expenses e ON e.trip_segment_id = s.id
		 WHERE s.trip_id = $1
		 GROUP BY s.id
		 ORDER BY s.start_date`,
		id,
	)
	if err != nil {
		log.Println("Error fetching segments:", err)
		http.Error(w, "could not fetch trip", http.StatusInternalServerError)
		return
	}
	defer rows.Close() // release the connection when we're done reading

	t.Segments = []segmentResponse{} // empty list, so the JSON shows [] instead of null
	for rows.Next() {                // moves to the next row; false when there are none left
		var seg segmentResponse
		var total string
		if err := rows.Scan(&seg.ID, &seg.CityStayed, &seg.CountryStayed,
			&seg.StartDate, &seg.EndDate, &seg.Nights, &total); err != nil {
			log.Println("Error reading segment:", err)
			http.Error(w, "could not fetch trip", http.StatusInternalServerError)
			return
		}
		seg.TotalUSD = json.Number(total)
		seg.Categories = []categoryTotal{}
		t.Segments = append(t.Segments, seg) // add to the end of the list
	}
	if err := rows.Err(); err != nil { // catches errors that happened mid-loop
		log.Println("Error iterating segments:", err)
		http.Error(w, "could not fetch trip", http.StatusInternalServerError)
		return
	}

	// Remember where each segment sits in t.Segments,
	// so each category row can find its segment.
	index := map[int64]int{}
	for i, seg := range t.Segments {
		index[seg.ID] = i
	}

	// Per-category totals for every segment of this trip, in one query.
	catRows, err := s.pool.Query(r.Context(),
		`SELECT e.trip_segment_id, c.name, SUM(e.amount_in_usd)::text
		 FROM expenses e
		 JOIN trip_segments s ON s.id = e.trip_segment_id
		 JOIN categories c    ON c.id = e.category_id
		 WHERE s.trip_id = $1
		 GROUP BY e.trip_segment_id, c.name
		 ORDER BY e.trip_segment_id, c.name`,
		id,
	)
	if err != nil {
		log.Println("Error fetching category totals:", err)
		http.Error(w, "could not fetch trip", http.StatusInternalServerError)
		return
	}
	defer catRows.Close()

	for catRows.Next() {
		var segID int64
		var ct categoryTotal
		var total string
		if err := catRows.Scan(&segID, &ct.Category, &total); err != nil {
			log.Println("Error reading category total:", err)
			http.Error(w, "could not fetch trip", http.StatusInternalServerError)
			return
		}
		ct.TotalUSD = json.Number(total)
		i := index[segID] // which segment this row belongs to
		t.Segments[i].Categories = append(t.Segments[i].Categories, ct)
	}
	if err := catRows.Err(); err != nil {
		log.Println("Error iterating category totals:", err)
		http.Error(w, "could not fetch trip", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(t)
}
