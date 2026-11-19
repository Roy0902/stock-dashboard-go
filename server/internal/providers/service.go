package providers

import (
	"context"
	"errors"
	"sync"
	"time"

	"stock-dashboard/server/internal/domain"
)

const (
	tickerLockStripes = 64
	fnvOffsetBasis    = 2166136261
	fnvPrime          = 16777619
)

var (
	defaultCacheTTL = DefaultLimits.CacheTTL
	defaultWorkers  = DefaultLimits.Workers
)

// quoteSourcePair keeps a provider's public attribution next to its adapter so
// the fallback order is one ordered slice instead of two index-linked slices.
type quoteSourcePair struct {
	name   domain.ProviderAttribution
	source QuoteSource
}

// sources returns the fallback order once per operation.
func (s *Service) sources() []quoteSourcePair {
	return []quoteSourcePair{
		{name: domain.ProviderFinnhub, source: s.primary},
		{name: domain.ProviderTwelveData, source: s.fallback},
	}
}

// quoteOutcome is one ticker's result. observe fills observation or the failure
// flags; the batch loop replaces a served observation with a derived quote.
type quoteOutcome struct {
	observation *domain.Observation
	quote       *domain.Quote
	refused     bool
	unavailable bool
	canceled    bool
	credential  bool
	retryAfter  time.Duration
}

// ServiceOptions wires the fallback order, chart source and clock.
type ServiceOptions struct {
	Finnhub    QuoteSource
	TwelveData QuoteSource
	Chart      SeriesSource
	Now        func() time.Time
	Workers    int
	CacheTTL   time.Duration
}

// Service owns the 60-second cache, last-known observations and the Finnhub ->
// Twelve Data fallback policy.
type Service struct {
	primary  QuoteSource
	fallback QuoteSource
	chart    SeriesSource
	now      func() time.Time
	workers  int
	cacheTTL time.Duration

	mu        sync.Mutex
	cache     map[domain.Ticker]cachedObservation
	lastKnown map[domain.Ticker]domain.Observation

	// stripes serialise work for the same ticker so two concurrent refreshes cannot
	// both miss the cache and spend two upstream calls (spec §7).
	stripes [tickerLockStripes]sync.Mutex
}

type cachedObservation struct {
	observation domain.Observation
	fetchedAt   time.Time
}

// NewService composes the adapters. Missing adapters simply mean those steps fail.
func NewService(options ServiceOptions) *Service {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Workers <= 0 {
		options.Workers = defaultWorkers
	}
	if options.CacheTTL <= 0 {
		options.CacheTTL = defaultCacheTTL
	}
	chart := options.Chart
	if chart == nil {
		if series, ok := options.TwelveData.(SeriesSource); ok {
			chart = series
		}
	}
	return &Service{
		primary:   options.Finnhub,
		fallback:  options.TwelveData,
		chart:     chart,
		now:       options.Now,
		workers:   options.Workers,
		cacheTTL:  options.CacheTTL,
		cache:     make(map[domain.Ticker]cachedObservation),
		lastKnown: make(map[domain.Ticker]domain.Observation),
	}
}

// Quotes fetches every ticker with bounded concurrency and preserves input order.
// Failures are omitted per ticker; they never fail the batch.
func (s *Service) Quotes(ctx context.Context, tickers []domain.Ticker) Batch {
	batch := Batch{Quotes: make([]domain.Quote, 0, len(tickers))}
	if len(tickers) == 0 {
		return batch
	}

	// A watchlist cannot contain duplicates, but a batch refresh must never spend two
	// provider calls for the same ticker.
	unique := make([]domain.Ticker, 0, len(tickers))
	seen := make(map[domain.Ticker]struct{}, len(tickers))
	for _, ticker := range tickers {
		if _, ok := seen[ticker]; ok {
			continue
		}
		seen[ticker] = struct{}{}
		unique = append(unique, ticker)
	}

	now := s.now()
	outcomes := make([]quoteOutcome, len(unique))

	jobs := make(chan int)
	var group sync.WaitGroup

	workers := s.workers
	if workers > len(unique) {
		workers = len(unique)
	}
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				result := s.observe(ctx, unique[index], now)
				if result.observation == nil {
					outcomes[index] = quoteOutcome{
						refused:     result.refused,
						unavailable: result.unavailable,
						canceled:    result.canceled,
						credential:  result.credential,
						retryAfter:  result.retryAfter,
					}
					continue
				}
				quote := domain.DeriveQuote(*result.observation, now)
				outcomes[index] = quoteOutcome{quote: &quote}
			}
		}()
	}

dispatch:
	for index := range unique {
		select {
		case jobs <- index:
		case <-ctx.Done():
			// Never leave an undispatched ticker looking like a clean miss.
			for remaining := index; remaining < len(unique); remaining++ {
				outcomes[remaining].canceled = true
			}
			break dispatch
		}
	}
	close(jobs)
	group.Wait()

	var shortest time.Duration
	refusedCount := 0
	unavailable := false
	canceled := false
	for _, result := range outcomes {
		if result.quote != nil {
			batch.Quotes = append(batch.Quotes, *result.quote)
			continue
		}
		if result.refused {
			refusedCount++
			if shortest == 0 || result.retryAfter < shortest {
				shortest = result.retryAfter
			}
		}
		if result.unavailable {
			unavailable = true
		}
		if result.canceled {
			canceled = true
		}
	}

	// A served quote makes the refresh partially useful; only a wholly refused batch
	// is a rate-limit response. A wholly failed batch is an upstream failure so the
	// client can show the provider-unavailable copy (SCN-047). Credential failures
	// are a distinct signal because the actionable copy is MISSING_API_KEY.
	if len(batch.Quotes) == 0 {
		switch {
		case canceled:
			batch.Canceled = true
		case refusedCount > 0:
			retryAfter := shortest
			if retryAfter <= 0 {
				retryAfter = time.Second
			}
			batch.RateLimitedAfter = &retryAfter
		default:
			if allCredentialFailures(outcomes) {
				batch.CredentialFailure = true
			}
			if unavailable {
				batch.Unavailable = true
			}
		}
	}
	return batch
}

// allCredentialFailures reports whether every ticker failed only because its
// upstream rejected the credential. A served quote, rate limit, unknown symbol,
// cancellation or other outage makes this false.
func allCredentialFailures(outcomes []quoteOutcome) bool {
	if len(outcomes) == 0 {
		return false
	}
	for _, result := range outcomes {
		if !result.credential {
			return false
		}
	}
	return true
}

// VerifySymbol reports whether a provider recognises ticker. An unknown symbol from
// either provider short-circuits; the result is nil when either provider succeeds.
func (s *Service) VerifySymbol(ctx context.Context, ticker domain.Ticker) error {
	unlock := s.lockTicker(ticker)
	defer unlock()

	var failures []*Failure
	for _, pair := range s.sources() {
		if pair.source == nil {
			continue
		}
		provider := pair.name
		observation, err := pair.source.Quote(ctx, ticker)
		if err == nil {
			if observation.Ticker == "" {
				observation.Ticker = ticker
			}
			s.remember(observation, s.now())
			return nil
		}
		var providerFailure *Failure
		if !errors.As(err, &providerFailure) {
			if isCanceledContext(ctx, err) {
				return failure(provider, FailureCanceled)
			}
			providerFailure = &Failure{Kind: FailureUnavailable, Provider: provider}
		}
		if providerFailure.Kind == FailureUnknownSymbol || providerFailure.Kind == FailureCanceled {
			return providerFailure
		}
		failures = append(failures, providerFailure)
	}

	if len(failures) == 0 {
		return &Failure{Kind: FailureUnavailable, Provider: domain.ProviderFinnhub}
	}

	var rateLimited *Failure
	allMissingKey := true
	allAuth := true
	for _, providerFailure := range failures {
		if providerFailure.Kind == FailureRateLimited {
			if rateLimited == nil || providerFailure.RetryAfter < rateLimited.RetryAfter {
				rateLimited = providerFailure
			}
		}
		if providerFailure.Kind != FailureMissingKey {
			allMissingKey = false
		}
		if providerFailure.Kind != FailureAuth {
			allAuth = false
		}
	}
	if rateLimited != nil {
		return rateLimited
	}
	if allMissingKey {
		return aggregateFailure(FailureMissingKey, failures)
	}
	if allAuth {
		return aggregateFailure(FailureAuth, failures)
	}
	return aggregateFailure(FailureUnavailable, failures)
}

// aggregateFailure keeps the attribution of the provider that actually failed.
// The first failure is the first source that was tried, so a Twelve-Data-only
// outage is never reported or logged as Finnhub. When both providers fail the
// aggregate names the primary, which is the one tried first.
func aggregateFailure(kind FailureKind, failures []*Failure) *Failure {
	provider := domain.ProviderFinnhub
	if len(failures) > 0 && failures[0].Provider != "" {
		provider = failures[0].Provider
	}
	return &Failure{Kind: kind, Provider: provider}
}

// DailySeries returns daily closes oldest first.
func (s *Service) DailySeries(ctx context.Context, ticker domain.Ticker, rangeValue domain.PriceRange) ([]domain.PricePoint, error) {
	if s.chart == nil {
		return nil, &Failure{Kind: FailureUnavailable, Provider: domain.ProviderTwelveData}
	}
	points, err := s.chart.DailySeries(ctx, ticker, domain.PointsFor(rangeValue))
	if err != nil {
		return nil, err
	}
	return domain.OrderOldestFirst(points), nil
}

func (s *Service) observe(ctx context.Context, ticker domain.Ticker, now time.Time) quoteOutcome {
	unlock := s.lockTicker(ticker)
	defer unlock()

	if ctx.Err() != nil {
		return quoteOutcome{canceled: true}
	}
	if cached, ok := s.cached(ticker, now); ok {
		return quoteOutcome{observation: &cached}
	}

	var refused bool
	var unavailable bool
	var canceled bool
	var credential bool
	credentialOnly := true
	var retryAfter time.Duration
	for _, pair := range s.sources() {
		if pair.source == nil {
			continue
		}
		observation, err := pair.source.Quote(ctx, ticker)
		if err == nil {
			if observation.Ticker == "" {
				observation.Ticker = ticker
			}
			s.remember(observation, now)
			return quoteOutcome{observation: &observation}
		}
		if ctx.Err() != nil || isCanceledContext(ctx, err) {
			canceled = true
			break
		}
		var providerFailure *Failure
		if !errors.As(err, &providerFailure) {
			unavailable = true
			credentialOnly = false
			continue
		}
		switch providerFailure.Kind {
		case FailureUnknownSymbol:
			// Unknown is definitive: do not spend the fallback budget, and do not
			// resurrect an older observation for a symbol the provider rejects.
			s.forget(ticker)
			return quoteOutcome{}
		case FailureCanceled:
			canceled = true
		case FailureRateLimited:
			refused = true
			credentialOnly = false
			wait := providerFailure.RetryAfter
			if wait <= 0 {
				wait = time.Second
			}
			if retryAfter == 0 || wait < retryAfter {
				retryAfter = wait
			}
		case FailureAuth, FailureMissingKey:
			credential = true
			unavailable = true
		case FailureUnavailable:
			unavailable = true
			credentialOnly = false
		default:
			unavailable = true
			credentialOnly = false
		}
		if canceled {
			break
		}
	}

	if canceled {
		return quoteOutcome{refused: refused, unavailable: unavailable, canceled: true, retryAfter: retryAfter}
	}
	s.forget(ticker)
	if last, ok := s.lastKnownObservation(ticker); ok && !domain.IsStale(last.Timestamp, now) {
		return quoteOutcome{observation: &last}
	}
	return quoteOutcome{
		refused:     refused,
		unavailable: unavailable,
		credential:  credential && credentialOnly && !refused,
		retryAfter:  retryAfter,
	}
}

// isCanceledContext distinguishes a caller cancellation from a provider outage.
func isCanceledContext(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *Service) cached(ticker domain.Ticker, now time.Time) (domain.Observation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[ticker]
	if !ok || now.Sub(entry.fetchedAt) >= s.cacheTTL {
		return domain.Observation{}, false
	}
	return entry.observation, true
}

func (s *Service) remember(observation domain.Observation, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[observation.Ticker] = cachedObservation{observation: observation, fetchedAt: now}
	s.lastKnown[observation.Ticker] = observation
}

func (s *Service) forget(ticker domain.Ticker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, ticker)
}

func (s *Service) lastKnownObservation(ticker domain.Ticker) (domain.Observation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	observation, ok := s.lastKnown[ticker]
	return observation, ok
}

// Forget drops every in-memory observation for ticker. Used when a ticker is removed
// from the watchlist, so churn cannot grow the maps and a later re-add starts clean.
func (s *Service) Forget(ticker domain.Ticker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, ticker)
	delete(s.lastKnown, ticker)
}

// lockTicker serialises work on one ticker via a fixed set of stripes. A fixed array
// keeps the lock table bounded; collisions only serialise unrelated tickers briefly.
func (s *Service) lockTicker(ticker domain.Ticker) func() {
	hash := uint32(fnvOffsetBasis)
	for index := 0; index < len(ticker); index++ {
		hash ^= uint32(ticker[index])
		hash *= fnvPrime
	}
	lock := &s.stripes[hash%tickerLockStripes]
	lock.Lock()
	return lock.Unlock
}
