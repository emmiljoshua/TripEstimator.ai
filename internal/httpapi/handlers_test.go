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
