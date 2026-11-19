package api

import (
	"log/slog"
	"net/http"
)

type handlers struct {
	deps Deps
}

// routeDefinition is the single route table. Every /api route is either
// explicitly public or wrapped by requireSession, so a new endpoint cannot ship
// unauthenticated by forgetting a second list elsewhere.
type routeDefinition struct {
	method  string
	pattern string
	public  bool
	handler func(*handlers, http.ResponseWriter, *http.Request)
}

var routeDefinitions = []routeDefinition{
	{method: http.MethodGet, pattern: "/healthz", public: true, handler: (*handlers).health},

	{method: http.MethodGet, pattern: "/api/watchlist", handler: (*handlers).listWatchlist},
	{method: http.MethodPost, pattern: "/api/watchlist", handler: (*handlers).addWatchlist},
	{method: http.MethodDelete, pattern: "/api/watchlist/{ticker}", handler: (*handlers).removeWatchlist},
	{method: http.MethodGet, pattern: "/api/quotes", handler: (*handlers).quotes},
	{method: http.MethodGet, pattern: "/api/chart/{ticker}", handler: (*handlers).chart},

	{method: http.MethodGet, pattern: "/api/auth/status", public: true, handler: (*handlers).authStatus},
	{method: http.MethodPost, pattern: "/api/auth/setup", public: true, handler: (*handlers).authSetup},
	{method: http.MethodPost, pattern: "/api/auth/2fa/activate", public: true, handler: (*handlers).authActivate},
	{method: http.MethodPost, pattern: "/api/auth/login", public: true, handler: (*handlers).authLogin},
	{method: http.MethodPost, pattern: "/api/auth/login/totp", public: true, handler: (*handlers).authLoginTOTP},
	{method: http.MethodPost, pattern: "/api/auth/login/recovery", public: true, handler: (*handlers).authLoginRecovery},
	{method: http.MethodPost, pattern: "/api/auth/logout", public: true, handler: (*handlers).authLogout},
	{method: http.MethodGet, pattern: "/api/auth/me", handler: (*handlers).authMe},

	// Unknown /api paths stay a JSON 404. They are explicitly public because the
	// handler only answers NOT_FOUND and never reads account state.
	{pattern: "/api", public: true, handler: (*handlers).apiNotFound},
	{pattern: "/api/", public: true, handler: (*handlers).apiNotFound},
}

// NewRouter builds the API and SPA routes and wraps them with the middleware stack.
// The order is: security headers, request id, access log, panic recovery, host guard,
// cross-origin guard, mux. Recovery sits outside the host guard so a panic in any
// guard still produces the error envelope.
func NewRouter(deps Deps, spa http.Handler) http.Handler {
	h := &handlers{deps: deps}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	for _, route := range routeDefinitions {
		pattern := route.pattern
		if route.method != "" {
			pattern = route.method + " " + route.pattern
		}
		handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route.handler(h, w, r)
		}))
		if !route.public {
			handler = h.requireSession(handler)
		}
		mux.Handle(pattern, handler)
	}

	if spa == nil {
		spa = http.NotFoundHandler()
	}
	mux.Handle("/", spa)

	return securityHeadersMiddleware(
		requestIDMiddleware(
			accessLogMiddleware(logger,
				recoverMiddleware(logger,
					hostGuard(deps.AllowedHosts, h.crossOriginGuard(mux)),
				),
			),
		),
	)
}
