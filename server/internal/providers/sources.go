package providers

import (
	"context"

	"stock-dashboard/server/internal/domain"
)

// QuoteSource supplies a raw observation for one ticker.
type QuoteSource interface {
	Quote(ctx context.Context, ticker domain.Ticker) (domain.Observation, error)
}

// SeriesSource supplies daily closes newest first for one ticker.
type SeriesSource interface {
	DailySeries(ctx context.Context, ticker domain.Ticker, points int) ([]domain.PricePoint, error)
}
