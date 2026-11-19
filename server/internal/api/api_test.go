package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"stock-dashboard/server/internal/domain"
	"stock-dashboard/server/internal/providers"
)

const testSessionUserID = "usr_test_owner"

type fakeStore struct {
	mu        sync.Mutex
	lists     map[string]domain.Watchlist
	listCalls int
}

func newFakeStore(tickers ...string) *fakeStore {
	return newFakeStoreFor(testSessionUserID, tickers...)
}

func newFakeStoreFor(userID string, tickers ...string) *fakeStore {
	list := domain.Watchlist{}
	for _, ticker := range tickers {
		var err error
		list, err = list.Add(ticker)
		if err != nil {
			panic(err)
		}
	}
	return &fakeStore{lists: map[string]domain.Watchlist{userID: list}}
}

func (s *fakeStore) List(userID string) domain.Watchlist {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listCalls++
	return s.lists[userID]
}

func (s *fakeStore) Update(userID string, fn func(domain.Watchlist) (domain.Watchlist, error)) (domain.Watchlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, err := fn(s.lists[userID])
	if err != nil {
		return domain.Watchlist{}, err
	}
	s.lists[userID] = next
	return next, nil
}

func (s *fakeStore) ticks() []string {
	return s.ticksFor(testSessionUserID)
}

func (s *fakeStore) ticksFor(userID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.lists[userID]
	out := make([]string, 0, list.Len())
	for _, ticker := range list.Tickers() {
		out = append(out, ticker.String())
	}
	return out
}

type fakeQuotes struct {
	mu            sync.Mutex
	batch         providers.Batch
	verifyErr     error
	verifyCalls   []domain.Ticker
	series        []domain.PricePoint
	seriesErr     error
	seriesCalls   int
	seriesRange   domain.PriceRange
	quotesCalls   int
	panicOnQuotes any
	forgotten     []domain.Ticker
	echoTickers   bool
	quoted        []domain.Ticker
}

func (f *fakeQuotes) Quotes(_ context.Context, tickers []domain.Ticker) providers.Batch {
	f.mu.Lock()
	f.quotesCalls++
	f.quoted = append([]domain.Ticker(nil), tickers...)
	panicValue := f.panicOnQuotes
	batch := f.batch
	echo := f.echoTickers
	f.mu.Unlock()
	if panicValue != nil {
		panic(panicValue)
	}
	if echo {
		quotes := make([]domain.Quote, 0, len(tickers))
		for _, ticker := range tickers {
			quotes = append(quotes, domain.Quote{
				Ticker:    ticker,
				Price:     1,
				Direction: domain.DirectionUnchanged,
				Provider:  domain.ProviderFinnhub,
			})
		}
		return providers.Batch{Quotes: quotes}
	}
	return batch
}

func (f *fakeQuotes) VerifySymbol(_ context.Context, ticker domain.Ticker) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.verifyCalls = append(f.verifyCalls, ticker)
	return f.verifyErr
}

func (f *fakeQuotes) DailySeries(_ context.Context, _ domain.Ticker, rangeValue domain.PriceRange) ([]domain.PricePoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seriesCalls++
	f.seriesRange = rangeValue
	return f.series, f.seriesErr
}

func (f *fakeQuotes) Forget(ticker domain.Ticker) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, ticker)
}

func (f *fakeQuotes) verifyCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.verifyCalls)
}

func (f *fakeQuotes) quoteCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.quotesCalls
}

type testEnv struct {
	handler  http.Handler
	store    *fakeStore
	quotes   *fakeQuotes
	logs     *bytes.Buffer
	spaCalls int
}

func newTestEnv(t *testing.T, store *fakeStore, quotes *fakeQuotes, keys Keys) *testEnv {
	t.Helper()
	return newTestEnvWithAuth(t, store, quotes, keys, allowAllAuth{})
}

func newTestEnvWithAuth(t *testing.T, store *fakeStore, quotes *fakeQuotes, keys Keys, authService AuthService) *testEnv {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	env := &testEnv{store: store, quotes: quotes, logs: logs}

	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		env.spaCalls++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>spa shell</title>")
	})
	env.handler = NewRouter(Deps{
		Watchlist: store,
		Quotes:    quotes,
		Auth:      authService,
		Keys:      keys,
		Logger:    logger,
	}, spa)
	return env
}

func completeKeys() Keys {
	return Keys{Finnhub: true, TwelveData: true}
}

func (e *testEnv) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, "http://127.0.0.1:3000"+target, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", recorder.Body.String(), err)
	}
	return body
}

func decodeWatchlist(t *testing.T, recorder *httptest.ResponseRecorder) watchlistBody {
	t.Helper()
	var body watchlistBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode watchlist %q: %v", recorder.Body.String(), err)
	}
	return body
}

func requireError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code domain.Code, message string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d (body %q)", recorder.Code, status, recorder.Body.String())
	}
	body := decodeError(t, recorder)
	if body.Code != string(code) {
		t.Fatalf("code = %q, want %q", body.Code, code)
	}
	if message != "" && body.Error != message {
		t.Fatalf("message = %q, want %q", body.Error, message)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q", got)
	}
}

func TestWatchlistList(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/watchlist", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d", recorder.Code)
		}
		if recorder.Body.String() != `{"tickers":[]}` {
			t.Fatalf("body = %q", recorder.Body.String())
		}
	})

	t.Run("populated order", func(t *testing.T) {
		env := newTestEnv(t, newFakeStore("AAPL", "MSFT"), &fakeQuotes{}, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/watchlist", "")
		body := decodeWatchlist(t, recorder)
		if len(body.Tickers) != 2 || body.Tickers[0] != "AAPL" || body.Tickers[1] != "MSFT" {
			t.Fatalf("tickers = %v", body.Tickers)
		}
	})
}

func TestWatchlistAdd(t *testing.T) {
	t.Run("canonicalises and returns the full list", func(t *testing.T) {
		store := newFakeStore("MSFT")
		quotes := &fakeQuotes{}
		env := newTestEnv(t, store, quotes, completeKeys())

		recorder := env.do(t, http.MethodPost, "/api/watchlist", `{"ticker":"aapl"}`)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d (%q)", recorder.Code, recorder.Body.String())
		}
		body := decodeWatchlist(t, recorder)
		if len(body.Tickers) != 2 || body.Tickers[0] != "MSFT" || body.Tickers[1] != "AAPL" {
			t.Fatalf("tickers = %v", body.Tickers)
		}
		got := store.ticks()
		if len(got) != 2 || got[1] != "AAPL" {
			t.Fatalf("store = %v", got)
		}
		quotes.mu.Lock()
		calls := quotes.verifyCalls
		quotes.mu.Unlock()
		if len(calls) != 1 || calls[0] != "AAPL" {
			t.Fatalf("verify calls = %v", calls)
		}
	})

	invalid := []string{
		`{"ticker":""}`,
		`{"ticker":"1AAPL"}`,
		`{"ticker":"AA-PL"}`,
		`{"ticker":"A1234567890"}`,
		`{}`,
		`{"ticker":42}`,
		`null`,
		`not json`,
		`{"ticker":"AAPL"} {"ticker":"MSFT"}`,
		`{"ticker":"AAPL","extra":true}`,
	}
	for _, body := range invalid {
		t.Run("invalid "+body[:min(len(body), 24)], func(t *testing.T) {
			store := newFakeStore()
			quotes := &fakeQuotes{}
			env := newTestEnv(t, store, quotes, completeKeys())
			recorder := env.do(t, http.MethodPost, "/api/watchlist", body)
			requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidTicker, "Enter a valid ticker, e.g. AAPL.")
			if len(store.ticks()) != 0 {
				t.Fatalf("invalid body mutated the store: %v", store.ticks())
			}
			if quotes.verifyCount() != 0 {
				t.Fatal("invalid body reached provider verification")
			}
		})
	}

	t.Run("non-JSON content type is refused", func(t *testing.T) {
		env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/watchlist", strings.NewReader(`{"ticker":"AAPL"}`))
		request.Header.Set("Content-Type", "text/plain")
		recorder := httptest.NewRecorder()
		env.handler.ServeHTTP(recorder, request)
		requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidTicker, "Enter a valid ticker, e.g. AAPL.")
		if len(env.store.ticks()) != 0 {
			t.Fatal("cross-site style request mutated the store")
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		quotes := &fakeQuotes{}
		env := newTestEnv(t, newFakeStore("AAPL"), quotes, completeKeys())
		recorder := env.do(t, http.MethodPost, "/api/watchlist", `{"ticker":"aapl"}`)
		requireError(t, recorder, http.StatusConflict, domain.CodeDuplicateTicker, "AAPL is already in your watchlist.")
		if quotes.verifyCount() != 0 {
			t.Fatal("duplicate reached provider verification")
		}
	})

	t.Run("unknown symbol", func(t *testing.T) {
		quotes := &fakeQuotes{verifyErr: &providers.Failure{Kind: providers.FailureUnknownSymbol, Provider: domain.ProviderFinnhub}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodPost, "/api/watchlist", `{"ticker":"ZZZZ"}`)
		requireError(t, recorder, http.StatusNotFound, domain.CodeUnknownSymbol, "Symbol not found.")
	})

	t.Run("provider unavailable", func(t *testing.T) {
		quotes := &fakeQuotes{verifyErr: &providers.Failure{Kind: providers.FailureUnavailable, Provider: domain.ProviderFinnhub}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodPost, "/api/watchlist", `{"ticker":"AAPL"}`)
		requireError(t, recorder, http.StatusBadGateway, domain.CodeProviderUnavailable, "Data provider is unavailable. Try again shortly.")
	})

	t.Run("rate limited", func(t *testing.T) {
		quotes := &fakeQuotes{verifyErr: &providers.Failure{
			Kind:       providers.FailureRateLimited,
			Provider:   domain.ProviderFinnhub,
			RetryAfter: 7 * time.Second,
		}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodPost, "/api/watchlist", `{"ticker":"AAPL"}`)
		requireError(t, recorder, http.StatusTooManyRequests, domain.CodeRateLimited, "Too many requests. Try again in 7 seconds.")
	})

	t.Run("missing keys", func(t *testing.T) {
		quotes := &fakeQuotes{verifyErr: &providers.Failure{Kind: providers.FailureMissingKey, Provider: domain.ProviderFinnhub}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodPost, "/api/watchlist", `{"ticker":"AAPL"}`)
		requireError(t, recorder, http.StatusServiceUnavailable, domain.CodeMissingAPIKey, "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.")
	})

	t.Run("unexpected verification failure is internal", func(t *testing.T) {
		quotes := &fakeQuotes{verifyErr: errors.New("token=SECRET-XYZ")}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodPost, "/api/watchlist", `{"ticker":"AAPL"}`)
		requireError(t, recorder, http.StatusInternalServerError, domain.CodeInternalError, "Something went wrong. Try again shortly.")
		if strings.Contains(recorder.Body.String(), "SECRET-XYZ") {
			t.Fatal("response leaked provider text")
		}
		if strings.Contains(env.logs.String(), "SECRET-XYZ") {
			t.Fatalf("logs leaked provider text: %s", env.logs.String())
		}
	})
}

func TestWatchlistRemove(t *testing.T) {
	t.Run("existing", func(t *testing.T) {
		quotes := &fakeQuotes{}
		env := newTestEnv(t, newFakeStore("AAPL", "MSFT"), quotes, completeKeys())
		recorder := env.do(t, http.MethodDelete, "/api/watchlist/aapl", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d", recorder.Code)
		}
		body := decodeWatchlist(t, recorder)
		if len(body.Tickers) != 1 || body.Tickers[0] != "MSFT" {
			t.Fatalf("tickers = %v", body.Tickers)
		}
		quotes.mu.Lock()
		defer quotes.mu.Unlock()
		if len(quotes.forgotten) != 1 || quotes.forgotten[0] != "AAPL" {
			t.Fatalf("forgotten = %v", quotes.forgotten)
		}
	})

	t.Run("missing", func(t *testing.T) {
		env := newTestEnv(t, newFakeStore("MSFT"), &fakeQuotes{}, completeKeys())
		recorder := env.do(t, http.MethodDelete, "/api/watchlist/AAPL", "")
		requireError(t, recorder, http.StatusNotFound, domain.CodeNotInWatchlist, "AAPL is not in your watchlist.")
		if len(env.store.ticks()) != 1 {
			t.Fatal("failed removal changed the store")
		}
	})

	t.Run("malformed path is INVALID_TICKER", func(t *testing.T) {
		env := newTestEnv(t, newFakeStore("AAPL"), &fakeQuotes{}, completeKeys())
		recorder := env.do(t, http.MethodDelete, "/api/watchlist/AA-PL", "")
		requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidTicker, "Enter a valid ticker, e.g. AAPL.")
	})
}

func TestQuotes(t *testing.T) {
	timestamp := time.Date(2026, 11, 16, 14, 30, 0, 0, time.UTC)
	percent := 0.66

	t.Run("happy path shape", func(t *testing.T) {
		quotes := &fakeQuotes{batch: providers.Batch{Quotes: []domain.Quote{{
			Ticker:        "AAPL",
			Price:         190.12,
			Change:        1.24,
			ChangePercent: &percent,
			Direction:     domain.DirectionUp,
			Provider:      domain.ProviderFinnhub,
			Timestamp:     timestamp,
			Stale:         false,
		}}}}
		env := newTestEnv(t, newFakeStore("AAPL"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d", recorder.Code)
		}

		var body struct {
			Quotes []map[string]any `json:"quotes"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Quotes) != 1 {
			t.Fatalf("quotes = %d", len(body.Quotes))
		}
		quote := body.Quotes[0]
		expected := map[string]any{
			"ticker":        "AAPL",
			"price":         190.12,
			"change":        1.24,
			"changePercent": 0.66,
			"direction":     "up",
			"provider":      "Finnhub",
			"timestamp":     "2026-11-16T14:30:00Z",
			"stale":         false,
		}
		for key, want := range expected {
			if quote[key] != want {
				t.Fatalf("%s = %#v, want %#v", key, quote[key], want)
			}
		}
	})

	t.Run("null percent", func(t *testing.T) {
		quotes := &fakeQuotes{batch: providers.Batch{Quotes: []domain.Quote{{
			Ticker:    "AAPL",
			Price:     50,
			Direction: domain.DirectionUnchanged,
			Provider:  domain.ProviderTwelveData,
			Timestamp: timestamp,
		}}}}
		env := newTestEnv(t, newFakeStore("AAPL"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		if !strings.Contains(recorder.Body.String(), `"changePercent":null`) {
			t.Fatalf("body = %q", recorder.Body.String())
		}
	})

	t.Run("empty list", func(t *testing.T) {
		quotes := &fakeQuotes{}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		if recorder.Code != http.StatusOK || recorder.Body.String() != `{"quotes":[]}` {
			t.Fatalf("status/body = %d %q", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("totally rate limited", func(t *testing.T) {
		retry := 5 * time.Second
		quotes := &fakeQuotes{batch: providers.Batch{Quotes: []domain.Quote{}, RateLimitedAfter: &retry}}
		env := newTestEnv(t, newFakeStore("AAPL"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		requireError(t, recorder, http.StatusTooManyRequests, domain.CodeRateLimited, "Too many requests. Try again in 5 seconds.")
		if got := recorder.Header().Get("Retry-After"); got != "5" {
			t.Fatalf("Retry-After = %q", got)
		}
	})

	t.Run("empty watchlist without keys still names the missing keys", func(t *testing.T) {
		env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, Keys{})
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		requireError(t, recorder, http.StatusServiceUnavailable, domain.CodeMissingAPIKey, "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.")
	})

	t.Run("partial rate limit is served", func(t *testing.T) {
		retry := 5 * time.Second
		quotes := &fakeQuotes{batch: providers.Batch{
			Quotes:           []domain.Quote{{Ticker: "AAPL", Price: 1, Timestamp: timestamp, Direction: domain.DirectionUnchanged, Provider: domain.ProviderFinnhub}},
			RateLimitedAfter: &retry,
		}}
		env := newTestEnv(t, newFakeStore("AAPL", "MSFT"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d", recorder.Code)
		}
	})

	t.Run("wholly unavailable batch maps to 502", func(t *testing.T) {
		quotes := &fakeQuotes{batch: providers.Batch{Quotes: []domain.Quote{}, Unavailable: true}}
		env := newTestEnv(t, newFakeStore("AAPL"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		requireError(t, recorder, http.StatusBadGateway, domain.CodeProviderUnavailable, "Data provider is unavailable. Try again shortly.")
	})

	t.Run("credential failure maps to missing key", func(t *testing.T) {
		quotes := &fakeQuotes{batch: providers.Batch{
			Quotes:            []domain.Quote{},
			Unavailable:       true,
			CredentialFailure: true,
		}}
		env := newTestEnv(t, newFakeStore("AAPL", "MSFT"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		requireError(t, recorder, http.StatusServiceUnavailable, domain.CodeMissingAPIKey,
			"Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.")
	})

	t.Run("a served quote keeps a partial credential batch at 200", func(t *testing.T) {
		quotes := &fakeQuotes{batch: providers.Batch{
			Quotes: []domain.Quote{{
				Ticker:    "AAPL",
				Price:     190.12,
				Direction: domain.DirectionUnchanged,
				Provider:  domain.ProviderFinnhub,
				Timestamp: timestamp,
			}},
			Unavailable:       true,
			CredentialFailure: true,
		}}
		env := newTestEnv(t, newFakeStore("AAPL", "MSFT"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d (%q)", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("unavailable with an empty watchlist stays 200", func(t *testing.T) {
		quotes := &fakeQuotes{batch: providers.Batch{Quotes: []domain.Quote{}, Unavailable: true}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		if recorder.Code != http.StatusOK || recorder.Body.String() != `{"quotes":[]}` {
			t.Fatalf("status/body = %d %q", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("missing either key", func(t *testing.T) {
		for _, keys := range []Keys{
			{Finnhub: true},
			{TwelveData: true},
			{},
		} {
			env := newTestEnv(t, newFakeStore("AAPL"), &fakeQuotes{}, keys)
			recorder := env.do(t, http.MethodGet, "/api/quotes", "")
			requireError(t, recorder, http.StatusServiceUnavailable, domain.CodeMissingAPIKey, "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.")
		}
	})

	t.Run("provider failure omission is visible in shape only", func(t *testing.T) {
		quotes := &fakeQuotes{batch: providers.Batch{Quotes: []domain.Quote{}}}
		env := newTestEnv(t, newFakeStore("AAPL", "ZZZZ"), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/quotes", "")
		if recorder.Code != http.StatusOK || recorder.Body.String() != `{"quotes":[]}` {
			t.Fatalf("status/body = %d %q", recorder.Code, recorder.Body.String())
		}
	})
}

func TestChart(t *testing.T) {
	points := []domain.PricePoint{
		{Date: "2026-11-13", Close: 190.1},
		{Date: "2026-11-14", Close: 191.2},
	}

	t.Run("happy path defaults to 1M", func(t *testing.T) {
		quotes := &fakeQuotes{series: points}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/chart/aapl", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d (%q)", recorder.Code, recorder.Body.String())
		}
		var body chartBody
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Ticker != "AAPL" || body.Range != "1M" || body.Source != "Twelve Data" {
			t.Fatalf("body = %+v", body)
		}
		if len(body.Points) != 2 || body.Points[0].Date != "2026-11-13" {
			t.Fatalf("points = %+v", body.Points)
		}
		quotes.mu.Lock()
		rangeValue := quotes.seriesRange
		quotes.mu.Unlock()
		if rangeValue != domain.Range1M {
			t.Fatalf("range = %q", rangeValue)
		}
	})

	t.Run("explicit ranges", func(t *testing.T) {
		for _, test := range []struct {
			query string
			want  domain.PriceRange
		}{
			{query: "1M", want: domain.Range1M},
			{query: "3M", want: domain.Range3M},
			{query: "1Y", want: domain.Range1Y},
		} {
			quotes := &fakeQuotes{series: points}
			env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
			recorder := env.do(t, http.MethodGet, "/api/chart/AAPL?range="+test.query, "")
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d", test.query, recorder.Code)
			}
			quotes.mu.Lock()
			got := quotes.seriesRange
			quotes.mu.Unlock()
			if got != test.want {
				t.Fatalf("range = %q, want %q", got, test.want)
			}
		}
	})

	t.Run("invalid ranges", func(t *testing.T) {
		for _, query := range []string{"", "1m", "2M", "1Y%20", "1W"} {
			quotes := &fakeQuotes{series: points}
			env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
			recorder := env.do(t, http.MethodGet, "/api/chart/AAPL?range="+query, "")
			requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidRange, "Range must be 1M, 3M or 1Y.")
			if quotes.seriesCalls != 0 {
				t.Fatalf("range %q reached the provider", query)
			}
		}
	})

	t.Run("unknown symbol", func(t *testing.T) {
		quotes := &fakeQuotes{seriesErr: &providers.Failure{Kind: providers.FailureUnknownSymbol, Provider: domain.ProviderTwelveData}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/chart/ZZZZ", "")
		requireError(t, recorder, http.StatusNotFound, domain.CodeUnknownSymbol, "Symbol not found.")
	})

	t.Run("provider unavailable", func(t *testing.T) {
		quotes := &fakeQuotes{seriesErr: &providers.Failure{Kind: providers.FailureUnavailable, Provider: domain.ProviderTwelveData}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/chart/AAPL", "")
		requireError(t, recorder, http.StatusBadGateway, domain.CodeChartUnavailable, "Chart unavailable")
	})

	t.Run("rate limited", func(t *testing.T) {
		quotes := &fakeQuotes{seriesErr: &providers.Failure{
			Kind:       providers.FailureRateLimited,
			Provider:   domain.ProviderTwelveData,
			RetryAfter: 2 * time.Second,
		}}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/chart/AAPL", "")
		requireError(t, recorder, http.StatusTooManyRequests, domain.CodeRateLimited, "Too many requests. Try again in 2 seconds.")
	})

	t.Run("missing keys", func(t *testing.T) {
		quotes := &fakeQuotes{series: points}
		env := newTestEnv(t, newFakeStore(), quotes, Keys{Finnhub: true})
		recorder := env.do(t, http.MethodGet, "/api/chart/AAPL", "")
		requireError(t, recorder, http.StatusServiceUnavailable, domain.CodeMissingAPIKey, "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.")
		if quotes.seriesCalls != 0 {
			t.Fatal("missing keys reached the provider")
		}
	})

	t.Run("invalid ticker", func(t *testing.T) {
		env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/chart/1AAPL", "")
		requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidTicker, "Enter a valid ticker, e.g. AAPL.")
	})

	t.Run("ticker need not be listed", func(t *testing.T) {
		quotes := &fakeQuotes{series: points}
		env := newTestEnv(t, newFakeStore(), quotes, completeKeys())
		recorder := env.do(t, http.MethodGet, "/api/chart/MSFT", "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d", recorder.Code)
		}
	})
}

func TestHealthz(t *testing.T) {
	store := newFakeStore("AAPL")
	quotes := &fakeQuotes{}
	env := newTestEnv(t, store, quotes, Keys{})

	recorder := env.do(t, http.MethodGet, "/healthz", "")
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"status":"ok"}` {
		t.Fatalf("status/body = %d %q", recorder.Code, recorder.Body.String())
	}
	if quotes.quoteCount() != 0 || quotes.verifyCount() != 0 || quotes.seriesCalls != 0 {
		t.Fatal("health touched a provider")
	}
	if store.listCalls != 0 {
		t.Fatal("health read the watchlist")
	}
}

func TestAPINotFoundEnvelope(t *testing.T) {
	env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
	for _, request := range []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/api/unknown"},
		{method: http.MethodGet, target: "/api/watchlist/AAPL/extra"},
		{method: http.MethodPost, target: "/api/quotes"},
		{method: http.MethodGet, target: "/api"},
	} {
		recorder := env.do(t, request.method, request.target, "")
		requireError(t, recorder, http.StatusNotFound, domain.CodeNotFound, "Not found.")
	}
	if env.spaCalls != 0 {
		t.Fatal("API 404 fell through to the SPA")
	}
}

func TestSPAFallback(t *testing.T) {
	env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
	for _, target := range []string{"/", "/chart/AAPL", "/some/deep/link?x=1"} {
		recorder := env.do(t, http.MethodGet, target, "")
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", target, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), "spa shell") {
			t.Fatalf("%s body = %q", target, recorder.Body.String())
		}
	}
}

func TestSecurityHeadersAndRequestID(t *testing.T) {
	env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
	recorder := env.do(t, http.MethodGet, "/healthz", "")
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff = %q", got)
	}
	if got := recorder.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("referrer = %q", got)
	}
	policy := recorder.Header().Get("Content-Security-Policy")
	for _, fragment := range []string{"default-src 'self'", "img-src 'self' data:", "script-src 'self'", "style-src 'self'"} {
		if !strings.Contains(policy, fragment) {
			t.Fatalf("CSP %q missing %q", policy, fragment)
		}
	}
	if strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("CSP unexpectedly allows inline content: %q", policy)
	}
	if recorder.Header().Get("X-Request-Id") == "" {
		t.Fatal("missing request id")
	}
}

func TestLogsDoNotContainQueryStrings(t *testing.T) {
	env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
	env.do(t, http.MethodGet, "/api/quotes?token=SECRET-XYZ", "")
	if strings.Contains(env.logs.String(), "SECRET-XYZ") {
		t.Fatalf("logs leaked a query string: %s", env.logs.String())
	}
	if !strings.Contains(env.logs.String(), "/api/quotes") {
		t.Fatalf("access log missing: %s", env.logs.String())
	}
}

func TestPanicRecovery(t *testing.T) {
	quotes := &fakeQuotes{panicOnQuotes: "SECRET-PANIC"}
	env := newTestEnv(t, newFakeStore("AAPL"), quotes, completeKeys())

	recorder := env.do(t, http.MethodGet, "/api/quotes", "")
	requireError(t, recorder, http.StatusInternalServerError, domain.CodeInternalError, "Something went wrong. Try again shortly.")
	if strings.Contains(recorder.Body.String(), "SECRET-PANIC") {
		t.Fatal("panic value leaked into the response")
	}
	if strings.Contains(env.logs.String(), "SECRET-PANIC") {
		t.Fatal("panic value leaked into the log")
	}

	health := env.do(t, http.MethodGet, "/healthz", "")
	if health.Code != http.StatusOK {
		t.Fatalf("server did not recover: %d", health.Code)
	}
}

func TestOversizedBodyIs413(t *testing.T) {
	env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
	body := `{"ticker":"` + strings.Repeat("A", 5000) + `"}`
	recorder := env.do(t, http.MethodPost, "/api/watchlist", body)
	requireError(t, recorder, http.StatusRequestEntityTooLarge, domain.CodeRequestTooLarge, "Request body is too large.")
}

func TestWriteJSONMarshalFailureKeepsEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, http.StatusOK, math.NaN())
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	body := decodeError(t, recorder)
	if body.Code != string(domain.CodeInternalError) || body.Error != domain.DefaultMessage(domain.CodeInternalError) {
		t.Fatalf("body = %+v", body)
	}
}

func TestDeleteMalformedTickerIs400(t *testing.T) {
	env := newTestEnv(t, newFakeStore("AAPL"), &fakeQuotes{}, completeKeys())
	for _, target := range []string{
		"/api/watchlist/AA-PL",
		"/api/watchlist/" + strings.Repeat("A", 5000),
	} {
		recorder := env.do(t, http.MethodDelete, target, "")
		requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidTicker, "Enter a valid ticker, e.g. AAPL.")
	}
}

func TestEveryAPIRouteIsPublicOrGuarded(t *testing.T) {
	// Every /api pattern must appear here, public or guarded. A new route that is
	// not added deliberately fails this test, so the public/guarded split stays a
	// reviewed decision rather than a default.
	wantPublic := map[string]bool{
		"/api/auth/status":         true,
		"/api/auth/setup":          true,
		"/api/auth/2fa/activate":   true,
		"/api/auth/login":          true,
		"/api/auth/login/totp":     true,
		"/api/auth/login/recovery": true,
		"/api/auth/logout":         true,
		"/api/auth/me":             false,
		"/api/watchlist":           false,
		"/api/watchlist/{ticker}":  false,
		"/api/quotes":              false,
		"/api/chart/{ticker}":      false,
		"/api":                     true,
		"/api/":                    true,
	}
	seen := map[string]bool{}
	for _, route := range routeDefinitions {
		if !strings.HasPrefix(route.pattern, "/api") {
			continue
		}
		seen[route.pattern] = true
		want, classified := wantPublic[route.pattern]
		if !classified {
			t.Fatalf("%s %q is not classified in wantPublic; decide whether it is public or guarded",
				route.method, route.pattern)
		}
		if route.public != want {
			t.Fatalf("%s public = %v, want %v", route.pattern, route.public, want)
		}
	}
	for pattern := range wantPublic {
		if !seen[pattern] {
			t.Fatalf("classified route %s is not registered", pattern)
		}
	}

	env := newTestEnvWithAuth(t, newFakeStore("AAPL"), &fakeQuotes{}, completeKeys(), denyAllAuth{})
	for _, route := range routeDefinitions {
		if route.public || !strings.HasPrefix(route.pattern, "/api") {
			continue
		}
		method := route.method
		if method == "" {
			method = http.MethodGet
		}
		target := strings.ReplaceAll(route.pattern, "{ticker}", "AAPL")
		recorder := env.do(t, method, target, "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", method, target, recorder.Code)
		}
	}
}

func TestAllowedHostsOverride(t *testing.T) {
	handler := NewRouter(Deps{
		Watchlist:    newFakeStore(),
		Quotes:       &fakeQuotes{},
		Auth:         allowAllAuth{},
		Keys:         completeKeys(),
		AllowedHosts: []string{"example.test"},
	}, nil)

	allowed := httptest.NewRecorder()
	handler.ServeHTTP(allowed, httptest.NewRequest(http.MethodGet, "http://example.test/healthz", nil))
	if allowed.Code != http.StatusOK {
		t.Fatalf("allowed host status = %d", allowed.Code)
	}
	rejected := httptest.NewRecorder()
	handler.ServeHTTP(rejected, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3000/healthz", nil))
	if rejected.Code != http.StatusForbidden {
		t.Fatalf("unlisted host status = %d", rejected.Code)
	}
}

func TestCanceledQuoteBatchIsNotACleanEmpty200(t *testing.T) {
	quotes := &fakeQuotes{batch: providers.Batch{Canceled: true}}
	env := newTestEnv(t, newFakeStore("AAPL"), quotes, completeKeys())
	recorder := env.do(t, http.MethodGet, "/api/quotes", "")
	requireError(t, recorder, http.StatusBadGateway, domain.CodeProviderUnavailable, "Data provider is unavailable. Try again shortly.")
}

func TestStatusRecorderUnwrap(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &statusRecorder{ResponseWriter: recorder}
	if wrapped.Unwrap() != recorder {
		t.Fatal("Unwrap did not return the real writer")
	}
}

func TestRecoveryDoesNotAppendAfterPartialWrite(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := recoverMiddleware(logger, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "partial")
		panic("late panic")
	}))
	recorder := httptest.NewRecorder()
	wrapped := &statusRecorder{ResponseWriter: recorder}
	handler.ServeHTTP(wrapped, httptest.NewRequest(http.MethodGet, "/api/quotes", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != "partial" {
		t.Fatalf("status = %d body = %q", recorder.Code, recorder.Body.String())
	}
}

func TestCookieExpiryUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)
	h := &handlers{deps: Deps{Now: func() time.Time { return fixed }}}
	recorder := httptest.NewRecorder()
	h.setAuthCookie(recorder, sessionCookieName, "token", "/", 12*time.Hour)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	if want := fixed.Add(12 * time.Hour); !cookies[0].Expires.Equal(want) {
		t.Fatalf("expires = %s, want %s", cookies[0].Expires, want)
	}
}

func TestAuthOversizedBodyIs413(t *testing.T) {
	env := newTestEnv(t, newFakeStore(), &fakeQuotes{}, completeKeys())
	body := `{"username":"owner","password":"` + strings.Repeat("A", 5000) + `"}`
	recorder := env.do(t, http.MethodPost, "/api/auth/setup", body)
	requireError(t, recorder, http.StatusRequestEntityTooLarge, domain.CodeRequestTooLarge, "Request body is too large.")
}
