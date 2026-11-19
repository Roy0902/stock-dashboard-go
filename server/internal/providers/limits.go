package providers

import "time"

// Limits are the production provider settings. They are defined once and
// referenced by the composition root and the adapter defaults.
type Limits struct {
	FinnhubRPM      int
	TwelveDataRPM   int
	UpstreamTimeout time.Duration
	CacheTTL        time.Duration
	Workers         int
}

// DefaultLimits matches docs/contracts.md section 5.
var DefaultLimits = Limits{
	FinnhubRPM:      55,
	TwelveDataRPM:   7,
	UpstreamTimeout: 2 * time.Second,
	CacheTTL:        time.Minute,
	Workers:         8,
}
