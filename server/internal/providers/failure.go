package providers

import (
	"time"

	"stock-dashboard/server/internal/domain"
)

// FailureKind classifies an upstream failure without carrying provider text.
type FailureKind string

const (
	FailureUnknownSymbol FailureKind = "unknown-symbol"
	FailureUnavailable   FailureKind = "unavailable"
	FailureMissingKey    FailureKind = "missing-key"
	FailureRateLimited   FailureKind = "rate-limited"
	// FailureAuth means the upstream rejected the credential (401/403).
	FailureAuth FailureKind = "auth"
	// FailureCanceled means the caller's context ended, not that the provider is down.
	FailureCanceled FailureKind = "canceled"
)

// Failure is the only error type provider adapters return for expected failures.
// It deliberately contains no URL, query string, key or upstream body.
type Failure struct {
	Kind       FailureKind
	Provider   domain.ProviderAttribution
	RetryAfter time.Duration
}

func (f *Failure) Error() string {
	return string(f.Provider) + ": " + string(f.Kind)
}
