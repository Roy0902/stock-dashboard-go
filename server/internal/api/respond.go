package api

import (
	"encoding/json"
	"net/http"

	"stock-dashboard/server/internal/domain"
)

// writeJSON marshals first so a marshal failure still returns the documented
// error envelope rather than a text/plain second body.
func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		status = http.StatusInternalServerError
		body, _ = json.Marshal(errorBody{
			Error: domain.DefaultMessage(domain.CodeInternalError),
			Code:  string(domain.CodeInternalError),
		})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
