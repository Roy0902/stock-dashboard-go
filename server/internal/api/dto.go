package api

import (
	"time"

	"stock-dashboard/server/internal/domain"
)

type errorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type statusBody struct {
	Status string `json:"status"`
}

type watchlistBody struct {
	Tickers []string `json:"tickers"`
}

type quoteBody struct {
	Ticker        string   `json:"ticker"`
	Price         float64  `json:"price"`
	Change        float64  `json:"change"`
	ChangePercent *float64 `json:"changePercent"`
	Direction     string   `json:"direction"`
	Provider      string   `json:"provider"`
	Timestamp     string   `json:"timestamp"`
	Stale         bool     `json:"stale"`
}

type quotesBody struct {
	Quotes []quoteBody `json:"quotes"`
}

type pointBody struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
}

type chartBody struct {
	Ticker string      `json:"ticker"`
	Range  string      `json:"range"`
	Source string      `json:"source"`
	Points []pointBody `json:"points"`
}

// Authentication extension shapes (docs/contracts-auth.md section 6).
type authUserBody struct {
	Username    string `json:"username"`
	TOTPEnabled bool   `json:"totpEnabled"`
}

type authStatusBody struct {
	SetupRequired bool          `json:"setupRequired"`
	Authenticated bool          `json:"authenticated"`
	User          *authUserBody `json:"user,omitempty"`
}

type authUserResponseBody struct {
	User *authUserBody `json:"user"`
}

type setupResponseBody struct {
	Username      string   `json:"username"`
	Secret        string   `json:"secret"`
	OTPAuthURL    string   `json:"otpauthUrl"`
	RecoveryCodes []string `json:"recoveryCodes"`
}

type challengeResponseBody struct {
	Status           string `json:"status"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

func toWatchlistBody(watchlist domain.Watchlist) watchlistBody {
	tickers := watchlist.Tickers()
	out := make([]string, 0, len(tickers))
	for _, ticker := range tickers {
		out = append(out, ticker.String())
	}
	return watchlistBody{Tickers: out}
}

func toQuotesBody(quotes []domain.Quote) quotesBody {
	out := make([]quoteBody, 0, len(quotes))
	for _, quote := range quotes {
		out = append(out, toQuoteBody(quote))
	}
	return quotesBody{Quotes: out}
}

func toQuoteBody(quote domain.Quote) quoteBody {
	return quoteBody{
		Ticker:        quote.Ticker.String(),
		Price:         quote.Price,
		Change:        quote.Change,
		ChangePercent: quote.ChangePercent,
		Direction:     string(quote.Direction),
		Provider:      string(quote.Provider),
		Timestamp:     quote.Timestamp.UTC().Format(time.RFC3339),
		Stale:         quote.Stale,
	}
}

func toChartBody(ticker domain.Ticker, rangeValue domain.PriceRange, points []domain.PricePoint) chartBody {
	out := make([]pointBody, 0, len(points))
	for _, point := range points {
		out = append(out, pointBody{Date: point.Date, Close: point.Close})
	}
	return chartBody{
		Ticker: ticker.String(),
		Range:  string(rangeValue),
		Source: string(domain.ProviderTwelveData),
		Points: out,
	}
}
