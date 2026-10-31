package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// newTestServer starts a fresh Postgres in Docker, applies the migrations,
// and returns the API wired to it. The container is deleted when the test ends.
func newTestServer(t *testing.T) http.Handler {
	t.Helper() // failures point at the calling test's line, not this helper
	if testing.Short() {
		t.Skip("integration test: needs Docker") // go test -short skips these
	}
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("trips_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		// Run your migrations when the database first starts, in file-name order.
		postgres.WithInitScripts(
			filepath.Join("..", "..", "migrations", "000001_init_schema.up.sql"),
			filepath.Join("..", "..", "migrations", "000002_seed_categories.up.sql"),
		),
		postgres.BasicWaitStrategies(), // wait until Postgres is actually ready
	)
	testcontainers.CleanupContainer(t, ctr) // delete the container when the test finishes
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close) // close connections before the container goes away

	return NewServer(pool).Routes()
}

// do sends a request through the API and returns the recorded response.
// body can be any Go value and is sent as JSON. Pass nil for no body.
func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, &buf))
	return rec
}

// TestHealthWithRealDB proves the helper works: a real SELECT 1 against a real Postgres.
func TestHealthWithRealDB(t *testing.T) {
	h := newTestServer(t)
	rec := do(t, h, "GET", "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

// TestTripEndToEnd runs a two-city trip through the real API and a real Postgres:
// create the trip, add Munich and Vienna, log expenses in USD, then check the
// nested totals from GET /trips/{id} and the normalized rates from GET /rates.
// USD only, so no request ever reaches the real Frankfurter.
func TestTripEndToEnd(t *testing.T) {
	h := newTestServer(t)

	// 1. Create the trip.
	rec := do(t, h, "POST", "/trips", createTripRequest{
		TripName: "Test Europe", StartDate: "2026-11-15", EndDate: "2026-11-26",
		PartySize: 2, OriginCity: "Austin", OriginCountry: "US",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create trip: status = %d, body = %s", rec.Code, rec.Body)
	}
	var trip createTripResponse
	decode(t, rec, &trip)

	// 2-3. Add Munich and Vienna.
	munich := addSegment(t, h, trip.ID, "Munich", "DE", "2026-11-15", "2026-11-19")
	vienna := addSegment(t, h, trip.ID, "Vienna", "AT", "2026-11-22", "2026-11-26")
	if munich.Nights != 4 || vienna.Nights != 4 {
		t.Errorf("nights = %d and %d, want 4 and 4", munich.Nights, vienna.Nights)
	}

	// 4-6. Log expenses in USD.
	addExpense(t, h, munich.ID, "lodging", "800", "2026-11-15")
	addExpense(t, h, munich.ID, "food", "120", "2026-11-15")
	addExpense(t, h, vienna.ID, "lodging", "600", "2026-11-22")

	// 7. The trip comes back with each city's total.
	rec = do(t, h, "GET", fmt.Sprintf("/trips/%d", trip.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get trip: status = %d, body = %s", rec.Code, rec.Body)
	}
	var got tripResponse
	decode(t, rec, &got)
	if len(got.Segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(got.Segments))
	}
	if s := got.Segments[0].TotalUSD.String(); s != "920.00" {
		t.Errorf("Munich total = %s, want 920.00", s)
	}
	if s := got.Segments[1].TotalUSD.String(); s != "600.00" {
		t.Errorf("Vienna total = %s, want 600.00", s)
	}

	// 8. The normalized rates.
	rec = do(t, h, "GET", "/rates", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get rates: status = %d, body = %s", rec.Code, rec.Body)
	}
	var rates []rateResponse
	decode(t, rec, &rates)

	want := map[string]string{
		"Munich lodging": "200.00", // 800 / 4 nights
		"Munich food":    "15.00",  // 120 / (2 people x 4 days)
		"Vienna lodging": "150.00", // 600 / 4 nights
	}
	for _, r := range rates {
		key := r.City + " " + r.Category
		if w, ok := want[key]; ok {
			if r.RateUSD.String() != w {
				t.Errorf("%s rate = %s, want %s", key, r.RateUSD, w)
			}
			delete(want, key) // checked, so remove it
		}
	}
	for key := range want { // anything left was never returned
		t.Errorf("missing rate for %s", key)
	}
}

// decode reads a JSON response body into v, failing the test if it can't.
func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
}

// addSegment adds a city stay to a trip and returns the created segment.
func addSegment(t *testing.T, h http.Handler, tripID int64, city, country, start, end string) createSegmentResponse {
	t.Helper()
	rec := do(t, h, "POST", fmt.Sprintf("/trips/%d/segments", tripID), createSegmentRequest{
		City: city, Country: country, StartDate: start, EndDate: end,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add %s: status = %d, body = %s", city, rec.Code, rec.Body)
	}
	var seg createSegmentResponse
	decode(t, rec, &seg)
	return seg
}

// addExpense logs a USD expense on a segment, failing the test if it isn't created.
func addExpense(t *testing.T, h http.Handler, segmentID int64, category, amount, spentOn string) {
	t.Helper()
	rec := do(t, h, "POST", fmt.Sprintf("/segments/%d/expenses", segmentID), createExpenseRequest{
		Category: category, Amount: json.Number(amount), Currency: "USD", SpentOn: spentOn,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add %s expense: status = %d, body = %s", category, rec.Code, rec.Body)
	}
}
