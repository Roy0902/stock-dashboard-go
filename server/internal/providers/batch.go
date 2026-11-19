package providers

import (
	"time"

	"stock-dashboard/server/internal/domain"
)

// Batch is one quotes refresh. RateLimitedAfter is set only when no quote was served
// and at least one ticker was refused by a local budget. Unavailable is set when no
// quote was served and at least one ticker failed because every provider errored.
// Canceled is set when the caller's context ended before every ticker was dispatched
// or observed, so an empty result is never mistaken for "no quotes exist".
// CredentialFailure is set when no quote was served and every failure was the
// upstream rejecting its credential (401/403 or a missing key), so the API can
// answer MISSING_API_KEY instead of PROVIDER_UNAVAILABLE.
type Batch struct {
	Quotes            []domain.Quote
	RateLimitedAfter  *time.Duration
	Unavailable       bool
	Canceled          bool
	CredentialFailure bool
}
