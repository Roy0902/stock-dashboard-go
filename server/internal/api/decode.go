package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"stock-dashboard/server/internal/domain"
)

// decodeJSON is the single body decoder for handlers. It enforces a body cap,
// strict field names, a single JSON value and application/json when a type is
// present. An oversized body is 413 regardless of the endpoint's malformed-body
// code; malformed small bodies keep the endpoint's domain copy.
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any, limit int64, malformed func() error) error {
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return malformed()
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return domain.NewError(domain.CodeRequestTooLarge, domain.DefaultMessage(domain.CodeRequestTooLarge))
		}
		return malformed()
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return domain.NewError(domain.CodeRequestTooLarge, domain.DefaultMessage(domain.CodeRequestTooLarge))
		}
		return malformed()
	}
	return nil
}
