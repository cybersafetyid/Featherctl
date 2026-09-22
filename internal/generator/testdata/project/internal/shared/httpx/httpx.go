// Package httpx holds the small HTTP helpers shared by every feature.
package httpx

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// DefaultMaxBodyBytes is the request body limit used when a caller does not
// ask for something else.
const DefaultMaxBodyBytes int64 = 1 << 20

// ErrorBody is the JSON envelope returned for a failed request.
type ErrorBody struct {
	Error string `json:"error"`
}

// JSON writes v as a JSON response with the given status code.
//
// A nil v sends the status code with no body, which is what a 204 wants.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line and headers are already on the wire, so the only
		// thing left to do is record the failure.
		slog.Error("httpx: encode response", "error", err)
	}
}

// Error writes an error envelope with the given status code.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorBody{Error: message})
}

// DecodeJSON decodes a JSON request body into dst.
//
// maxBytes limits how much of the body is read; pass 0 to use
// [DefaultMaxBodyBytes]. Unknown fields are rejected so that a typo in a client
// payload fails loudly instead of being silently ignored.
func DecodeJSON(r *http.Request, maxBytes int64, dst any) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}

	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBytes))
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}
