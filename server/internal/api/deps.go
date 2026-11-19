package api

import (
	"context"
	"log/slog"
	"time"

	"stock-dashboard/server/internal/auth"
	"stock-dashboard/server/internal/domain"
	"stock-dashboard/server/internal/providers"
)

// WatchlistStore is the per-account persistence boundary the handlers depend on.
type WatchlistStore interface {
	List(userID string) domain.Watchlist
	Update(userID string, fn func(domain.Watchlist) (domain.Watchlist, error)) (domain.Watchlist, error)
}

// QuoteService is the provider-facing boundary the handlers depend on.
type QuoteService interface {
	Quotes(ctx context.Context, tickers []domain.Ticker) providers.Batch
	VerifySymbol(ctx context.Context, ticker domain.Ticker) error
	DailySeries(ctx context.Context, ticker domain.Ticker, rangeValue domain.PriceRange) ([]domain.PricePoint, error)
	// Forget drops cached and last-known observations for a removed ticker.
	Forget(ticker domain.Ticker)
}

// AuthService is the authentication boundary the handlers depend on. It owns
// credentials, TOTP, recovery codes and session state; the HTTP layer owns
// cookies, status mapping and auth event logs.
type AuthService interface {
	Status(sessionToken string) auth.Status
	BeginSetup(username, password string) (*auth.SetupMaterial, error)
	Activate(enrollmentToken, code string) (*auth.Session, error)
	Login(username, password string) (*auth.Challenge, error)
	CompleteTOTP(challengeToken, code string) (*auth.Session, error)
	CompleteRecovery(challengeToken, code string) (*auth.Session, error)
	Logout(sessionToken string)
	Authenticate(sessionToken string) (*auth.Account, bool)
}

// Keys records key presence, never the key values.
type Keys struct {
	Finnhub    bool
	TwelveData bool
}

func (k Keys) Complete() bool {
	return k.Finnhub && k.TwelveData
}

// Deps is everything the HTTP layer needs. Keys are booleans so secrets cannot reach it.
type Deps struct {
	Watchlist WatchlistStore
	Quotes    QuoteService
	Auth      AuthService
	Keys      Keys
	// CookieSecure sets the Secure flag on auth cookies; enable behind HTTPS.
	CookieSecure bool
	// AllowedHosts are accepted Host header values; nil uses the loopback default.
	AllowedHosts []string
	// Now is the clock for cookie expiry; nil uses time.Now.
	Now    func() time.Time
	Logger *slog.Logger
}
