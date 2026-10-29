package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// rateResponse is one normalized rate: what a category cost in a city,
// per night, per person per day, or per person per trip.
type rateResponse struct {
	City     string      `json:"city"`
	Country  string      `json:"country"`
	Category string      `json:"category"`
	Unit     string      `json:"unit"`
	RateUSD  json.Number `json:"rate_usd"`
}

// getRates handles GET /rates.
// It returns your normalized spending rates by city and category, across all
// trips: per night, per person per day, or per person per trip, depending on
// the category. Returns 200, or 500 on a database error.
func (s *Server) getRates(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		WITH seg AS (
		  SELECT s.city_stayed, s.country_stayed, c.name AS category, c.normalization_unit,
		         SUM(e.amount_in_usd) AS total_usd,
		         CASE c.normalization_unit
		           WHEN 'per_night'           THEN s.nights
		           WHEN 'per_person_per_day'  THEN t.party_size * GREATEST(s.nights, 1)
		           WHEN 'per_person_per_trip' THEN t.party_size
		         END AS units
		  FROM expenses e
		  JOIN trip_segments s ON s.id = e.trip_segment_id
		  JOIN trips t         ON t.id = s.trip_id
		  JOIN categories c    ON c.id = e.category_id
		  GROUP BY s.id, t.party_size, c.name, c.normalization_unit
		)
		SELECT city_stayed, country_stayed, category, normalization_unit,
		       round(SUM(total_usd) / SUM(units), 2)::text
		FROM seg
		GROUP BY city_stayed, country_stayed, category, normalization_unit
		HAVING SUM(units) > 0
		ORDER BY city_stayed, category`)
	if err != nil {
		log.Println("Error fetching rates:", err)
		http.Error(w, "could not fetch rates", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	rates := []rateResponse{}
	for rows.Next() {
		var rate rateResponse
		var value string // the rate as exact text, like "283.21"
		if err := rows.Scan(&rate.City, &rate.Country, &rate.Category, &rate.Unit, &value); err != nil {
			log.Println("Error reading rate:", err)
			http.Error(w, "could not fetch rates", http.StatusInternalServerError)
			return
		}
		rate.RateUSD = json.Number(value)
		rates = append(rates, rate)
	}
	if err := rows.Err(); err != nil {
		log.Println("Error iterating rates:", err)
		http.Error(w, "could not fetch rates", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rates)
}
