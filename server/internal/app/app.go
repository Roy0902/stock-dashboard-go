package app

import (
	"log/slog"
	"net/http"
	"time"

	"stock-dashboard/server/internal/api"
	"stock-dashboard/server/internal/auth"
	"stock-dashboard/server/internal/config"
	"stock-dashboard/server/internal/domain"
	"stock-dashboard/server/internal/providers"
	"stock-dashboard/server/internal/store"
	"stock-dashboard/server/internal/webui"
)

// App is the composed server. Close releases idle upstream connections and the
// database handle.
type App struct {
	handler http.Handler
	client  *providers.HTTPClient
	store   *store.Store
}

// Options inject the clock and password hashing cost at the composition root.
// Zero values use production defaults; tests opt into a fixed clock and cheap
// PBKDF2 parameters.
type Options struct {
	Now              func() time.Time
	PBKDF2Iterations uint32
}

// New wires config, persistence, budgets, providers, the quote service and HTTP.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	return NewWithOptions(cfg, logger, Options{})
}

// NewWithOptions is New with injectable time and hashing parameters.
func NewWithOptions(cfg config.Config, logger *slog.Logger, opts Options) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}

	storage, err := store.OpenWithLogger(cfg.DataDir, logger)
	if err != nil {
		return nil, err
	}
	closeStore := true
	defer func() {
		if closeStore {
			_ = storage.Close()
		}
	}()

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	limits := providers.DefaultLimits
	if cfg.FinnhubRPM > 0 {
		limits.FinnhubRPM = cfg.FinnhubRPM
	}
	if cfg.TwelveDataRPM > 0 {
		limits.TwelveDataRPM = cfg.TwelveDataRPM
	}
	if cfg.UpstreamTimeout > 0 {
		limits.UpstreamTimeout = cfg.UpstreamTimeout
	}
	if cfg.QuoteCacheTTL > 0 {
		limits.CacheTTL = cfg.QuoteCacheTTL
	}
	client := providers.NewHTTPClient(limits.UpstreamTimeout)

	finnhub := providers.NewFinnhub(providers.FinnhubOptions{
		BaseURL: cfg.FinnhubBaseURL,
		APIKey:  cfg.FinnhubAPIKey,
		Budget:  providers.NewBudget(domain.ProviderFinnhub, limits.FinnhubRPM, now),
		Client:  client,
	})
	twelveData := providers.NewTwelveData(providers.TwelveDataOptions{
		BaseURL: cfg.TwelveDataBaseURL,
		APIKey:  cfg.TwelveDataAPIKey,
		Budget:  providers.NewBudget(domain.ProviderTwelveData, limits.TwelveDataRPM, now),
		Client:  client,
	})

	service := providers.NewService(providers.ServiceOptions{
		Finnhub:    finnhub,
		TwelveData: twelveData,
		Chart:      twelveData,
		Now:        now,
		Workers:    limits.Workers,
		CacheTTL:   limits.CacheTTL,
	})

	spa, err := webui.New()
	if err != nil {
		return nil, err
	}

	authService, err := auth.New(storage.Users, auth.Options{Now: now, PBKDF2Iterations: opts.PBKDF2Iterations})
	if err != nil {
		return nil, err
	}

	handler := api.NewRouter(api.Deps{
		Watchlist: storage.Watchlist,
		Quotes:    service,
		Auth:      authService,
		Keys: api.Keys{
			Finnhub:    cfg.FinnhubAPIKey != "",
			TwelveData: cfg.TwelveDataAPIKey != "",
		},
		CookieSecure: cfg.AuthCookieSecure,
		AllowedHosts: cfg.AllowedHosts,
		Now:          now,
		Logger:       logger,
	}, spa)

	closeStore = false
	return &App{handler: handler, client: client, store: storage}, nil
}

// Handler returns the root HTTP handler.
func (a *App) Handler() http.Handler {
	return a.handler
}

// Close releases resources held by upstream transports and persistence.
func (a *App) Close() error {
	if a.client != nil {
		a.client.CloseIdleConnections()
	}
	if a.store != nil {
		return a.store.Close()
	}
	return nil
}
