package domain

import (
	"math"
	"time"
)

// Direction is the signed movement of a quote. It carries the meaning that colour
// reinforces in the client.
type Direction string

const (
	DirectionUp        Direction = "up"
	DirectionDown      Direction = "down"
	DirectionUnchanged Direction = "unchanged"
)

// ProviderAttribution names where an observation came from.
type ProviderAttribution string

const (
	ProviderFinnhub    ProviderAttribution = "Finnhub"
	ProviderTwelveData ProviderAttribution = "Twelve Data"
)

// StaleAfter is the age beyond which a quote is reported as stale.
const StaleAfter = 15 * time.Minute

// Observation is a raw provider reading before change and percent are derived.
// PreviousClose is nil when the provider did not supply a usable baseline.
type Observation struct {
	Ticker        Ticker
	Price         float64
	PreviousClose *float64
	Timestamp     time.Time
	Provider      ProviderAttribution
}

// Quote is the derived, presentation-ready form of an observation.
type Quote struct {
	Ticker        Ticker
	Price         float64
	Change        float64
	ChangePercent *float64
	Direction     Direction
	Provider      ProviderAttribution
	Timestamp     time.Time
	Stale         bool
}

// IsStale reports whether timestamp is more than fifteen minutes before now.
func IsStale(timestamp, now time.Time) bool {
	return now.Sub(timestamp) > StaleAfter
}

// DeriveQuote computes change, percent, direction and staleness exactly once, on the
// server. A zero previous close shows price and change as zero with no percent; a
// missing or negative baseline keeps the price and shows no percent.
func DeriveQuote(observation Observation, now time.Time) Quote {
	quote := Quote{
		Ticker:    observation.Ticker,
		Provider:  observation.Provider,
		Timestamp: observation.Timestamp,
		Stale:     IsStale(observation.Timestamp, now),
	}

	previousClose := observation.PreviousClose
	if previousClose == nil {
		quote.Price = observation.Price
		quote.Direction = DirectionUnchanged
		return quote
	}
	if *previousClose == 0 {
		quote.Price = 0
		quote.Change = 0
		quote.Direction = DirectionUnchanged
		return quote
	}
	if *previousClose < 0 {
		quote.Price = observation.Price
		quote.Direction = DirectionUnchanged
		return quote
	}

	change := roundTo2(observation.Price - *previousClose)
	percent := roundTo2(change / *previousClose * 100)
	quote.Price = observation.Price
	quote.Change = change
	quote.ChangePercent = &percent
	quote.Direction = directionOf(change)
	return quote
}

func roundTo2(value float64) float64 {
	return math.Round(value*100) / 100
}

func directionOf(change float64) Direction {
	switch {
	case change > 0:
		return DirectionUp
	case change < 0:
		return DirectionDown
	default:
		return DirectionUnchanged
	}
}
