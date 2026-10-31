// Package fx fetches currency exchange rates from an external API
package fx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// frankfurterResponse is the part of Frankfurter's JSON we use.
// For {"rates":{"USD":0.0064704}}, Rates["USD"] is "0.0064704".
type frankfurterResponse struct {
	Rates map[string]json.Number `json:"rates"`
}

// httpClient is shared by all outbound calls. The timeout stops a slow
// Frankfurter from hanging our request forever.
var httpClient = &http.Client{Timeout: 5 * time.Second}

// baseURL is where Frankfurter lives. Tests swap it for a fake server's address.
var baseURL = "https://api.frankfurter.dev/v1/"

// FetchUSDRate returns how many US dollars one unit of currency was worth
// on date (YYYY-MM-DD), as exact text like "0.0064704".
func FetchUSDRate(ctx context.Context, currency, date string) (string, error) {
	q := url.Values{"base": {currency}, "symbols": {"USD"}}
	endpoint := baseURL + url.PathEscape(date) + "?" + q.Encode()

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
