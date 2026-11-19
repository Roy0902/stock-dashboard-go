package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"stock-dashboard/server/internal/domain"
	"stock-dashboard/server/internal/providers"
)

// statusByCode is the only mapping from a stable domain code to an HTTP status.
var statusByCode = map[domain.Code]int{
	domain.CodeInvalidTicker:       http.StatusBadRequest,
	domain.CodeDuplicateTicker:     http.StatusConflict,
	domain.CodeUnknownSymbol:       http.StatusNotFound,
	domain.CodeNotInWatchlist:      http.StatusNotFound,
	domain.CodeInvalidRange:        http.StatusBadRequest,
	domain.CodeMissingAPIKey:       http.StatusServiceUnavailable,
	domain.CodeProviderUnavailable: http.StatusBadGateway,
	domain.CodeRateLimited:         http.StatusTooManyRequests,
	domain.CodeChartUnavailable:    http.StatusBadGateway,
	domain.CodeInternalError:       http.StatusInternalServerError,
	domain.CodeNotFound:            http.StatusNotFound,
	domain.CodeRequestTooLarge:     http.StatusRequestEntityTooLarge,

	// Authentication extension (docs/contracts-auth.md section 7).
	domain.CodeUnauthenticated:     http.StatusUnauthorized,
	domain.CodeInvalidCredentials:  http.StatusUnauthorized,
	domain.CodeInvalidCode:         http.StatusUnauthorized,
	domain.CodeInvalidRecoveryCode: http.StatusUnauthorized,
	domain.CodeChallengeExpired:    http.StatusUnauthorized,
	domain.CodeEnrollmentExpired:   http.StatusUnauthorized,
	domain.CodeUsernameTaken:       http.StatusConflict,
	domain.CodeInvalidUsername:     http.StatusBadRequest,
	domain.CodeWeakPassword:        http.StatusBadRequest,
	domain.CodeInvalidRequest:      http.StatusBadRequest,
	domain.CodeCrossOriginRequest:  http.StatusForbidden,
}

func statusFor(code domain.Code) (int, bool) {
	status, ok := statusByCode[code]
	return status, ok
}

func domainError(code domain.Code) *domain.Error {
	return domain.NewError(code, domain.DefaultMessage(code))
}

func invalidTickerError() *domain.Error {
	return domainError(domain.CodeInvalidTicker)
}

func rateLimitedError(after time.Duration) *domain.Error {
	return domain.NewRateLimited(after)
}

// mapProviderFailure turns an adapter failure into the endpoint's domain error.
// Chart endpoints use CHART_UNAVAILABLE for upstream failures; quote and add paths
// use PROVIDER_UNAVAILABLE.
func mapProviderFailure(err error, unavailable domain.Code) error {
	var providerFailure *providers.Failure
	if !errors.As(err, &providerFailure) {
		return err
	}
	switch providerFailure.Kind {
	case providers.FailureUnknownSymbol:
		return domainError(domain.CodeUnknownSymbol)
	case providers.FailureMissingKey, providers.FailureAuth:
		return domainError(domain.CodeMissingAPIKey)
	case providers.FailureRateLimited:
		return rateLimitedError(providerFailure.RetryAfter)
	default:
		return domainError(unavailable)
	}
}

func (h *handlers) fail(w http.ResponseWriter, r *http.Request, err error) {
	var domainFailure *domain.Error
	if !errors.As(err, &domainFailure) {
		h.logInternal(r, err)
		domainFailure = domainError(domain.CodeInternalError)
	}

	status, ok := statusFor(domainFailure.Code)
	if !ok {
		h.logInternal(r, err)
		domainFailure = domainError(domain.CodeInternalError)
		status = http.StatusInternalServerError
	}

	if status >= http.StatusInternalServerError {
		// Codes and types are safe; messages and provider text are not logged.
		h.logger().Warn("request failed",
			"request_id", requestIDFrom(r.Context()),
			"code", string(domainFailure.Code),
		)
	}

	message := domainFailure.Message
	if message == "" {
		message = domain.DefaultMessage(domainFailure.Code)
	}
	if domainFailure.Code == domain.CodeRateLimited && domainFailure.RetryAfter > 0 {
		seconds := int(domainFailure.RetryAfter.Seconds() + 0.999)
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	writeJSON(w, status, errorBody{Error: message, Code: string(domainFailure.Code)})
}

func (h *handlers) logInternal(r *http.Request, err error) {
	h.logger().Error("internal error",
		"request_id", requestIDFrom(r.Context()),
		"error_type", fmt.Sprintf("%T", err),
	)
}

func (h *handlers) logger() *slog.Logger {
	if h.deps.Logger != nil {
		return h.deps.Logger
	}
	return slog.Default()
}
