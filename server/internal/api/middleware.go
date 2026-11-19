package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"stock-dashboard/server/internal/domain"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	sessionUserKey
	requestIDHeader = "X-Request-Id"
)

// sessionUser is the authenticated account attached to a protected request. The
// ID is internal only; responses expose just the username and TOTP state.
type sessionUser struct {
	ID          string
	Username    string
	TOTPEnabled bool
}

func sessionUserFrom(ctx context.Context) (sessionUser, bool) {
	user, ok := ctx.Value(sessionUserKey).(sessionUser)
	return user, ok
}

func sessionUserID(ctx context.Context) string {
	user, _ := sessionUserFrom(ctx)
	return user.ID
}

func withSessionUser(ctx context.Context, user sessionUser) context.Context {
	return context.WithValue(ctx, sessionUserKey, user)
}

// ContentSecurityPolicy is compatible with a Vite SPA build: scripts and styles are
// external files, and only data-URI images are allowed beyond same-origin.
const ContentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; " +
	"frame-ancestors 'none'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; font-src 'self'; connect-src 'self'; form-action 'self'"

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(bytes[:])
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Content-Security-Policy", ContentSecurityPolicy)
		next.ServeHTTP(w, r)
	})
}

// defaultAllowedHosts keeps the local tool loopback-only unless ALLOWED_HOSTS
// widens it. The values match config's default.
var defaultAllowedHosts = []string{"127.0.0.1", "localhost", "::1"}

func recoverMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				// The panic value is deliberately not logged: it can contain upstream
				// text. The stack identifies the code path.
				logger.Error("panic recovered",
					"request_id", requestIDFrom(r.Context()),
					"panic_type", fmt.Sprintf("%T", recovered),
					"stack", string(debug.Stack()),
				)
				// A handler that already wrote a header/body must not receive a
				// second status line and envelope appended to its response.
				if recorder, ok := w.(*statusRecorder); ok && recorder.status != 0 {
					return
				}
				writeJSON(w, http.StatusInternalServerError, errorBody{
					Error: domain.DefaultMessage(domain.CodeInternalError),
					Code:  string(domain.CodeInternalError),
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func accessLogMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		// Path only: query strings can carry keys.
		logger.LogAttrs(r.Context(), levelFor(status), "request",
			slog.String("request_id", requestIDFrom(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}

func levelFor(status int) slog.Level {
	if status >= http.StatusInternalServerError {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

// hostGuard rejects requests whose Host is not in the configured allowlist.
// Together with the Origin guard this blocks DNS-rebinding pages. A nil list
// keeps the legacy loopback default.
func hostGuard(allowed []string, next http.Handler) http.Handler {
	if len(allowed) == 0 {
		allowed = defaultAllowedHosts
	}
	permitted := make(map[string]struct{}, len(allowed))
	for _, host := range allowed {
		permitted[strings.ToLower(strings.Trim(host, "[]"))] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostAllowed(r.Host, permitted) {
			writeJSON(w, http.StatusForbidden, errorBody{
				Error: domain.DefaultMessage(domain.CodeCrossOriginRequest),
				Code:  string(domain.CodeCrossOriginRequest),
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hostAllowed(hostport string, permitted map[string]struct{}) bool {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return false
	}
	host := hostport
	if parsed, _, err := net.SplitHostPort(hostport); err == nil {
		host = parsed
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	_, ok := permitted[host]
	return ok
}

// crossOriginGuard rejects unsafe /api requests whose Origin header is present
// and does not match the request host. Requests without Origin (scripts, tests,
// same-origin navigations) pass; "null" never matches.
func (h *handlers) crossOriginGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isUnsafeMethod(r.Method) && isAPIPath(r.URL.Path) {
			if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
				h.fail(w, r, authError(domain.CodeCrossOriginRequest))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireSession guards a route. Unauthenticated requests get UNAUTHENTICATED;
// a presented session cookie is also expired so the SPA can show the expiry
// screen. The guard is attached per route so a new /api route cannot ship public
// by accident.
func (h *handlers) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.deps.Auth != nil {
			if account, ok := h.deps.Auth.Authenticate(sessionToken(r)); ok {
				ctx := withSessionUser(r.Context(), sessionUser{
					ID:          account.ID,
					Username:    account.Username,
					TOTPEnabled: account.TOTPEnabled,
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		h.clearSessionCookieIfPresent(w, r)
		h.fail(w, r, authError(domain.CodeUnauthenticated))
	})
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

// sameOrigin accepts http://<Host> and https://<Host> only, both compared
// case-insensitively against the request host.
func sameOrigin(origin, host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	for _, scheme := range []string{"http://", "https://"} {
		if strings.EqualFold(origin, scheme+host) {
			return true
		}
	}
	return false
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

// Unwrap lets http.ResponseController reach the real writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(body)
}
