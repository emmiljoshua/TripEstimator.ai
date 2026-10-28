package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/emmiljoshua/TripEstimate.ai/internal/fx"
	"github.com/jackc/pgx/v5/pgconn"
	"log"
	"net/http"
	"strconv"
)

type createExpenseRequest struct {
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Amount      json.Number `json:"amount"`
	Currency    string      `json:"currency"`
	SpentOn     string      `json:"spent_on"`
}

type createExpenseResponse struct {
	ID        int64       `json:"id"`
	AmountUSD json.Number `json:"amount_usd"`
	Rate      json.Number `json:"rate"`
}

// createExpense handles POST /segments/{id}/expenses.
// It records an expense in any currency, converts it to USD, and saves it
// in one transaction so nothing is saved if any step fails.
func (s *Server) createExpense(w http.ResponseWriter, r *http.Request) {
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
		rate, err = fx.FetchUSDRate(r.Context(), req.Currency, req.SpentOn)
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
