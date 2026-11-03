package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBadInputs(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"trip id not a number", "GET", "/trips/abc", "", http.StatusBadRequest},
		{"trip id not a number for segment", "POST", "/trips/abc/segments", "", http.StatusBadRequest},
		{"segment id not a number for expense", "POST", "/segments/abc/expenses", "", http.StatusBadRequest},
		{"bad JSON for segment", "POST", "/trips/1/segments", "{", http.StatusBadRequest},
		{"bad JSON for trip", "POST", "/trips", "{", http.StatusBadRequest},
		{"empty body for expense", "POST", "/segments/1/expenses", "", http.StatusBadRequest},
		{"bad JSON for expense", "POST", "/segments/1/expenses", "{", http.StatusBadRequest},
		{"post health", "POST", "/health", "", http.StatusMethodNotAllowed},
		{"get nope", "GET", "/nope", "", http.StatusNotFound},
		{"trip missing name", "POST", "/trips", `{"start_date":"2026-10-01","end_date":"2026-10-12","party_size":2,"origin_city":"Austin","origin_country":"US"}`, http.StatusBadRequest},
		{"trip bad date", "POST", "/trips", `{"trip_name":"T","start_date":"2026-13-01","end_date":"2026-10-12","party_size":2,"origin_city":"Austin","origin_country":"US"}`, http.StatusBadRequest},
		{"trip end before start", "POST", "/trips", `{"trip_name":"T","start_date":"2026-10-12","end_date":"2026-10-01","party_size":2,"origin_city":"Austin","origin_country":"US"}`, http.StatusBadRequest},
		{"trip party of zero", "POST", "/trips", `{"trip_name":"T","start_date":"2026-10-01","end_date":"2026-10-12","party_size":0,"origin_city":"Austin","origin_country":"US"}`, http.StatusBadRequest},
		{"segment missing city", "POST", "/trips/1/segments", `{"start_date":"2026-10-01","end_date":"2026-10-05"}`, http.StatusBadRequest},
		{"expense negative amount", "POST", "/segments/1/expenses", `{"category":"food","amount":-5,"currency":"USD","spent_on":"2026-10-01"}`, http.StatusBadRequest},
		{"expense bad currency", "POST", "/segments/1/expenses", `{"category":"food","amount":5,"currency":"US","spent_on":"2026-10-01"}`, http.StatusBadRequest},
		{"expense future date", "POST", "/segments/1/expenses", `{"category":"food","amount":5,"currency":"USD","spent_on":"2099-01-01"}`, http.StatusBadRequest},
	}

	h := NewServer(nil).Routes() // nil pool is fine, we won't hit the database

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.body != "" {
				req.Body = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)).Body
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("expected status %v, got %v", tc.want, rec.Code)
			}
		})
	}
}
