package providers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"stock-dashboard/server/internal/domain"
)

// TwelveDataOptions configures the fallback quote and only chart adapter.
type TwelveDataOptions struct {
	BaseURL string
	APIKey  string
	Budget  *Budget
	Client  *HTTPClient
}

// TwelveData reads quotes and daily series from Twelve Data.
type TwelveData struct {
	baseURL string
	apiKey  string
	budget  *Budget
	client  *HTTPClient
}

func NewTwelveData(options TwelveDataOptions) *TwelveData {
	if options.Budget == nil {
		options.Budget = NewBudget(domain.ProviderTwelveData, DefaultLimits.TwelveDataRPM, time.Now)
	}
	if options.Client == nil {
		options.Client = NewHTTPClient(DefaultLimits.UpstreamTimeout)
	}
	return &TwelveData{
		baseURL: trimBase(options.BaseURL),
		apiKey:  options.APIKey,
		budget:  options.Budget,
		client:  options.Client,
	}
}

// Quote maps close, previous_close and timestamp. Numeric fields may be strings.
func (t *TwelveData) Quote(ctx context.Context, ticker domain.Ticker) (domain.Observation, error) {
	provider := domain.ProviderTwelveData
	if t.apiKey == "" {
		return domain.Observation{}, failure(provider, FailureMissingKey)
	}
	if err := t.budget.Reserve(ctx); err != nil {
		return domain.Observation{}, err
	}

	params := url.Values{}
	params.Set("symbol", ticker.String())
	params.Set("apikey", t.apiKey)
	response, err := t.client.GetJSON(ctx, provider, t.baseURL+"/quote?"+params.Encode())
	if err != nil {
		return domain.Observation{}, err
	}

	body, ok := asObject(response.Body)
	if statusFailure := twelveDataStatus(response, body, ok); statusFailure != nil {
		return domain.Observation{}, statusFailure
	}

	price, ok := flexNumber(body["close"])
	if !ok {
		return domain.Observation{}, failure(provider, FailureUnavailable)
	}
	at, ok := flexNumber(body["timestamp"])
	if !ok {
		return domain.Observation{}, failure(provider, FailureUnavailable)
	}

	observation := domain.Observation{
		Ticker:    ticker,
		Price:     price,
		Timestamp: unixSeconds(at),
		Provider:  provider,
	}
	if previousClose, ok := flexNumber(body["previous_close"]); ok {
		observation.PreviousClose = &previousClose
	}
	return observation, nil
}

// DailySeries maps values[].datetime and values[].close. Values arrive newest first.
func (t *TwelveData) DailySeries(ctx context.Context, ticker domain.Ticker, points int) ([]domain.PricePoint, error) {
	provider := domain.ProviderTwelveData
	if t.apiKey == "" {
		return nil, failure(provider, FailureMissingKey)
	}
	if err := t.budget.Reserve(ctx); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("symbol", ticker.String())
	params.Set("interval", "1day")
	params.Set("outputsize", strconv.Itoa(points))
	params.Set("apikey", t.apiKey)
	response, err := t.client.GetJSON(ctx, provider, t.baseURL+"/time_series?"+params.Encode())
	if err != nil {
		return nil, err
	}

	body, ok := asObject(response.Body)
	if statusFailure := twelveDataStatus(response, body, ok); statusFailure != nil {
		return nil, statusFailure
	}

	values, ok := body["values"].([]any)
	if !ok {
		return nil, failure(provider, FailureUnavailable)
	}

	series := make([]domain.PricePoint, 0, len(values))
	for _, entry := range values {
		row, ok := asObject(entry)
		if !ok {
			return nil, failure(provider, FailureUnavailable)
		}
		date, ok := row["datetime"].(string)
		if !ok || strings.TrimSpace(date) == "" {
			return nil, failure(provider, FailureUnavailable)
		}
		// Only canonical calendar dates reach OrderOldestFirst, which sorts the
		// strings lexicographically. A non-canonical value such as 2026-1-5 or
		// 17/10/2026 is a provider failure, so fallback/stale handling applies.
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, failure(provider, FailureUnavailable)
		}
		close, ok := flexNumber(row["close"])
		if !ok {
			return nil, failure(provider, FailureUnavailable)
		}
		series = append(series, domain.PricePoint{Date: date, Close: close})
	}
	if len(series) == 0 {
		return nil, failure(provider, FailureUnavailable)
	}
	return series, nil
}

// twelveDataStatus maps the provider's error envelope and HTTP status. A body with
// status "error" and code 400 or 404 means the symbol is unknown.
func twelveDataStatus(response Response, body map[string]any, decoded bool) *Failure {
	provider := domain.ProviderTwelveData
	if statusFailure := authOrLimitFailure(provider, response); statusFailure != nil {
		return statusFailure
	}
	if decoded {
		if state, ok := body["status"].(string); ok && state == "error" {
			if code, ok := flexNumber(body["code"]); ok && (code == 400 || code == 404) {
				return failure(provider, FailureUnknownSymbol)
			}
			return failure(provider, FailureUnavailable)
		}
		if _, hasError := body["error"]; hasError {
			return failure(provider, FailureUnavailable)
		}
	}
	if response.Status == http.StatusNotFound {
		return failure(provider, FailureUnknownSymbol)
	}
	if response.Status < 200 || response.Status > 299 {
		return failure(provider, FailureUnavailable)
	}
	if !decoded {
		return failure(provider, FailureUnavailable)
	}
	return nil
}
