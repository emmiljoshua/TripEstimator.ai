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
	"time"
	"net/url"
	"fmt"
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

type createExpenseRequest struct {
	Category  string `json:"category"`
	Description string `json:"description"`
	Amount    json.Number `json:"amount"`
	Currency  string `json:"currency"`
	SpentOn	string `json:"spent_on"`
}

type createSegmentResponse struct {
	ID int64 `json:"id"`
	TripID int64 `json:"trip_id"`
	Nights int `json:"nights"`
}

type createTripResponse struct {
	ID int64 `json:"id"`
}

type createExpenseResponse struct {
	ID int64 `json:"id"`
	AmountUSD json.Number `json:"amount_usd"`
	Rate json.Number `json:"rate"`
}

type server struct {
	pool *pgxpool.Pool
}

// frankfurterResponse is the part of Frankfurter's JSON we use.
// For {"rates":{"USD":0.0064704}}, Rates["USD"] is "0.0064704".
type frankfurterResponse struct {
	Rates map[string]json.Number `json:"rates"`
}

// httpClient is shared by all outbound calls. The timeout stops a slow
// Frankfurter from hanging our request forever.
var httpClient = &http.Client{Timeout: 5 * time.Second}

// fetchUSDRate returns how many US dollars one unit of currency was worth
// on date (YYYY-MM-DD), as exact text like "0.0064704".
func fetchUSDRate(ctx context.Context, currency, date string) (string, error) {
	q := url.Values{"base": {currency}, "symbols": {"USD"}}
	endpoint := "https://api.frankfurter.dev/v1/" + url.PathEscape(date) + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("frankfurter returned status %d", resp.StatusCode)
	}

	var body frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	rate, ok := body.Rates["USD"]
	if !ok {
		return "", fmt.Errorf("no USD rate for %s", currency)
	}
	return rate.String(), nil
}

// createTrip handles POST /trips.
// It decodes a createTripRequest from the body, inserts the trip, and returns
// the new id with 201. Bad JSON returns 400. A database error returns 500.
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

// createSegment handles POST /trips/{id}/segments.
// It adds a city stay to the trip and returns the segment id, trip id, and
// nights (computed by Postgres from the dates) with 201. Returns 404 if the
// trip doesn't exist (foreign key violation, SQLSTATE 23503), 400 for a bad
// id or bad JSON, and 500 on any other database error.
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

// createExpense handles POST /segments/{id}/expenses.
// It records an expense in any currency, converts it to USD, and saves it
// in one transaction so nothing is saved if any step fails.
func (s *server) createExpense(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	segmentID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
    	return
	}

	var req createExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get the exchange rate. USD to USD is always 1, so skip the API call.
	rate := "1"
	if req.Currency != "USD" {
		rate, err = fetchUSDRate(r.Context(), req.Currency, req.SpentOn)
		if err != nil {
			log.Println("Error fetching rate:", err)
			http.Error(w, "could not get exchange rate", http.StatusBadGateway)
			return
		}
	}

	// Start a transaction: everything through tx is saved together, or not at all.
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		log.Println("Error starting transaction:", err)
		http.Error(w, "could not create expense", http.StatusInternalServerError)
		return
	}
	// Safety net: undo everything if we return early. Does nothing after Commit.
	defer tx.Rollback(r.Context())

	// Save the rate we used. tx.Exec (not s.pool.Exec) keeps it inside the transaction.
	// ON CONFLICT DO NOTHING: if this currency+date is already stored, skip it.
	_, err = tx.Exec(r.Context(),
		`INSERT INTO fx_rates (currency, rate_date, usd_per_unit)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (currency, rate_date) DO NOTHING`,
		req.Currency, req.SpentOn, rate,
	)
	if err != nil {
		log.Println("Error saving rate:", err)
		http.Error(w, "could not create expense", http.StatusInternalServerError)
		return
	}

	// Save the expense. Postgres does the money math with exact decimals:
	// amount x rate, rounded to cents. The subquery turns the category name into its id.
	var expenseID int64
	var amountUSD string
	err = tx.QueryRow(r.Context(),
		`INSERT INTO expenses (trip_segment_id, category_id, description, amount, currency, amount_in_usd, spent_on)
		 VALUES ($1, (SELECT id FROM categories WHERE name = $2), $3, $4, $5, round($4::numeric * $6::numeric, 2), $7)
		 RETURNING id, amount_in_usd::text`,
		segmentID, req.Category, req.Description, req.Amount.String(), req.Currency, rate, req.SpentOn,
	).Scan(&expenseID, &amountUSD)

	// Turn specific Postgres errors into the right status codes.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 23503 = foreign key violation: no segment with that id.
		if pgErr.Code == "23503" {
			http.Error(w, "segment not found", http.StatusNotFound)
			return
		}
		// 23502 = NOT NULL violation. On category_id, it means the name lookup found nothing.
		if pgErr.Code == "23502" && pgErr.ColumnName == "category_id" {
			http.Error(w, "unknown category", http.StatusBadRequest)
			return
		}
	}
	if err != nil {
		log.Println("Error inserting expense:", err)
		http.Error(w, "could not create expense", http.StatusInternalServerError)
		return
	}

	// Make it permanent. Until this line, the rate and the expense aren't visible to anyone else.
	if err := tx.Commit(r.Context()); err != nil {
		log.Println("Error committing:", err)
		http.Error(w, "could not create expense", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createExpenseResponse{
		ID:        expenseID,
		AmountUSD: json.Number(amountUSD),
		Rate:      json.Number(rate),
	})
}

// getTrip handles GET /trips/{id}.
// It returns the trip as JSON with 200, 404 if no trip has that id,
// 400 if the id isn't a number, and 500 on a database error.
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

// healthHandler handles GET /health.
// It runs SELECT 1 against the database and returns {"status":"ok"} with 200,
// or {"status":"db unavailable"} with 503 if the query fails.
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
	mux.HandleFunc("POST /segments/{id}/expenses", srv.createExpense)
	log.Fatal(http.ListenAndServe(":" + port, mux))
}
