package domain

import (
	"math"
	"testing"
	"time"
)

func pointer(value float64) *float64 {
	return &value
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestDeriveQuoteUp(t *testing.T) {
	now := time.Date(2026, 11, 16, 14, 31, 0, 0, time.UTC)
	observation := Observation{
		Ticker:        "AAPL",
		Price:         190.12,
		PreviousClose: pointer(188.88),
		Timestamp:     now.Add(-time.Minute),
		Provider:      ProviderFinnhub,
	}

	quote := DeriveQuote(observation, now)
	if quote.Change != 1.24 {
		t.Fatalf("change = %v, want 1.24", quote.Change)
	}
	if quote.ChangePercent == nil || *quote.ChangePercent != 0.66 {
		t.Fatalf("percent = %v, want 0.66", quote.ChangePercent)
	}
	if quote.Direction != DirectionUp {
		t.Fatalf("direction = %q", quote.Direction)
	}
	if quote.Price != 190.12 {
		t.Fatalf("price = %v", quote.Price)
	}
	if quote.Provider != ProviderFinnhub || !quote.Timestamp.Equal(observation.Timestamp) {
		t.Fatalf("metadata changed: %+v", quote)
	}
	if quote.Stale {
		t.Fatal("fresh observation reported stale")
	}
}

func TestDeriveQuoteDirection(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name     string
		price    float64
		previous float64
		change   float64
		percent  float64
		dir      Direction
	}{
		{name: "down", price: 190, previous: 200, change: -10, percent: -5, dir: DirectionDown},
		{name: "unchanged", price: 100, previous: 100, change: 0, percent: 0, dir: DirectionUnchanged},
		{name: "rounds change", price: 190.125, previous: 188, change: 2.13, percent: 1.13, dir: DirectionUp},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			quote := DeriveQuote(Observation{
				Ticker:        "AAPL",
				Price:         test.price,
				PreviousClose: pointer(test.previous),
				Timestamp:     now,
				Provider:      ProviderFinnhub,
			}, now)
			if quote.Change != test.change {
				t.Fatalf("change = %v, want %v", quote.Change, test.change)
			}
			if quote.ChangePercent == nil || *quote.ChangePercent != test.percent {
				t.Fatalf("percent = %v, want %v", quote.ChangePercent, test.percent)
			}
			if quote.Direction != test.dir {
				t.Fatalf("direction = %q, want %q", quote.Direction, test.dir)
			}
		})
	}
}

func TestDeriveQuoteZeroPreviousClose(t *testing.T) {
	now := time.Now().UTC()
	quote := DeriveQuote(Observation{
		Ticker:        "AAPL",
		Price:         12.34,
		PreviousClose: pointer(0),
		Timestamp:     now,
		Provider:      ProviderFinnhub,
	}, now)

	if quote.Price != 0 || quote.Change != 0 {
		t.Fatalf("price/change = %v/%v, want 0/0", quote.Price, quote.Change)
	}
	if quote.ChangePercent != nil {
		t.Fatalf("percent = %v, want nil", *quote.ChangePercent)
	}
	if quote.Direction != DirectionUnchanged {
		t.Fatalf("direction = %q", quote.Direction)
	}
}

func TestDeriveQuoteMissingOrNegativePreviousClose(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name     string
		previous *float64
	}{
		{name: "missing", previous: nil},
		{name: "negative", previous: pointer(-1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			quote := DeriveQuote(Observation{
				Ticker:        "AAPL",
				Price:         50,
				PreviousClose: test.previous,
				Timestamp:     now,
				Provider:      ProviderTwelveData,
			}, now)
			if quote.Price != 50 || quote.Change != 0 {
				t.Fatalf("price/change = %v/%v, want 50/0", quote.Price, quote.Change)
			}
			if quote.ChangePercent != nil {
				t.Fatalf("percent = %v, want nil", *quote.ChangePercent)
			}
			if quote.Direction != DirectionUnchanged {
				t.Fatalf("direction = %q", quote.Direction)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	now := time.Date(2026, 11, 16, 14, 30, 0, 0, time.UTC)
	tests := []struct {
		name      string
		timestamp time.Time
		want      bool
	}{
		{name: "14 minutes", timestamp: now.Add(-14 * time.Minute), want: false},
		{name: "exactly 15 minutes", timestamp: now.Add(-15 * time.Minute), want: false},
		{name: "15 minutes and a second", timestamp: now.Add(-15*time.Minute - time.Second), want: true},
		{name: "16 minutes", timestamp: now.Add(-16 * time.Minute), want: true},
		{name: "future", timestamp: now.Add(time.Minute), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsStale(test.timestamp, now); got != test.want {
				t.Fatalf("IsStale = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDeriveQuoteStale(t *testing.T) {
	now := time.Date(2026, 11, 16, 14, 16, 0, 0, time.UTC)
	quote := DeriveQuote(Observation{
		Ticker:        "AAPL",
		Price:         190,
		PreviousClose: pointer(188),
		Timestamp:     now.Add(-16 * time.Minute),
		Provider:      ProviderFinnhub,
	}, now)
	if !quote.Stale {
		t.Fatal("16-minute-old quote not stale")
	}
}

func TestRoundTo2(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{input: 2.125, want: 2.13},
		{input: 2.124, want: 2.12},
		{input: -2.125, want: -2.13},
		{input: 0, want: 0},
	}
	for _, test := range tests {
		got := roundTo2(test.input)
		if got != test.want {
			t.Fatalf("roundTo2(%v) = %v, want %v", test.input, got, test.want)
		}
	}
}
