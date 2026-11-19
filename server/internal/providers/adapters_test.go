package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"stock-dashboard/server/internal/domain"
)

func requireFailure(t *testing.T, err error, kind FailureKind, provider domain.ProviderAttribution) {
	t.Helper()
	var failure *Failure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v (%T), want *Failure", err, err)
	}
	if failure.Kind != kind {
		t.Fatalf("kind = %q, want %q", failure.Kind, kind)
	}
	if failure.Provider != provider {
		t.Fatalf("provider = %q, want %q", failure.Provider, provider)
	}
}

func finnhubSource(t *testing.T, status int, body string) (*Finnhub, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/api/v1/quote" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("symbol") != "AAPL" {
			t.Errorf("symbol = %q", r.URL.Query().Get("symbol"))
		}
		if r.URL.Query().Get("token") != "test-key" {
			t.Errorf("token = %q", r.URL.Query().Get("token"))
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	source := NewFinnhub(FinnhubOptions{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Budget:  NewBudget(domain.ProviderFinnhub, 55, time.Now),
		Client:  NewHTTPClient(2 * time.Second),
	})
	return source, &requests
}

func TestFinnhubQuoteSuccess(t *testing.T) {
	source, requests := finnhubSource(t, http.StatusOK, `{"c":190.12,"d":1.24,"dp":0.66,"pc":188.88,"t":1763303400}`)
	observation, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Price != 190.12 {
		t.Fatalf("price = %v", observation.Price)
	}
	if observation.PreviousClose == nil || *observation.PreviousClose != 188.88 {
		t.Fatalf("previous close = %v", observation.PreviousClose)
	}
	want := time.Unix(1763303400, 0).UTC()
	if !observation.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", observation.Timestamp, want)
	}
	if observation.Provider != domain.ProviderFinnhub {
		t.Fatalf("provider = %q", observation.Provider)
	}
	if observation.Ticker != "AAPL" {
		t.Fatalf("ticker = %q", observation.Ticker)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d", requests.Load())
	}
}

func TestFinnhubPreviousCloseAbsentOrNull(t *testing.T) {
	for _, body := range []string{
		`{"c":50,"t":1763303400}`,
		`{"c":50,"pc":null,"t":1763303400}`,
	} {
		source, _ := finnhubSource(t, http.StatusOK, body)
		observation, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
		if err != nil {
			t.Fatal(err)
		}
		if observation.PreviousClose != nil {
			t.Fatalf("previous close = %v, want nil", *observation.PreviousClose)
		}
	}
}

func TestFinnhubUnknownSymbol(t *testing.T) {
	source, _ := finnhubSource(t, http.StatusOK, `{"c":0,"d":null,"dp":null,"pc":null,"t":0}`)
	_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	requireFailure(t, err, FailureUnknownSymbol, domain.ProviderFinnhub)
}

func TestFinnhubServerError(t *testing.T) {
	source, _ := finnhubSource(t, http.StatusInternalServerError, `{"error":"token=SECRET-XYZ rejected"}`)
	_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	requireFailure(t, err, FailureUnavailable, domain.ProviderFinnhub)
}

func TestFinnhubMalformedBody(t *testing.T) {
	for _, body := range []string{`{`, `{"unexpected":true}`, `{"c":"190.12","t":1763303400}`, `[]`} {
		source, _ := finnhubSource(t, http.StatusOK, body)
		_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
		requireFailure(t, err, FailureUnavailable, domain.ProviderFinnhub)
	}
}

func TestFinnhubMissingKey(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"c":1,"t":1}`))
	}))
	defer server.Close()

	source := NewFinnhub(FinnhubOptions{BaseURL: server.URL})
	_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	requireFailure(t, err, FailureMissingKey, domain.ProviderFinnhub)
	if requests.Load() != 0 {
		t.Fatalf("missing key still called upstream")
	}
}

func TestFinnhubBudgetRefusal(t *testing.T) {
	source, requests := finnhubSource(t, http.StatusOK, `{"c":1,"pc":1,"t":1763303400}`)
	source.budget = NewBudget(domain.ProviderFinnhub, 1, time.Now)
	if err := source.budget.Reserve(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	requireFailure(t, err, FailureRateLimited, domain.ProviderFinnhub)
	if requests.Load() != 0 {
		t.Fatalf("budget refusal still called upstream")
	}
}

func twelveDataSource(t *testing.T, status int, body string) (*TwelveData, *atomic.Int32, *url.Values) {
	t.Helper()
	var requests atomic.Int32
	var query url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		query = r.URL.Query()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	source := NewTwelveData(TwelveDataOptions{
		BaseURL: server.URL,
		APIKey:  "td-key",
		Budget:  NewBudget(domain.ProviderTwelveData, 7, time.Now),
		Client:  NewHTTPClient(2 * time.Second),
	})
	return source, &requests, &query
}

func TestTwelveDataQuoteSuccessNumbers(t *testing.T) {
	source, _, query := twelveDataSource(t, http.StatusOK, `{"symbol":"AAPL","close":190.12,"change":1.24,"percent_change":0.66,"previous_close":188.88,"timestamp":1763303400}`)
	observation, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Price != 190.12 {
		t.Fatalf("price = %v", observation.Price)
	}
	if observation.PreviousClose == nil || *observation.PreviousClose != 188.88 {
		t.Fatalf("previous close = %v", observation.PreviousClose)
	}
	if observation.Provider != domain.ProviderTwelveData {
		t.Fatalf("provider = %q", observation.Provider)
	}
	if (*query).Get("symbol") != "AAPL" || (*query).Get("apikey") != "td-key" {
		t.Fatalf("query = %v", *query)
	}
}

func TestTwelveDataQuoteSuccessNumericStrings(t *testing.T) {
	source, _, _ := twelveDataSource(t, http.StatusOK, `{"close":"190.12","previous_close":"188.88","timestamp":"1763303400"}`)
	observation, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	if err != nil {
		t.Fatal(err)
	}
	if observation.Price != 190.12 {
		t.Fatalf("price = %v", observation.Price)
	}
	if observation.PreviousClose == nil || *observation.PreviousClose != 188.88 {
		t.Fatalf("previous close = %v", observation.PreviousClose)
	}
	want := time.Unix(1763303400, 0).UTC()
	if !observation.Timestamp.Equal(want) {
		t.Fatalf("timestamp = %v", observation.Timestamp)
	}
}

func TestTwelveDataUnknownSymbol(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "error envelope 200 code 404", status: http.StatusOK, body: `{"code":404,"message":"not found","status":"error"}`},
		{name: "error envelope 200 code 400", status: http.StatusOK, body: `{"code":"400","message":"bad symbol","status":"error"}`},
		{name: "http 404", status: http.StatusNotFound, body: `{"code":404,"message":"not found","status":"error"}`},
		{name: "http 404 non-json", status: http.StatusNotFound, body: `not json`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, _, _ := twelveDataSource(t, test.status, test.body)
			_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
			requireFailure(t, err, FailureUnknownSymbol, domain.ProviderTwelveData)
		})
	}
}

func TestTwelveDataFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "server error", status: http.StatusInternalServerError, body: `{"code":500,"message":"token=SECRET","status":"error"}`},
		{name: "error envelope 200", status: http.StatusOK, body: `{"code":429,"message":"limit","status":"error"}`},
		{name: "malformed", status: http.StatusOK, body: `{`},
		{name: "missing close", status: http.StatusOK, body: `{"timestamp":1763303400}`},
		{name: "wrong close type", status: http.StatusOK, body: `{"close":"abc","timestamp":1763303400}`},
		{name: "nan close", status: http.StatusOK, body: `{"close":"NaN","timestamp":1763303400}`},
		{name: "infinite close", status: http.StatusOK, body: `{"close":"1e999","timestamp":1763303400}`},
		{name: "missing timestamp", status: http.StatusOK, body: `{"close":190.12}`},
		{name: "array body", status: http.StatusOK, body: `[1,2]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, _, _ := twelveDataSource(t, test.status, test.body)
			_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
			requireFailure(t, err, FailureUnavailable, domain.ProviderTwelveData)
		})
	}
}

func TestTwelveDataMissingKey(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"close":1,"timestamp":1}`))
	}))
	defer server.Close()

	source := NewTwelveData(TwelveDataOptions{BaseURL: server.URL})
	_, err := source.Quote(context.Background(), domain.MustTicker("AAPL"))
	requireFailure(t, err, FailureMissingKey, domain.ProviderTwelveData)
	if requests.Load() != 0 {
		t.Fatalf("missing key still called upstream")
	}
}

func TestTwelveDataDailySeries(t *testing.T) {
	source, _, query := twelveDataSource(t, http.StatusOK, `{"meta":{},"values":[{"datetime":"2026-11-14","close":"191.2"},{"datetime":"2026-11-13","close":190.1}],"status":"ok"}`)
	points, err := source.DailySeries(context.Background(), domain.MustTicker("AAPL"), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 {
		t.Fatalf("points = %d", len(points))
	}
	if points[0].Date != "2026-11-14" || points[0].Close != 191.2 {
		t.Fatalf("first point = %+v", points[0])
	}
	if (*query).Get("interval") != "1day" || (*query).Get("outputsize") != "30" {
		t.Fatalf("query = %v", *query)
	}
	if (*query).Get("apikey") != "td-key" {
		t.Fatal("apikey missing from query")
	}
}

func TestTwelveDataDailySeriesFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		kind   FailureKind
	}{
		{name: "empty values", status: http.StatusOK, body: `{"values":[]}`, kind: FailureUnavailable},
		{name: "missing values", status: http.StatusOK, body: `{"status":"ok"}`, kind: FailureUnavailable},
		{name: "bad close", status: http.StatusOK, body: `{"values":[{"datetime":"2026-11-14","close":"nope"}]}`, kind: FailureUnavailable},
		{name: "bad datetime", status: http.StatusOK, body: `{"values":[{"datetime":42,"close":1}]}`, kind: FailureUnavailable},
		{name: "empty datetime", status: http.StatusOK, body: `{"values":[{"datetime":"","close":1}]}`, kind: FailureUnavailable},
		{name: "slash date", status: http.StatusOK, body: `{"values":[{"datetime":"17/10/2026","close":1}]}`, kind: FailureUnavailable},
		{name: "non-padded date", status: http.StatusOK, body: `{"values":[{"datetime":"2026-1-5","close":1}]}`, kind: FailureUnavailable},
		{name: "unknown symbol", status: http.StatusOK, body: `{"code":404,"status":"error"}`, kind: FailureUnknownSymbol},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, _, _ := twelveDataSource(t, test.status, test.body)
			_, err := source.DailySeries(context.Background(), domain.MustTicker("AAPL"), 30)
			requireFailure(t, err, test.kind, domain.ProviderTwelveData)
		})
	}
}

func TestTwelveDataDailySeriesKeepsCanonicalDate(t *testing.T) {
	source, _, _ := twelveDataSource(t, http.StatusOK, `{"values":[{"datetime":"2026-10-17","close":189.5}]}`)
	points, err := source.DailySeries(context.Background(), domain.MustTicker("AAPL"), 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Date != "2026-10-17" || points[0].Close != 189.5 {
		t.Fatalf("points = %+v", points)
	}
}

func TestTwelveDataDailySeriesRejectsNonCanonicalDate(t *testing.T) {
	source, _, _ := twelveDataSource(t, http.StatusOK, `{"values":[{"datetime":"17/10/2026","close":189.5}]}`)
	_, err := source.DailySeries(context.Background(), domain.MustTicker("AAPL"), 30)
	requireFailure(t, err, FailureUnavailable, domain.ProviderTwelveData)
}

func TestTwelveDataUpstream429IsRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"status":"error","message":"quota"}`))
	}))
	defer server.Close()
	source := NewTwelveData(TwelveDataOptions{
		BaseURL: server.URL,
		APIKey:  "td-key",
		Budget:  NewBudget(domain.ProviderTwelveData, 7, time.Now),
		Client:  NewHTTPClient(time.Second),
	})

	_, err := source.Quote(context.Background(), mustTicker("AAPL"))
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureRateLimited {
		t.Fatalf("error = %v, want rate-limited failure", err)
	}
	if failure.RetryAfter != 7*time.Second {
		t.Fatalf("retry after = %s", failure.RetryAfter)
	}
}

func TestFinnhubAuthFailureIsDistinct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad token"}`))
	}))
	defer server.Close()
	source := NewFinnhub(FinnhubOptions{
		BaseURL: server.URL,
		APIKey:  "fin-key",
		Budget:  NewBudget(domain.ProviderFinnhub, 55, time.Now),
		Client:  NewHTTPClient(time.Second),
	})

	_, err := source.Quote(context.Background(), mustTicker("AAPL"))
	requireFailure(t, err, FailureAuth, domain.ProviderFinnhub)
}

func mustTicker(raw string) domain.Ticker {
	ticker, err := domain.Canonicalize(raw)
	if err != nil {
		panic(err)
	}
	return ticker
}
