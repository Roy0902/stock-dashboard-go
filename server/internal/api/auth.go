package api

import (
	"net/http"
	"time"

	"stock-dashboard/server/internal/auth"
	"stock-dashboard/server/internal/domain"
)

const (
	sessionCookieName   = "sd_session"
	challengeCookieName = "sd_challenge"
	enrollCookieName    = "sd_enroll"

	sessionCookiePath = "/"
	authCookiePath    = "/api/auth"

	maxAuthBodyBytes = 4 << 10
)

// authStatus is public: it reports first-run state and, with a valid session,
// the signed-in account.
func (h *handlers) authStatus(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable(w, r) {
		return
	}
	status := h.deps.Auth.Status(sessionToken(r))
	body := authStatusBody{
		SetupRequired: status.SetupRequired,
		Authenticated: status.Authenticated,
	}
	if status.User != nil {
		body.User = toAuthUser(*status.User)
	}
	writeJSON(w, http.StatusOK, body)
}

// authSetup validates the account credentials and holds them in memory only.
func (h *handlers) authSetup(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable(w, r) {
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeAuthJSON(w, r, &request, domain.CodeInvalidRequest); err != nil {
		h.fail(w, r, err)
		return
	}

	material, err := h.deps.Auth.BeginSetup(request.Username, request.Password)
	if err != nil {
		h.logAuthEvent(r, "setup_failed", loggableUsername(request.Username))
		h.fail(w, r, err)
		return
	}
	h.setAuthCookie(w, enrollCookieName, material.EnrollmentToken, authCookiePath, auth.EnrollmentTTL)
	h.logAuthEvent(r, "setup_pending", material.Username)
	writeJSON(w, http.StatusCreated, setupResponseBody{
		Username:      material.Username,
		Secret:        material.Secret,
		OTPAuthURL:    material.OTPAuthURL,
		RecoveryCodes: material.RecoveryCodes,
	})
}

// authActivate verifies the first TOTP code, persists the account and opens its
// first session.
func (h *handlers) authActivate(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable(w, r) {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := decodeAuthJSON(w, r, &request, domain.CodeInvalidCode); err != nil {
		h.fail(w, r, err)
		return
	}

	session, err := h.deps.Auth.Activate(cookieValue(r, enrollCookieName), request.Code)
	if err != nil {
		if domain.IsCode(err, domain.CodeEnrollmentExpired) {
			h.clearAuthCookie(w, enrollCookieName, authCookiePath)
		}
		h.logAuthEvent(r, "activation_failed", "")
		h.fail(w, r, err)
		return
	}
	h.clearAuthCookie(w, enrollCookieName, authCookiePath)
	h.setAuthCookie(w, sessionCookieName, session.Token, sessionCookiePath, auth.SessionAbsoluteTTL)
	h.logAuthEvent(r, "owner_activated", session.User.Username)
	writeJSON(w, http.StatusOK, authUserResponseBody{User: toAuthUser(session.User)})
}

// authLogin verifies username and password and starts a second-factor challenge.
func (h *handlers) authLogin(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable(w, r) {
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeAuthJSON(w, r, &request, domain.CodeInvalidRequest); err != nil {
		h.fail(w, r, err)
		return
	}

	challenge, err := h.deps.Auth.Login(request.Username, request.Password)
	if err != nil {
		h.logAuthEvent(r, "login_failed", loggableUsername(request.Username))
		h.fail(w, r, err)
		return
	}
	h.setAuthCookie(w, challengeCookieName, challenge.Token, authCookiePath, auth.ChallengeTTL)
	h.logAuthEvent(r, "login_password_ok", request.Username)
	writeJSON(w, http.StatusOK, challengeResponseBody{
		Status:           "totp_required",
		ExpiresInSeconds: challenge.ExpiresInSeconds,
	})
}

// authLoginTOTP completes login with a six-digit code.
func (h *handlers) authLoginTOTP(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable(w, r) {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := decodeAuthJSON(w, r, &request, domain.CodeInvalidCode); err != nil {
		h.fail(w, r, err)
		return
	}

	session, err := h.deps.Auth.CompleteTOTP(cookieValue(r, challengeCookieName), request.Code)
	if err != nil {
		h.finishChallengeFailure(w, r, "totp_failed", err)
		return
	}
	h.clearAuthCookie(w, challengeCookieName, authCookiePath)
	h.setAuthCookie(w, sessionCookieName, session.Token, sessionCookiePath, auth.SessionAbsoluteTTL)
	h.logAuthEvent(r, "totp_succeeded", session.User.Username)
	writeJSON(w, http.StatusOK, authUserResponseBody{User: toAuthUser(session.User)})
}

// authLoginRecovery completes login with a single-use recovery code.
func (h *handlers) authLoginRecovery(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable(w, r) {
		return
	}
	var request struct {
		Code string `json:"code"`
	}
	if err := decodeAuthJSON(w, r, &request, domain.CodeInvalidRecoveryCode); err != nil {
		h.fail(w, r, err)
		return
	}

	session, err := h.deps.Auth.CompleteRecovery(cookieValue(r, challengeCookieName), request.Code)
	if err != nil {
		h.finishChallengeFailure(w, r, "recovery_failed", err)
		return
	}
	h.clearAuthCookie(w, challengeCookieName, authCookiePath)
	h.setAuthCookie(w, sessionCookieName, session.Token, sessionCookiePath, auth.SessionAbsoluteTTL)
	h.logAuthEvent(r, "recovery_succeeded", session.User.Username)
	writeJSON(w, http.StatusOK, authUserResponseBody{User: toAuthUser(session.User)})
}

// authLogout revokes the current session and clears auth cookies. The session
// cookie is cleared only when the request presented one, so a cross-site image
// cannot force a sign-out; the challenge and enrollment cookies are always
// cleared because they are unusable after sign-out. Idempotent.
func (h *handlers) authLogout(w http.ResponseWriter, r *http.Request) {
	if h.deps.Auth != nil {
		h.deps.Auth.Logout(sessionToken(r))
	}
	h.clearSessionCookieIfPresent(w, r)
	h.clearAuthCookie(w, challengeCookieName, authCookiePath)
	h.clearAuthCookie(w, enrollCookieName, authCookiePath)
	h.logAuthEvent(r, "logout", "")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

// authMe returns the account requireSession attached to the request. The guard
// owns authentication, the 401 envelope and any cookie clearing, so this handler
// never authenticates or clears a cookie a second time.
func (h *handlers) authMe(w http.ResponseWriter, r *http.Request) {
	user, _ := sessionUserFrom(r.Context())
	writeJSON(w, http.StatusOK, authUserResponseBody{User: &authUserBody{
		Username:    user.Username,
		TOTPEnabled: user.TOTPEnabled,
	}})
}

// finishChallengeFailure clears an expired challenge and logs a generic event.
func (h *handlers) finishChallengeFailure(w http.ResponseWriter, r *http.Request, event string, err error) {
	if domain.IsCode(err, domain.CodeChallengeExpired) {
		h.clearAuthCookie(w, challengeCookieName, authCookiePath)
	}
	h.logAuthEvent(r, event, "")
	h.fail(w, r, err)
}

func (h *handlers) authAvailable(w http.ResponseWriter, r *http.Request) bool {
	if h.deps.Auth != nil {
		return true
	}
	h.fail(w, r, authError(domain.CodeInternalError))
	return false
}

func authError(code domain.Code) *domain.Error {
	return domain.NewError(code, domain.DefaultMessage(code))
}

// logAuthEvent records event, username and request_id only. Secrets, codes and
// token values never reach this method.
func (h *handlers) logAuthEvent(r *http.Request, event, username string) {
	h.logger().Info("auth event",
		"event", event,
		"username", safeUsername(username),
		"request_id", requestIDFrom(r.Context()),
	)
}

// loggableUsername drops input that is not a valid username before it reaches a
// log line, so a confused client cannot smuggle a password into the username field.
func loggableUsername(raw string) string {
	normalized := auth.NormalizeUsername(raw)
	if !auth.ValidUsername(normalized) {
		return ""
	}
	return normalized
}

// safeUsername keeps log lines bounded and free of control characters.
func safeUsername(username string) string {
	const limit = 64
	usable := make([]rune, 0, limit)
	for _, symbol := range username {
		if symbol < 0x20 || symbol == 0x7f {
			continue
		}
		usable = append(usable, symbol)
		if len(usable) == limit {
			break
		}
	}
	return string(usable)
}

func sessionToken(r *http.Request) string {
	return cookieValue(r, sessionCookieName)
}

func cookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *handlers) now() time.Time {
	if h.deps.Now != nil {
		return h.deps.Now()
	}
	return time.Now()
}

func (h *handlers) setAuthCookie(w http.ResponseWriter, name, value, path string, lifetime time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   int(lifetime / time.Second),
		Expires:  h.now().Add(lifetime).UTC(),
		HttpOnly: true,
		Secure:   h.deps.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearAuthCookie uses Max-Age=0 plus an epoch Expires, per the contract.
func (h *handlers) clearAuthCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		Secure:   h.deps.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearSessionCookieIfPresent expires sd_session only when the request actually
// carried one. A request without the cookie must never emit a clearing
// Set-Cookie: a hostile cross-site <img> could otherwise sign the user out.
func (h *handlers) clearSessionCookieIfPresent(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie(sessionCookieName); err == nil {
		h.clearAuthCookie(w, sessionCookieName, sessionCookiePath)
	}
}

func toAuthUser(account auth.Account) *authUserBody {
	return &authUserBody{Username: account.Username, TOTPEnabled: account.TOTPEnabled}
}

// decodeAuthJSON enforces the shared body rules with endpoint-specific copy so a
// code step never shows the credentials copy.
func decodeAuthJSON(w http.ResponseWriter, r *http.Request, destination any, failure domain.Code) error {
	return decodeJSON(w, r, destination, maxAuthBodyBytes, func() error { return authError(failure) })
}
