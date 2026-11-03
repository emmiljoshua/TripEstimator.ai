package httpapi

import (
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
}

// healthHandler handles GET /health.
// It runs SELECT 1 against the database and returns {"status":"ok"} with 200,
// or {"status":"db unavailable"} with 503 if the query fails.
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	_, err := s.pool.Exec(r.Context(), "SELECT 1")
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "db unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}
