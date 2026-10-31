package fx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchUSDRate_ReturnsExactRate(t *testing.T) {
	// Create a fake server to simulate Frankfurter's response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2026-09-14" {
			t.Errorf("path = %q, want /2026-09-14", r.URL.Path)
		}
		if got := r.URL.Query().Get("base"); got != "JPY" {
			t.Errorf("base = %q, want JPY", got)
		}
		if got := r.URL.Query().Get("symbols"); got != "USD" {
			t.Errorf("symbols = %q, want USD", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rates":{"USD": 0.0064704}}`))
	}))
	defer server.Close()

	// Swap the baseURL to point to the fake server.
	originalBaseURL := baseURL
	baseURL = server.URL + "/"
	defer func() { baseURL = originalBaseURL }()

	rate, err := FetchUSDRate(context.Background(), "JPY", "2026-09-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != "0.0064704" {
		t.Errorf("expected rate %v, got %v", "0.0064704", rate)
	}
}

func TestFetchUSDRate_Non200(t *testing.T) {
	// Create a fake server to simulate Frankfurter's non-200 response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	// Swap the baseURL to point to the fake server.
	originalBaseURL := baseURL
	baseURL = server.URL + "/"
	defer func() { baseURL = originalBaseURL }()

	_, err := FetchUSDRate(context.Background(), "JPY", "2026-09-14")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestFetchUSDRate_NoUSD(t *testing.T) {
	// Create a fake server to simulate Frankfurter's response without USD.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rates":{"EUR": 0.0064704}}`))
	}))
	defer server.Close()

	// Swap the baseURL to point to the fake server.
	originalBaseURL := baseURL
	baseURL = server.URL + "/"
	defer func() { baseURL = originalBaseURL }()

	_, err := FetchUSDRate(context.Background(), "JPY", "2026-09-14")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
