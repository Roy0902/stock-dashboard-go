package providers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"stock-dashboard/server/internal/domain"
)

type fakeSource struct {
	mu         sync.Mutex
	quoteCalls []domain.Ticker
	seriesCall int
	seriesSize int
	quoteFn    func(domain.Ticker) (domain.Observation, error)
	seriesFn   func(domain.Ticker, int) ([]domain.PricePoint, error)
}

func (f *fakeSource) Quote(_ context.Context, ticker domain.Ticker) (domain.Observation, error) {
	f.mu.Lock()
	f.quoteCalls = append(f.quoteCalls, ticker)
	f.mu.Unlock()
	if f.quoteFn == nil {
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}
	return f.quoteFn(ticker)
}

func (f *fakeSource) DailySeries(_ context.Context, ticker domain.Ticker, points int) ([]domain.PricePoint, error) {
	f.mu.Lock()
	f.seriesCall++
	f.seriesSize = points
	f.mu.Unlock()
	if f.seriesFn == nil {
		return nil, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderTwelveData}
	}
	return f.seriesFn(ticker, points)
}

func (f *fakeSource) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.quoteCalls)
}

func successfulObservation(ticker domain.Ticker, price float64, previous float64, timestamp time.Time, provider domain.ProviderAttribution) domain.Observation {
	return domain.Observation{
		Ticker:        ticker,
		Price:         price,
		PreviousClose: &previous,
		Timestamp:     timestamp,
		Provider:      provider,
	}
}

func newTestService(primary, fallback *fakeSource, clock *fakeClock) *Service {
	return NewService(ServiceOptions{
		Finnhub:    primary,
		TwelveData: fallback,
		Chart:      fallback,
		Now:        clock.Now,
		Workers:    8,
		CacheTTL:   time.Minute,
	})
}

func TestServiceFallsBackToTwelveData(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}}
	fallback := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190.12, 188.88, clock.Now(), domain.ProviderTwelveData), nil
	}}
	service := newTestService(primary, fallback, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
	if len(batch.Quotes) != 1 {
		t.Fatalf("quotes = %d", len(batch.Quotes))
	}
	if batch.Quotes[0].Provider != domain.ProviderTwelveData {
		t.Fatalf("provider = %q", batch.Quotes[0].Provider)
	}
	if batch.Quotes[0].Change != 1.24 {
		t.Fatalf("change = %v", batch.Quotes[0].Change)
	}
	if primary.calls() != 1 || fallback.calls() != 1 {
		t.Fatalf("calls primary=%d fallback=%d", primary.calls(), fallback.calls())
	}
	if batch.RateLimitedAfter != nil {
		t.Fatal("unexpected rate limit")
	}
}

func TestServiceCacheHitDoesNotCallProviders(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	fallback := &fakeSource{}
	service := newTestService(primary, fallback, clock)
	tickers := []domain.Ticker{"AAPL"}

	first := service.Quotes(context.Background(), tickers)
	second := service.Quotes(context.Background(), tickers)
	if len(first.Quotes) != 1 || len(second.Quotes) != 1 {
		t.Fatalf("quotes = %d/%d", len(first.Quotes), len(second.Quotes))
	}
	if primary.calls() != 1 || fallback.calls() != 0 {
		t.Fatalf("cache miss: primary=%d fallback=%d", primary.calls(), fallback.calls())
	}

	clock.Advance(30 * time.Second)
	service.Quotes(context.Background(), tickers)
	if primary.calls() != 1 {
		t.Fatalf("cache expired early: calls=%d", primary.calls())
	}

	clock.Advance(31 * time.Second)
	service.Quotes(context.Background(), tickers)
	if primary.calls() != 2 {
		t.Fatalf("cache did not expire after 61s: calls=%d", primary.calls())
	}
}

func TestServiceServesLastKnownWithinFifteenMinutes(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	fallback := &fakeSource{}
	service := newTestService(primary, fallback, clock)

	service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
	clock.Advance(61 * time.Second)

	primary.quoteFn = func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}
	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
	if len(batch.Quotes) != 1 {
		t.Fatalf("last known not served: %d quotes", len(batch.Quotes))
	}
	if batch.Quotes[0].Stale {
		t.Fatal("61-second-old quote reported stale")
	}
	if batch.Quotes[0].Provider != domain.ProviderFinnhub {
		t.Fatalf("provider = %q", batch.Quotes[0].Provider)
	}
	if primary.calls() != 2 {
		t.Fatalf("primary calls = %d", primary.calls())
	}
}

func TestServiceDropsStaleLastKnown(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190, 188, clock.Now().Add(-20*time.Minute), domain.ProviderFinnhub), nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	first := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
	if len(first.Quotes) != 1 || !first.Quotes[0].Stale {
		t.Fatalf("first quote = %+v", first.Quotes)
	}

	clock.Advance(61 * time.Second)
	primary.quoteFn = func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}
	second := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
	if len(second.Quotes) != 0 {
		t.Fatalf("stale last known was served: %+v", second.Quotes)
	}
}

func TestServiceUnknownShortCircuitsFallback(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureUnknownSymbol, Provider: domain.ProviderFinnhub}
	}}
	fallback := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 1, 1, clock.Now(), domain.ProviderTwelveData), nil
	}}
	service := newTestService(primary, fallback, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"ZZZZ"})
	if len(batch.Quotes) != 0 {
		t.Fatalf("unknown symbol produced a quote: %+v", batch.Quotes)
	}
	if fallback.calls() != 0 {
		t.Fatalf("fallback called for a definitive unknown: %d", fallback.calls())
	}
}

func TestServiceRateLimitAggregation(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureRateLimited, Provider: domain.ProviderFinnhub, RetryAfter: 5 * time.Second}
	}}
	fallback := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureRateLimited, Provider: domain.ProviderTwelveData, RetryAfter: 3 * time.Second}
	}}
	service := newTestService(primary, fallback, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL", "MSFT"})
	if len(batch.Quotes) != 0 {
		t.Fatalf("quotes = %d", len(batch.Quotes))
	}
	if batch.RateLimitedAfter == nil {
		t.Fatal("RateLimitedAfter not set")
	}
	if *batch.RateLimitedAfter != 3*time.Second {
		t.Fatalf("RateLimitedAfter = %v, want 3s", *batch.RateLimitedAfter)
	}
}

func TestServicePartialRateLimitIsNotAFailure(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		if ticker == "AAPL" {
			return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
		}
		return domain.Observation{}, &Failure{Kind: FailureRateLimited, Provider: domain.ProviderFinnhub, RetryAfter: 5 * time.Second}
	}}
	fallback := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureRateLimited, Provider: domain.ProviderTwelveData, RetryAfter: 3 * time.Second}
	}}
	service := newTestService(primary, fallback, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL", "MSFT"})
	if len(batch.Quotes) != 1 {
		t.Fatalf("quotes = %d", len(batch.Quotes))
	}
	if batch.RateLimitedAfter != nil {
		t.Fatalf("partial batch reported rate limited: %v", *batch.RateLimitedAfter)
	}
}

func TestServiceIsolatesPerTickerFailures(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		if ticker == "AAPL" {
			return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
		}
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}}
	fallback := &fakeSource{}
	service := newTestService(primary, fallback, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL", "MSFT"})
	if len(batch.Quotes) != 1 || batch.Quotes[0].Ticker != "AAPL" {
		t.Fatalf("quotes = %+v", batch.Quotes)
	}
	if batch.RateLimitedAfter != nil {
		t.Fatal("unexpected rate limit")
	}
}

func TestServiceMarksWhollyUnavailableBatch(t *testing.T) {
	clock := newFakeClock()
	unavailable := func() (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}
	primary := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return unavailable()
	}}
	fallback := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return unavailable()
	}}
	service := newTestService(primary, fallback, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
	if len(batch.Quotes) != 0 {
		t.Fatalf("quotes = %+v", batch.Quotes)
	}
	if !batch.Unavailable || batch.RateLimitedAfter != nil {
		t.Fatalf("batch = %+v", batch)
	}
}

func TestServiceDoesNotMarkPartiallyServedBatchUnavailable(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		if ticker == "AAPL" {
			return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
		}
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL", "MSFT"})
	if len(batch.Quotes) != 1 || batch.Unavailable {
		t.Fatalf("batch = %+v", batch)
	}
}

func TestServiceMarksWhollyCredentialFailureBatch(t *testing.T) {
	clock := newFakeClock()
	for _, kind := range []FailureKind{FailureAuth, FailureMissingKey} {
		t.Run(string(kind), func(t *testing.T) {
			failing := func(provider domain.ProviderAttribution) *fakeSource {
				return &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
					return domain.Observation{}, &Failure{Kind: kind, Provider: provider}
				}}
			}
			service := newTestService(failing(domain.ProviderFinnhub), failing(domain.ProviderTwelveData), clock)

			batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
			if len(batch.Quotes) != 0 {
				t.Fatalf("quotes = %+v", batch.Quotes)
			}
			if !batch.CredentialFailure {
				t.Fatalf("batch = %+v, want credential failure", batch)
			}
			if batch.RateLimitedAfter != nil || batch.Canceled {
				t.Fatalf("batch = %+v, credential failure must not look throttled or canceled", batch)
			}
		})
	}
}

func TestServiceDoesNotMarkServedBatchCredentialFailure(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		if ticker == "AAPL" {
			return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
		}
		return domain.Observation{}, &Failure{Kind: FailureAuth, Provider: domain.ProviderFinnhub}
	}}
	fallback := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureAuth, Provider: domain.ProviderTwelveData}
	}}
	service := newTestService(primary, fallback, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL", "MSFT"})
	if len(batch.Quotes) != 1 || batch.Quotes[0].Ticker != "AAPL" {
		t.Fatalf("quotes = %+v", batch.Quotes)
	}
	if batch.CredentialFailure || batch.Unavailable {
		t.Fatalf("partial batch = %+v, want a plain 200", batch)
	}
}

func TestServiceCredentialSignalRequiresEveryFailureToBeCredential(t *testing.T) {
	clock := newFakeClock()
	tests := []struct {
		name     string
		primary  *Failure
		fallback *Failure
	}{
		{name: "auth then unavailable", primary: &Failure{Kind: FailureAuth, Provider: domain.ProviderFinnhub}, fallback: &Failure{Kind: FailureUnavailable, Provider: domain.ProviderTwelveData}},
		{name: "auth then rate limit", primary: &Failure{Kind: FailureAuth, Provider: domain.ProviderFinnhub}, fallback: &Failure{Kind: FailureRateLimited, Provider: domain.ProviderTwelveData, RetryAfter: time.Second}},
		{name: "unavailable then missing key", primary: &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}, fallback: &Failure{Kind: FailureMissingKey, Provider: domain.ProviderTwelveData}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			primary := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
				return domain.Observation{}, test.primary
			}}
			fallback := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
				return domain.Observation{}, test.fallback
			}}
			service := newTestService(primary, fallback, clock)

			batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
			if batch.CredentialFailure {
				t.Fatalf("batch = %+v, mixed failure reported as credential", batch)
			}
		})
	}
}

func TestServiceSerialisesConcurrentWorkForOneTicker(t *testing.T) {
	clock := newFakeClock()
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var calls atomic.Int32

	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
		}()
	}

	<-entered
	select {
	case <-entered:
		t.Fatal("two concurrent requests both reached the provider")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	group.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

func TestServiceForgetDropsCacheAndLastKnown(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	if batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"}); len(batch.Quotes) != 1 {
		t.Fatalf("prime batch = %+v", batch)
	}

	// Swap in failing adapters: without Forget, both the 60 s cache and the 15 min
	// last-known observation would still answer.
	service.primary = &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}}
	service.fallback = &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderTwelveData}
	}}

	if batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"}); len(batch.Quotes) != 1 {
		t.Fatalf("warm cache should still serve before Forget: %+v", batch)
	}

	service.Forget("AAPL")
	if batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"}); len(batch.Quotes) != 0 || !batch.Unavailable {
		t.Fatalf("after forget batch = %+v", batch)
	}
}

func TestServicePreservesInputOrder(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		price := 1.0
		if ticker == "MSFT" {
			price = 2
		}
		observation := successfulObservation(ticker, price, price, clock.Now(), domain.ProviderFinnhub)
		return observation, nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"MSFT", "AAPL"})
	if len(batch.Quotes) != 2 || batch.Quotes[0].Ticker != "MSFT" || batch.Quotes[1].Ticker != "AAPL" {
		t.Fatalf("order = %+v", batch.Quotes)
	}
}

func TestServiceDeduplicatesBatchCalls(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL", "AAPL", "MSFT"})
	if len(batch.Quotes) != 2 {
		t.Fatalf("quotes = %d", len(batch.Quotes))
	}
	if primary.calls() != 2 {
		t.Fatalf("calls = %d, want 2", primary.calls())
	}
}

func TestServiceEmptyBatchDoesNotCallProviders(t *testing.T) {
	primary := &fakeSource{}
	fallback := &fakeSource{}
	service := newTestService(primary, fallback, newFakeClock())

	batch := service.Quotes(context.Background(), nil)
	if len(batch.Quotes) != 0 || batch.RateLimitedAfter != nil {
		t.Fatalf("batch = %+v", batch)
	}
	if primary.calls() != 0 || fallback.calls() != 0 {
		t.Fatal("empty batch called a provider")
	}
}

func TestServiceVerifySymbol(t *testing.T) {
	clock := newFakeClock()
	unknown := &Failure{Kind: FailureUnknownSymbol, Provider: domain.ProviderFinnhub}
	missing := &Failure{Kind: FailureMissingKey, Provider: domain.ProviderFinnhub}
	unavailable := &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	rate := &Failure{Kind: FailureRateLimited, Provider: domain.ProviderFinnhub, RetryAfter: 9 * time.Second}

	tests := []struct {
		name       string
		primary    error
		fallback   error
		wantKind   FailureKind
		wantOK     bool
		fallbackOK bool
	}{
		{name: "primary succeeds", primary: nil, wantOK: true, fallbackOK: false},
		{name: "primary unknown short-circuits", primary: unknown, wantKind: FailureUnknownSymbol, fallbackOK: false},
		{name: "fallback succeeds", primary: unavailable, wantOK: true, fallbackOK: true},
		{name: "fallback unknown", primary: unavailable, fallback: unknown, wantKind: FailureUnknownSymbol, fallbackOK: true},
		{name: "both missing keys", primary: missing, fallback: missing, wantKind: FailureMissingKey, fallbackOK: true},
		{name: "both unavailable", primary: unavailable, fallback: unavailable, wantKind: FailureUnavailable, fallbackOK: true},
		{name: "rate limit wins", primary: rate, fallback: unavailable, wantKind: FailureRateLimited, fallbackOK: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
				if test.primary != nil {
					return domain.Observation{}, test.primary
				}
				return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
			}}
			fallback := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
				if test.fallback != nil {
					return domain.Observation{}, test.fallback
				}
				return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderTwelveData), nil
			}}
			service := newTestService(primary, fallback, clock)

			err := service.VerifySymbol(context.Background(), "AAPL")
			if test.wantOK {
				if err != nil {
					t.Fatalf("error = %v", err)
				}
			} else {
				var failure *Failure
				if !errors.As(err, &failure) || failure.Kind != test.wantKind {
					t.Fatalf("error = %v, want kind %q", err, test.wantKind)
				}
			}
			if called := fallback.calls() == 1; called != test.fallbackOK {
				t.Fatalf("fallback called = %v, want %v", called, test.fallbackOK)
			}
		})
	}
}

func TestServiceVerifySymbolAggregateAttribution(t *testing.T) {
	clock := newFakeClock()
	failing := func(provider domain.ProviderAttribution, kind FailureKind) *fakeSource {
		return &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
			return domain.Observation{}, &Failure{Kind: kind, Provider: provider}
		}}
	}
	plainError := &fakeSource{quoteFn: func(domain.Ticker) (domain.Observation, error) {
		return domain.Observation{}, errors.New("unexpected adapter error")
	}}

	tests := []struct {
		name         string
		primary      QuoteSource
		fallback     QuoteSource
		wantKind     FailureKind
		wantProvider domain.ProviderAttribution
	}{
		{
			name:         "twelve data only unavailable",
			fallback:     failing(domain.ProviderTwelveData, FailureUnavailable),
			wantKind:     FailureUnavailable,
			wantProvider: domain.ProviderTwelveData,
		},
		{
			name:         "twelve data only missing key",
			fallback:     failing(domain.ProviderTwelveData, FailureMissingKey),
			wantKind:     FailureMissingKey,
			wantProvider: domain.ProviderTwelveData,
		},
		{
			name:         "twelve data only auth",
			fallback:     failing(domain.ProviderTwelveData, FailureAuth),
			wantKind:     FailureAuth,
			wantProvider: domain.ProviderTwelveData,
		},
		{
			name:         "twelve data only non-failure error",
			fallback:     plainError,
			wantKind:     FailureUnavailable,
			wantProvider: domain.ProviderTwelveData,
		},
		{
			name:         "finnhub only unavailable",
			primary:      failing(domain.ProviderFinnhub, FailureUnavailable),
			wantKind:     FailureUnavailable,
			wantProvider: domain.ProviderFinnhub,
		},
		{
			name:         "both unavailable names the primary",
			primary:      failing(domain.ProviderFinnhub, FailureUnavailable),
			fallback:     failing(domain.ProviderTwelveData, FailureUnavailable),
			wantKind:     FailureUnavailable,
			wantProvider: domain.ProviderFinnhub,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(ServiceOptions{
				Finnhub:    test.primary,
				TwelveData: test.fallback,
				Now:        clock.Now,
				Workers:    8,
				CacheTTL:   time.Minute,
			})
			err := service.VerifySymbol(context.Background(), "AAPL")
			var failure *Failure
			if !errors.As(err, &failure) {
				t.Fatalf("error = %v, want *Failure", err)
			}
			if failure.Kind != test.wantKind || failure.Provider != test.wantProvider {
				t.Fatalf("failure = %s/%s, want %s/%s",
					failure.Provider, failure.Kind, test.wantProvider, test.wantKind)
			}
		})
	}
}

func TestServiceVerifyCachesObservation(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	if err := service.VerifySymbol(context.Background(), "AAPL"); err != nil {
		t.Fatal(err)
	}
	batch := service.Quotes(context.Background(), []domain.Ticker{"AAPL"})
	if len(batch.Quotes) != 1 {
		t.Fatalf("quotes = %d", len(batch.Quotes))
	}
	if primary.calls() != 1 {
		t.Fatalf("verification observation was not cached: calls=%d", primary.calls())
	}
}

func TestServiceDailySeriesOrdersAndForwardsSize(t *testing.T) {
	clock := newFakeClock()
	fallback := &fakeSource{seriesFn: func(_ domain.Ticker, _ int) ([]domain.PricePoint, error) {
		return []domain.PricePoint{
			{Date: "2026-11-14", Close: 3},
			{Date: "2026-11-12", Close: 1},
			{Date: "2026-11-13", Close: 2},
		}, nil
	}}
	service := newTestService(&fakeSource{}, fallback, clock)

	points, err := service.DailySeries(context.Background(), "AAPL", domain.Range3M)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 || points[0].Date != "2026-11-12" || points[2].Date != "2026-11-14" {
		t.Fatalf("points = %+v", points)
	}
	if fallback.seriesSize != 90 {
		t.Fatalf("output size = %d, want 90", fallback.seriesSize)
	}
}

func TestServiceDailySeriesFailure(t *testing.T) {
	fallback := &fakeSource{seriesFn: func(domain.Ticker, int) ([]domain.PricePoint, error) {
		return nil, &Failure{Kind: FailureUnknownSymbol, Provider: domain.ProviderTwelveData}
	}}
	service := newTestService(&fakeSource{}, fallback, newFakeClock())

	_, err := service.DailySeries(context.Background(), "ZZZZ", domain.Range1M)
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureUnknownSymbol {
		t.Fatalf("error = %v", err)
	}
}

func TestServiceBoundedConcurrency(t *testing.T) {
	clock := newFakeClock()
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32

	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		current := inFlight.Add(1)
		for {
			observed := maxInFlight.Load()
			if current <= observed || maxInFlight.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	tickers := make([]domain.Ticker, 20)
	for index := range tickers {
		tickers[index] = domain.MustTicker("T" + string(rune('A'+index)))
	}
	batch := service.Quotes(context.Background(), tickers)
	if len(batch.Quotes) != len(tickers) {
		t.Fatalf("quotes = %d", len(batch.Quotes))
	}
	if got := maxInFlight.Load(); got > 8 {
		t.Fatalf("max concurrency = %d, want <= 8", got)
	}
	if got := maxInFlight.Load(); got < 2 {
		t.Fatalf("max concurrency = %d; the batch was effectively serial", got)
	}
}

func TestQuotesCanceledContextIsMarked(t *testing.T) {
	clock := newFakeClock()
	primary := &fakeSource{quoteFn: func(ticker domain.Ticker) (domain.Observation, error) {
		return successfulObservation(ticker, 190, 188, clock.Now(), domain.ProviderFinnhub), nil
	}}
	service := newTestService(primary, &fakeSource{}, clock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	batch := service.Quotes(ctx, []domain.Ticker{"AAPL", "MSFT"})
	if len(batch.Quotes) != 0 || !batch.Canceled {
		t.Fatalf("batch = %+v, want canceled with no quotes", batch)
	}
	if primary.calls() != 0 {
		t.Fatalf("provider calls = %d after cancellation", primary.calls())
	}
}
