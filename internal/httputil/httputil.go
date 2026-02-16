// Package httputil provides shared HTTP helper functions for the Gatify gateway.
package httputil

import (
"encoding/json"
"log/slog"
"net/http"
)

// WriteJSON encodes body as JSON and writes it with the given status code.
// Uses json.Marshal to produce compact JSON, followed by a newline.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		slog.Error("failed to encode JSON response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(append(payload, '\n')); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}
