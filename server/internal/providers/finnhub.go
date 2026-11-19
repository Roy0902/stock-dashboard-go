package providers

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"stock-dashboard/server/internal/domain"
)

// FinnhubOptions configures the primary quote adapter.
type FinnhubOptions struct {
	BaseURL string
	APIKey  string
	Budget  *Budget
	Client  *HTTPClient
}

// Finnhub reads quotes from the Finnhub /quote endpoint.
type Finnhub struct {
	baseURL string
	apiKey  string
	budget  *Budget
	client  *HTTPClient
}

func NewFinnhub(options FinnhubOptions) *Finnhub {
	if options.Budget == nil {
		options.Budget = NewBudget(domain.ProviderFinnhub, DefaultLimits.FinnhubRPM, time.Now)
	}
	if options.Client == nil {
		options.Client = NewHTTPClient(DefaultLimits.UpstreamTimeout)
	}
	return &Finnhub{
		baseURL: trimBase(options.BaseURL),
		apiKey:  options.APIKey,
		budget:  options.Budget,
		client:  options.Client,
	}
}

// Quote maps c, pc and t. c == 0 and t == 0 means the symbol is unknown.
func (f *Finnhub) Quote(ctx context.Context, ticker domain.Ticker) (domain.Observation, error) {
	provider := domain.ProviderFinnhub
	if f.apiKey == "" {
		return domain.Observation{}, failure(provider, FailureMissingKey)
	}
	if err := f.budget.Reserve(ctx); err != nil {
		return domain.Observation{}, err
	}

	params := url.Values{}
	params.Set("symbol", ticker.String())
	params.Set("token", f.apiKey)
	response, err := f.client.GetJSON(ctx, provider, f.baseURL+"/api/v1/quote?"+params.Encode())
	if err != nil {
		return domain.Observation{}, err
	}
	if statusFailure := authOrLimitFailure(provider, response); statusFailure != nil {
		return domain.Observation{}, statusFailure
	}
	if response.Status != http.StatusOK {
		return domain.Observation{}, failure(provider, FailureUnavailable)
	}

	body, ok := asObject(response.Body)
	if !ok {
		return domain.Observation{}, failure(provider, FailureUnavailable)
	}
	if _, hasError := body["error"]; hasError {
		return domain.Observation{}, failure(provider, FailureUnavailable)
	}

	price, ok := jsonNumber(body["c"])
	if !ok {
		return domain.Observation{}, failure(provider, FailureUnavailable)
	}
	at, ok := jsonNumber(body["t"])
	if !ok {
		return domain.Observation{}, failure(provider, FailureUnavailable)
	}
	if price == 0 && at == 0 {
		return domain.Observation{}, failure(provider, FailureUnknownSymbol)
	}

	observation := domain.Observation{
		Ticker:    ticker,
		Price:     price,
		Timestamp: unixSeconds(at),
		Provider:  provider,
	}
	if previousClose, ok := jsonNumber(body["pc"]); ok {
		observation.PreviousClose = &previousClose
	}
	return observation, nil
}

func unixSeconds(value float64) time.Time {
	return time.Unix(int64(value), 0).UTC()
}

func trimBase(base string) string {
	for len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}
	return base
}
