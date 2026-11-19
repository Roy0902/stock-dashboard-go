package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"stock-dashboard/server/internal/auth"
	"stock-dashboard/server/internal/domain"
	"stock-dashboard/server/internal/store"
)

// testPBKDF2Iterations keeps API tests fast while exercising the real PHC path.
const testPBKDF2Iterations uint32 = 1_000

const testPassword = "correct horse battery"

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(delta time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delta)
	c.mu.Unlock()
}

type authEnv struct {
	*testEnv
	clock   *testClock
	dataDir string
	storage *store.Store
	service *auth.Service
}

func newAuthEnv(t *testing.T, secure bool) *authEnv {
	t.Helper()
	dataDir := t.TempDir()
	storage, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	clock := &testClock{now: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)}
	service, err := auth.New(storage.Users, auth.Options{Now: clock.Now, PBKDF2Iterations: testPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}

	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	env := &testEnv{store: newFakeStore(), quotes: &fakeQuotes{}, logs: logs}
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		env.spaCalls++
		_, _ = io.WriteString(w, "<!doctype html><title>spa shell</title>")
	})
	env.handler = NewRouter(Deps{
		Watchlist:    storage.Watchlist,
		Quotes:       env.quotes,
		Auth:         service,
		Keys:         completeKeys(),
		CookieSecure: secure,
		Logger:       logger,
	}, spa)
	return &authEnv{testEnv: env, clock: clock, dataDir: dataDir, storage: storage, service: service}
}

func (e *authEnv) request(t *testing.T, method, target, body string, cookies map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, "http://127.0.0.1:3000"+target, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range cookies {
		request.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, request)
	return recorder
}

func (e *authEnv) post(t *testing.T, target, body string, cookies map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return e.request(t, http.MethodPost, target, body, cookies)
}

func (e *authEnv) get(t *testing.T, target string, cookies map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return e.request(t, http.MethodGet, target, "", cookies)
}

func credentialsFor(username, password string) string {
	return `{"username":"` + username + `","password":"` + password + `"}`
}

func (e *authEnv) credentials() string {
	return credentialsFor("owner", testPassword)
}

func (e *authEnv) beginSetup(t *testing.T) (setupResponseBody, *http.Cookie, *httptest.ResponseRecorder) {
	t.Helper()
	return e.beginSetupFor(t, "owner")
}

func (e *authEnv) beginSetupFor(t *testing.T, username string) (setupResponseBody, *http.Cookie, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := e.post(t, "/api/auth/setup", credentialsFor(username, testPassword), nil)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("setup status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	var body setupResponseBody
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body, responseCookie(t, recorder, enrollCookieName), recorder
}

func (e *authEnv) code(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, e.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func (e *authEnv) activate(t *testing.T, material setupResponseBody, enroll *http.Cookie) *http.Cookie {
	t.Helper()
	recorder := e.post(t, "/api/auth/2fa/activate",
		`{"code":"`+e.code(t, material.Secret)+`"}`,
		map[string]string{enrollCookieName: enroll.Value})
	if recorder.Code != http.StatusOK {
		t.Fatalf("activate status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	return responseCookie(t, recorder, sessionCookieName)
}

// owner sets up, activates and returns the session cookie plus setup material.
func (e *authEnv) owner(t *testing.T) (*http.Cookie, setupResponseBody) {
	t.Helper()
	return e.signup(t, "owner")
}

// signup registers and activates one account.
func (e *authEnv) signup(t *testing.T, username string) (*http.Cookie, setupResponseBody) {
	t.Helper()
	material, enroll, _ := e.beginSetupFor(t, username)
	return e.activate(t, material, enroll), material
}

// passwordStep returns the challenge cookie from a successful password login.
func (e *authEnv) passwordStep(t *testing.T) *http.Cookie {
	t.Helper()
	return e.passwordStepFor(t, "owner")
}

func (e *authEnv) passwordStepFor(t *testing.T, username string) *http.Cookie {
	t.Helper()
	recorder := e.post(t, "/api/auth/login", credentialsFor(username, testPassword), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	return responseCookie(t, recorder, challengeCookieName)
}

// loginSession completes password and TOTP and returns the session cookie.
func (e *authEnv) loginSession(t *testing.T, secret string) *http.Cookie {
	t.Helper()
	challenge := e.passwordStep(t)
	recorder := e.post(t, "/api/auth/login/totp",
		`{"code":"`+e.code(t, secret)+`"}`,
		map[string]string{challengeCookieName: challenge.Value})
	if recorder.Code != http.StatusOK {
		t.Fatalf("totp status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	return responseCookie(t, recorder, sessionCookieName)
}

func responseCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	if cookie := findCookie(recorder, name); cookie != nil {
		return cookie
	}
	t.Fatalf("response has no %s cookie; headers %v", name, recorder.Header().Values("Set-Cookie"))
	return nil
}

func findCookie(recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func decodeBody[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var body T
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}
	return body
}

func requireCookieFlags(t *testing.T, cookie *http.Cookie, path string, maxAge int, secure bool) {
	t.Helper()
	if cookie.Path != path {
		t.Fatalf("%s path = %q, want %q", cookie.Name, cookie.Path, path)
	}
	if cookie.MaxAge != maxAge {
		t.Fatalf("%s max-age = %d, want %d", cookie.Name, cookie.MaxAge, maxAge)
	}
	if !cookie.HttpOnly {
		t.Fatalf("%s is not HttpOnly", cookie.Name)
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("%s same-site = %v", cookie.Name, cookie.SameSite)
	}
	if cookie.Secure != secure {
		t.Fatalf("%s secure = %v, want %v", cookie.Name, cookie.Secure, secure)
	}
	if cookie.Value == "" {
		t.Fatalf("%s value is empty", cookie.Name)
	}
}

// requireCleared rejects both an empty Set-Cookie and a live cookie. Go parses
// Max-Age=0 into MaxAge -1.
func requireCleared(t *testing.T, cookie *http.Cookie) {
	t.Helper()
	if cookie.MaxAge >= 0 {
		t.Fatalf("%s max-age = %d, want a clearing cookie", cookie.Name, cookie.MaxAge)
	}
	if cookie.Value != "" {
		t.Fatalf("%s has a value on clear", cookie.Name)
	}
	if cookie.Path == "" {
		t.Fatalf("%s clear has no path", cookie.Name)
	}
}

func TestAuthStatusBeforeSetup(t *testing.T) {
	env := newAuthEnv(t, false)
	recorder := env.get(t, "/api/auth/status", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Body.String() != `{"setupRequired":true,"authenticated":false}` {
		t.Fatalf("body = %q", recorder.Body.String())
	}
	if cookies := recorder.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("status set cookies: %v", cookies)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q", got)
	}

	for _, target := range []string{"/api/watchlist", "/api/quotes", "/api/chart/AAPL"} {
		recorder := env.get(t, target, nil)
		requireError(t, recorder, http.StatusUnauthorized, domain.CodeUnauthenticated, "Authentication required.")
	}
}

func TestAuthSetupStaysPendingUntilActivation(t *testing.T) {
	env := newAuthEnv(t, false)

	material, enroll, setupRecorder := env.beginSetup(t)
	if material.Username != "owner" {
		t.Fatalf("username = %q", material.Username)
	}
	if material.Secret == "" {
		t.Fatal("setup did not return a secret")
	}
	if !strings.HasPrefix(material.OTPAuthURL, "otpauth://totp/Stock%20Dashboard:owner?") {
		t.Fatalf("otpauth url = %q", material.OTPAuthURL)
	}
	if !strings.Contains(material.OTPAuthURL, "issuer=Stock%20Dashboard") {
		t.Fatalf("otpauth url = %q", material.OTPAuthURL)
	}
	if len(material.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("recovery codes = %d", len(material.RecoveryCodes))
	}
	shape := regexp.MustCompile(`^[A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4}$`)
	for _, code := range material.RecoveryCodes {
		if !shape.MatchString(code) {
			t.Fatalf("recovery code %q has the wrong shape", code)
		}
	}
	requireCookieFlags(t, enroll, authCookiePath, int(auth.EnrollmentTTL/time.Second), false)
	if session := findCookie(setupRecorder, sessionCookieName); session != nil {
		t.Fatal("setup issued a session cookie")
	}

	// Setup must not persist an owner or issue a session.
	if env.storage.Users.Count() != 0 {
		t.Fatal("account stored before activation")
	}

	status := decodeBody[authStatusBody](t, env.get(t, "/api/auth/status", nil))
	if !status.SetupRequired || status.Authenticated || status.User != nil {
		t.Fatalf("status = %+v", status)
	}

	// Invalid activation keeps the owner absent and the enrollment alive.
	recorder := env.post(t, "/api/auth/2fa/activate", `{"code":"000000"}`,
		map[string]string{enrollCookieName: enroll.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCode, "Invalid verification code.")
	if env.storage.Users.Count() != 0 {
		t.Fatal("invalid activation stored an account")
	}

	session := env.activate(t, material, enroll)
	requireCookieFlags(t, session, sessionCookiePath, int(auth.SessionAbsoluteTTL/time.Second), false)

	// A consumed enrollment token reports expiration and clears the cookie.
	recorder = env.post(t, "/api/auth/2fa/activate", `{"code":"000000"}`,
		map[string]string{enrollCookieName: enroll.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeEnrollmentExpired, "Setup timed out. Start again.")
	requireCleared(t, responseCookie(t, recorder, enrollCookieName))

	owner := env.storage.Users.FindByUsername("owner")
	if owner == nil || owner.Username != "owner" {
		t.Fatalf("account = %+v", owner)
	}
	if !strings.HasPrefix(owner.PasswordHash, "$pbkdf2-sha256$i=1000$") {
		t.Fatalf("account has no pbkdf2-sha256 PHC hash: %q", owner.PasswordHash)
	}
	if strings.Contains(owner.PasswordHash, testPassword) {
		t.Fatal("stored password hash contains the plaintext password")
	}
	if len(owner.RecoveryCodeHashes) != 0 {
		t.Fatalf("FindByUsername loaded recovery hashes: %d", len(owner.RecoveryCodeHashes))
	}
	// Recovery hashes stay persisted; the full update path loads them.
	var storedHashes []string
	probe := errors.New("probe: read full account")
	err := env.storage.Users.Update(owner.ID, func(user *auth.User) error {
		storedHashes = append([]string(nil), user.RecoveryCodeHashes...)
		return probe
	})
	if !errors.Is(err, probe) {
		t.Fatalf("full account probe = %v", err)
	}
	if len(storedHashes) != auth.RecoveryCodeCount {
		t.Fatalf("recovery hashes = %d", len(storedHashes))
	}
	for _, hash := range storedHashes {
		if len(hash) != 64 {
			t.Fatalf("recovery hash %q is malformed", hash)
		}
	}

	// The database file is the only persisted state and holds no plaintext secret.
	databasePath := filepath.Join(env.dataDir, store.DBFileName)
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatalf("read database: %v", err)
	}
	if bytes.Contains(database, []byte(testPassword)) {
		t.Fatal("database contains the plaintext password")
	}
	if bytes.Contains(database, []byte(material.RecoveryCodes[0])) {
		t.Fatal("database contains a plaintext recovery code")
	}
	if _, err := os.Stat(filepath.Join(env.dataDir, "users.json")); !os.IsNotExist(err) {
		t.Fatalf("users.json exists after activation: %v", err)
	}

	status = decodeBody[authStatusBody](t, env.get(t, "/api/auth/status",
		map[string]string{sessionCookieName: session.Value}))
	if status.SetupRequired || !status.Authenticated || status.User == nil || status.User.Username != "owner" {
		t.Fatalf("status = %+v", status)
	}

	me := env.get(t, "/api/auth/me", map[string]string{sessionCookieName: session.Value})
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d", me.Code)
	}
	list := env.get(t, "/api/watchlist", map[string]string{sessionCookieName: session.Value})
	if list.Code != http.StatusOK || list.Body.String() != `{"tickers":[]}` {
		t.Fatalf("watchlist = %d %q", list.Code, list.Body.String())
	}
}

func TestAuthDuplicateSetup(t *testing.T) {
	env := newAuthEnv(t, false)
	env.owner(t)
	recorder := env.post(t, "/api/auth/setup", env.credentials(), nil)
	requireError(t, recorder, http.StatusConflict, domain.CodeUsernameTaken, "That username is already taken.")

	// Sign-up stays open for a new username.
	recorder = env.post(t, "/api/auth/setup", credentialsFor("second", testPassword), nil)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("second setup status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	material := decodeBody[setupResponseBody](t, recorder)
	if material.Username != "second" {
		t.Fatalf("username = %q", material.Username)
	}
}

// TestAuthUsernameIsCaseSensitive pins the frozen scenario: uppercase input is
// invalid at setup (no case folding) and never matches at login.
func TestAuthMultiUserSeparateWatchlistsAndQuotes(t *testing.T) {
	env := newAuthEnv(t, false)
	aliceSession, _ := env.signup(t, "alice")
	bobSession, _ := env.signup(t, "bob")
	alice := map[string]string{sessionCookieName: aliceSession.Value}
	bob := map[string]string{sessionCookieName: bobSession.Value}

	// Both sessions authenticate independently and expose no account ID.
	for username, cookies := range map[string]map[string]string{"alice": alice, "bob": bob} {
		status := env.get(t, "/api/auth/status", cookies)
		if status.Code != http.StatusOK {
			t.Fatalf("%s status = %d", username, status.Code)
		}
		body := decodeBody[authStatusBody](t, status)
		if body.SetupRequired || !body.Authenticated || body.User == nil || body.User.Username != username {
			t.Fatalf("%s status = %+v", username, body)
		}
		if strings.Contains(status.Body.String(), "usr_") {
			t.Fatalf("%s status leaked an account id: %q", username, status.Body.String())
		}
		me := env.get(t, "/api/auth/me", cookies)
		if me.Code != http.StatusOK || strings.Contains(me.Body.String(), "usr_") {
			t.Fatalf("%s me = %d %q", username, me.Code, me.Body.String())
		}
	}

	// The same ticker is not a duplicate across accounts.
	for _, test := range []struct {
		cookies map[string]string
		ticker  string
		want    string
	}{
		{cookies: alice, ticker: "AAPL", want: `{"tickers":["AAPL"]}`},
		{cookies: bob, ticker: "AAPL", want: `{"tickers":["AAPL"]}`},
		{cookies: bob, ticker: "MSFT", want: `{"tickers":["AAPL","MSFT"]}`},
	} {
		recorder := env.post(t, "/api/watchlist", `{"ticker":"`+test.ticker+`"}`, test.cookies)
		if recorder.Code != http.StatusCreated || recorder.Body.String() != test.want {
			t.Fatalf("add %s = %d %q", test.ticker, recorder.Code, recorder.Body.String())
		}
	}

	// Adding a ticker twice only fails for the account that already has it.
	recorder := env.post(t, "/api/watchlist", `{"ticker":"AAPL"}`, alice)
	requireError(t, recorder, http.StatusConflict, domain.CodeDuplicateTicker, "AAPL is already in your watchlist.")

	if got := env.get(t, "/api/watchlist", alice); got.Body.String() != `{"tickers":["AAPL"]}` {
		t.Fatalf("alice list = %q", got.Body.String())
	}
	if got := env.get(t, "/api/watchlist", bob); got.Body.String() != `{"tickers":["AAPL","MSFT"]}` {
		t.Fatalf("bob list = %q", got.Body.String())
	}

	// Quotes price only the caller's tickers.
	env.quotes.mu.Lock()
	env.quotes.echoTickers = true
	env.quotes.mu.Unlock()

	aliceQuotes := env.get(t, "/api/quotes", alice)
	if aliceQuotes.Code != http.StatusOK {
		t.Fatalf("alice quotes = %d (%q)", aliceQuotes.Code, aliceQuotes.Body.String())
	}
	if body := decodeBody[quotesBody](t, aliceQuotes); len(body.Quotes) != 1 || body.Quotes[0].Ticker != "AAPL" {
		t.Fatalf("alice quotes = %+v", body.Quotes)
	}
	bobQuotes := env.get(t, "/api/quotes", bob)
	if body := decodeBody[quotesBody](t, bobQuotes); len(body.Quotes) != 2 ||
		body.Quotes[0].Ticker != "AAPL" || body.Quotes[1].Ticker != "MSFT" {
		t.Fatalf("bob quotes = %+v", body.Quotes)
	}

	// A removal only touches the caller's list.
	if got := env.request(t, http.MethodDelete, "/api/watchlist/AAPL", "", bob); got.Code != http.StatusOK ||
		got.Body.String() != `{"tickers":["MSFT"]}` {
		t.Fatalf("bob remove = %d %q", got.Code, got.Body.String())
	}
	if got := env.get(t, "/api/watchlist", alice); got.Body.String() != `{"tickers":["AAPL"]}` {
		t.Fatalf("alice list after bob's removal = %q", got.Body.String())
	}

	// Logging one session out leaves the other usable.
	env.post(t, "/api/auth/logout", "", alice)
	requireError(t, env.get(t, "/api/watchlist", alice), http.StatusUnauthorized,
		domain.CodeUnauthenticated, "Authentication required.")
	if got := env.get(t, "/api/watchlist", bob); got.Code != http.StatusOK {
		t.Fatalf("bob lost his session: %d", got.Code)
	}
}

func TestAuthUsernameIsCaseSensitive(t *testing.T) {
	env := newAuthEnv(t, false)
	const copyText = "Use 3-32 lowercase letters, numbers, dots, dashes or underscores."
	for _, username := range []string{"Owner", "OWNER", "_owner", "Owner.name"} {
		body := `{"username":"` + username + `","password":"` + testPassword + `"}`
		recorder := env.post(t, "/api/auth/setup", body, nil)
		requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidUsername, copyText)
	}
	if env.storage.Users.Count() != 0 {
		t.Fatal("rejected setup stored an account")
	}

	// Surrounding whitespace is trimmed, but the stored name stays lowercase.
	recorder := env.post(t, "/api/auth/setup",
		`{"username":"  owner  ","password":"`+testPassword+`"}`, nil)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("trimmed setup status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	material := decodeBody[setupResponseBody](t, recorder)
	if material.Username != "owner" {
		t.Fatalf("stored username = %q", material.Username)
	}
	env.activate(t, material, responseCookie(t, recorder, enrollCookieName))

	// The owner exists; an uppercase sign-in is a generic credential failure.
	recorder = env.post(t, "/api/auth/login",
		`{"username":"Owner","password":"`+testPassword+`"}`, nil)
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCredentials,
		"Invalid username or password.")
	recorder = env.post(t, "/api/auth/login",
		`{"username":"OWNER","password":"`+testPassword+`"}`, nil)
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCredentials,
		"Invalid username or password.")
	recorder = env.post(t, "/api/auth/login",
		`{"username":" owner ","password":"`+testPassword+`"}`, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("trimmed login = %d (%q)", recorder.Code, recorder.Body.String())
	}
}

func TestAuthLoginChallengeAndTOTP(t *testing.T) {
	env := newAuthEnv(t, false)
	session, material := env.owner(t)
	env.post(t, "/api/auth/logout", "", map[string]string{sessionCookieName: session.Value})

	// Activation consumed the current step; log in on the next one.
	env.clock.Advance(auth.TOTPPeriod)
	recorder := env.post(t, "/api/auth/login", env.credentials(), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	challenge := decodeBody[challengeResponseBody](t, recorder)
	if challenge.Status != "totp_required" || challenge.ExpiresInSeconds != 300 {
		t.Fatalf("challenge = %+v", challenge)
	}
	challengeCookie := responseCookie(t, recorder, challengeCookieName)
	requireCookieFlags(t, challengeCookie, authCookiePath, int(auth.ChallengeTTL/time.Second), false)
	if sessionCookie := findCookie(recorder, sessionCookieName); sessionCookie != nil {
		t.Fatal("password step issued a session")
	}

	// A missing code is a code failure, not a request-format failure.
	recorder = env.post(t, "/api/auth/login/totp", "",
		map[string]string{challengeCookieName: challengeCookie.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCode, "Invalid verification code.")

	// A wrong code keeps the challenge usable but returns generic copy.
	wrong := "000000"
	if env.code(t, material.Secret) == wrong {
		wrong = "999999"
	}
	recorder = env.post(t, "/api/auth/login/totp", `{"code":"`+wrong+`"}`,
		map[string]string{challengeCookieName: challengeCookie.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCode, "Invalid verification code.")

	recorder = env.post(t, "/api/auth/login/totp", `{"code":"`+env.code(t, material.Secret)+`"}`,
		map[string]string{challengeCookieName: challengeCookie.Value})
	if recorder.Code != http.StatusOK {
		t.Fatalf("totp status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	sessionCookie := responseCookie(t, recorder, sessionCookieName)
	requireCookieFlags(t, sessionCookie, sessionCookiePath, int(auth.SessionAbsoluteTTL/time.Second), false)
	requireCleared(t, responseCookie(t, recorder, challengeCookieName))

	user := decodeBody[authUserResponseBody](t, recorder)
	if user.User == nil || user.User.Username != "owner" || !user.User.TOTPEnabled {
		t.Fatalf("user = %+v", user.User)
	}

	// Replay: a fresh challenge cannot reuse a consumed step.
	replayChallenge := env.passwordStep(t)
	recorder = env.post(t, "/api/auth/login/totp", `{"code":"`+env.code(t, material.Secret)+`"}`,
		map[string]string{challengeCookieName: replayChallenge.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCode, "Invalid verification code.")

	// The next time step is accepted.
	env.clock.Advance(auth.TOTPPeriod)
	recorder = env.post(t, "/api/auth/login/totp", `{"code":"`+env.code(t, material.Secret)+`"}`,
		map[string]string{challengeCookieName: replayChallenge.Value})
	if recorder.Code != http.StatusOK {
		t.Fatalf("next-step totp = %d (%q)", recorder.Code, recorder.Body.String())
	}
}

func TestAuthChallengeExpiry(t *testing.T) {
	env := newAuthEnv(t, false)
	env.owner(t)
	env.clock.Advance(auth.TOTPPeriod)
	challenge := env.passwordStep(t)

	env.clock.Advance(auth.ChallengeTTL + time.Second)
	recorder := env.post(t, "/api/auth/login/totp", `{"code":"123456"}`,
		map[string]string{challengeCookieName: challenge.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeChallengeExpired,
		"Your sign-in attempt expired. Sign in again.")
	requireCleared(t, responseCookie(t, recorder, challengeCookieName))
}

func TestAuthChallengeAttemptLimit(t *testing.T) {
	env := newAuthEnv(t, false)
	env.owner(t)
	env.clock.Advance(auth.TOTPPeriod)
	challenge := env.passwordStep(t)

	for attempt := 0; attempt < auth.ChallengeMaxAttempts; attempt++ {
		recorder := env.post(t, "/api/auth/login/totp", `{"code":"000000"}`,
			map[string]string{challengeCookieName: challenge.Value})
		requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCode, "Invalid verification code.")
	}
	recorder := env.post(t, "/api/auth/login/totp", `{"code":"123456"}`,
		map[string]string{challengeCookieName: challenge.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeChallengeExpired,
		"Your sign-in attempt expired. Sign in again.")
}

func TestAuthWrongCredentialsAreGeneric(t *testing.T) {
	env := newAuthEnv(t, false)
	env.owner(t)

	unknown := env.post(t, "/api/auth/login", `{"username":"nobody","password":"some long password"}`, nil)
	requireError(t, unknown, http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid username or password.")
	wrong := env.post(t, "/api/auth/login", `{"username":"owner","password":"some long password"}`, nil)
	requireError(t, wrong, http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid username or password.")
	if unknown.Body.String() != wrong.Body.String() {
		t.Fatalf("bodies differ: %q vs %q", unknown.Body.String(), wrong.Body.String())
	}

	missing := env.post(t, "/api/auth/login", `{"username":"owner"}`, nil)
	requireError(t, missing, http.StatusBadRequest, domain.CodeInvalidRequest, "Enter your username and password.")
	malformed := env.post(t, "/api/auth/login", `{"username":42}`, nil)
	requireError(t, malformed, http.StatusBadRequest, domain.CodeInvalidRequest, "Enter your username and password.")
}

func TestAuthPasswordRateLimit(t *testing.T) {
	env := newAuthEnv(t, false)
	env.owner(t)

	for attempt := 0; attempt < auth.RateFailures; attempt++ {
		recorder := env.post(t, "/api/auth/login", `{"username":"owner","password":"wrong password here"}`, nil)
		requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid username or password.")
	}
	recorder := env.post(t, "/api/auth/login", `{"username":"owner","password":"wrong password here"}`, nil)
	requireError(t, recorder, http.StatusTooManyRequests, domain.CodeRateLimited,
		"Too many requests. Try again in 60 seconds.")
	if got := recorder.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("retry-after = %q", got)
	}

	env.clock.Advance(auth.RateBlock + time.Second)
	recorder = env.post(t, "/api/auth/login", env.credentials(), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login after window = %d (%q)", recorder.Code, recorder.Body.String())
	}
}

func TestAuthRecoveryCodeSingleUse(t *testing.T) {
	env := newAuthEnv(t, false)
	session, material := env.owner(t)
	env.post(t, "/api/auth/logout", "", map[string]string{sessionCookieName: session.Value})

	recovery := material.RecoveryCodes[0]
	messy := strings.ToLower(strings.ReplaceAll(recovery, "-", ""))
	challenge := env.passwordStep(t)
	recorder := env.post(t, "/api/auth/login/recovery", `{"code":"`+messy+`"}`,
		map[string]string{challengeCookieName: challenge.Value})
	if recorder.Code != http.StatusOK {
		t.Fatalf("recovery status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	requireCookieFlags(t, responseCookie(t, recorder, sessionCookieName), sessionCookiePath,
		int(auth.SessionAbsoluteTTL/time.Second), false)
	requireCleared(t, responseCookie(t, recorder, challengeCookieName))

	challenge = env.passwordStep(t)
	recorder = env.post(t, "/api/auth/login/recovery", `{"code":"`+recovery+`"}`,
		map[string]string{challengeCookieName: challenge.Value})
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidRecoveryCode, "Invalid recovery code.")
}

func TestAuthLogoutRevokesSession(t *testing.T) {
	env := newAuthEnv(t, false)
	session, _ := env.owner(t)
	cookies := map[string]string{sessionCookieName: session.Value}

	recorder := env.post(t, "/api/auth/logout", "", cookies)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", recorder.Code)
	}
	cleared := responseCookie(t, recorder, sessionCookieName)
	requireCleared(t, cleared)
	if cleared.Path != "/" {
		t.Fatalf("cleared session cookie path = %q", cleared.Path)
	}
	requireError(t, env.get(t, "/api/watchlist", cookies), http.StatusUnauthorized,
		domain.CodeUnauthenticated, "Authentication required.")
	// Logout is idempotent.
	if again := env.post(t, "/api/auth/logout", "", nil); again.Code != http.StatusNoContent {
		t.Fatalf("second logout = %d", again.Code)
	}
}

func TestAuthLogoutCookieContract(t *testing.T) {
	t.Run("valid session clears every auth cookie", func(t *testing.T) {
		env := newAuthEnv(t, false)
		session, _ := env.owner(t)
		recorder := env.post(t, "/api/auth/logout", "", map[string]string{sessionCookieName: session.Value})
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("logout status = %d", recorder.Code)
		}
		requireCleared(t, responseCookie(t, recorder, sessionCookieName))
		requireCleared(t, responseCookie(t, recorder, challengeCookieName))
		requireCleared(t, responseCookie(t, recorder, enrollCookieName))
	})

	t.Run("no session cookie gets no clearing sd_session", func(t *testing.T) {
		env := newAuthEnv(t, false)
		recorder := env.post(t, "/api/auth/logout", "", nil)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("logout status = %d", recorder.Code)
		}
		if cleared := findCookie(recorder, sessionCookieName); cleared != nil {
			t.Fatalf("logout cleared sd_session although the request carried none: %+v", cleared)
		}
		requireCleared(t, responseCookie(t, recorder, challengeCookieName))
		requireCleared(t, responseCookie(t, recorder, enrollCookieName))
	})

	t.Run("stale challenge and enrollment cookies are cleared", func(t *testing.T) {
		env := newAuthEnv(t, false)
		recorder := env.post(t, "/api/auth/logout", "", map[string]string{
			challengeCookieName: "stale-challenge",
			enrollCookieName:    "stale-enrollment",
		})
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("logout status = %d", recorder.Code)
		}
		if cleared := findCookie(recorder, sessionCookieName); cleared != nil {
			t.Fatalf("logout cleared sd_session although the request carried none: %+v", cleared)
		}
		requireCleared(t, responseCookie(t, recorder, challengeCookieName))
		requireCleared(t, responseCookie(t, recorder, enrollCookieName))
	})
}

func TestAuthMeUsesSessionFromContext(t *testing.T) {
	calls := &atomic.Int32{}
	env := newTestEnvWithAuth(t, newFakeStore(), &fakeQuotes{}, completeKeys(), &countingAuth{calls: calls})

	recorder := env.do(t, http.MethodGet, "/api/auth/me", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("me status = %d (%q)", recorder.Code, recorder.Body.String())
	}
	if got, want := recorder.Body.String(), `{"user":{"username":"owner","totpEnabled":true}}`; got != want {
		t.Fatalf("me body = %q, want %q", got, want)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("Authenticate calls = %d, want 1 (requireSession owns authentication)", got)
	}
}

func TestAuthSessionExpiry(t *testing.T) {
	env := newAuthEnv(t, false)
	session, material := env.owner(t)
	cookies := map[string]string{sessionCookieName: session.Value}

	env.clock.Advance(auth.SessionIdleTTL - time.Minute)
	if recorder := env.get(t, "/api/auth/me", cookies); recorder.Code != http.StatusOK {
		t.Fatalf("me after 29m = %d", recorder.Code)
	}
	env.clock.Advance(auth.SessionIdleTTL - time.Minute)
	if recorder := env.get(t, "/api/auth/me", cookies); recorder.Code != http.StatusOK {
		t.Fatalf("idle window did not slide: %d", recorder.Code)
	}
	env.clock.Advance(auth.SessionIdleTTL + time.Minute)
	recorder := env.get(t, "/api/watchlist", cookies)
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeUnauthenticated, "Authentication required.")
	requireCleared(t, responseCookie(t, recorder, sessionCookieName))

	fresh := env.loginSession(t, material.Secret)
	freshCookies := map[string]string{sessionCookieName: fresh.Value}
	env.clock.Advance(auth.SessionAbsoluteTTL + time.Minute)
	requireError(t, env.get(t, "/api/quotes", freshCookies), http.StatusUnauthorized,
		domain.CodeUnauthenticated, "Authentication required.")
}

// A cross-site request with no session cookie must not be able to sign the user
// out. Only a cookie the request actually presented may be expired.
func TestUnauthenticatedRequestOnlyClearsPresentedCookie(t *testing.T) {
	env := newAuthEnv(t, false)
	tests := []struct {
		name        string
		cookie      *string
		wantCleared bool
	}{
		{name: "no cookie presented", cookie: nil, wantCleared: false},
		{name: "unknown token is cleared", cookie: pointerTo("not-a-live-session"), wantCleared: true},
		{name: "empty value is cleared", cookie: pointerTo(""), wantCleared: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var cookies map[string]string
			if test.cookie != nil {
				cookies = map[string]string{sessionCookieName: *test.cookie}
			}
			recorder := env.get(t, "/api/watchlist", cookies)
			requireError(t, recorder, http.StatusUnauthorized, domain.CodeUnauthenticated,
				"Authentication required.")
			cleared := findCookie(recorder, sessionCookieName)
			if !test.wantCleared {
				if cleared != nil {
					t.Fatal("response cleared sd_session although the request carried none")
				}
				return
			}
			if cleared == nil {
				t.Fatalf("presented cookie was not cleared; headers %v",
					recorder.Header().Values("Set-Cookie"))
			}
			requireCleared(t, cleared)
		})
	}
}

func pointerTo(value string) *string { return &value }

func TestAuthCookieSecureFlag(t *testing.T) {
	env := newAuthEnv(t, true)
	material, enroll, _ := env.beginSetup(t)
	requireCookieFlags(t, enroll, authCookiePath, int(auth.EnrollmentTTL/time.Second), true)
	session := env.activate(t, material, enroll)
	requireCookieFlags(t, session, sessionCookiePath, int(auth.SessionAbsoluteTTL/time.Second), true)
}

func TestHostGuardRejectsDNSRebindingHosts(t *testing.T) {
	env := newAuthEnv(t, false)

	for _, host := range []string{"evil.example:3000", "127.0.0.1.evil:3000", "", "localhost.evil"} {
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3000/healthz", nil)
		request.Host = host
		recorder := httptest.NewRecorder()
		env.handler.ServeHTTP(recorder, request)
		requireError(t, recorder, http.StatusForbidden, domain.CodeCrossOriginRequest,
			"Cross-origin requests are not allowed.")
	}

	for _, host := range []string{"127.0.0.1:3000", "localhost:3000", "LOCALHOST:5173", "[::1]:3000"} {
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3000/healthz", nil)
		request.Host = host
		recorder := httptest.NewRecorder()
		env.handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", host, recorder.Code)
		}
	}
}

func TestCrossOriginGuard(t *testing.T) {
	env := newAuthEnv(t, false)
	env.owner(t)

	post := func(target, origin string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3000"+target, strings.NewReader(`{"username":"owner","password":"wrong password here"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", origin)
		recorder := httptest.NewRecorder()
		env.handler.ServeHTTP(recorder, request)
		return recorder
	}

	for _, origin := range []string{"https://evil.example", "http://evil.example", "null", "http://127.0.0.1:3000.evil"} {
		recorder := post("/api/auth/login", origin)
		requireError(t, recorder, http.StatusForbidden, domain.CodeCrossOriginRequest,
			"Cross-origin requests are not allowed.")
	}
	for _, origin := range []string{"http://127.0.0.1:3000", "https://127.0.0.1:3000", "HTTP://127.0.0.1:3000"} {
		recorder := post("/api/auth/login", origin)
		requireError(t, recorder, http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid username or password.")
	}

	// The same guard covers every unsafe /api path, including the watchlist.
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/watchlist", strings.NewReader(`{"ticker":"AAPL"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	recorder := httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)
	requireError(t, recorder, http.StatusForbidden, domain.CodeCrossOriginRequest,
		"Cross-origin requests are not allowed.")

	// Safe methods are not origin-gated.
	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3000/api/watchlist", nil)
	request.Header.Set("Origin", "https://evil.example")
	recorder = httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)
	requireError(t, recorder, http.StatusUnauthorized, domain.CodeUnauthenticated, "Authentication required.")
}

func TestAuthContentTypeRequired(t *testing.T) {
	env := newAuthEnv(t, false)
	session, _ := env.owner(t)

	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/auth/login", strings.NewReader(`{"username":"owner","password":"x"}`))
	request.Header.Set("Content-Type", "text/plain")
	recorder := httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)
	requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidRequest, "Enter your username and password.")

	request = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/watchlist", strings.NewReader(`{"ticker":"AAPL"}`))
	request.Header.Set("Content-Type", "text/plain")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.Value})
	recorder = httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)
	requireError(t, recorder, http.StatusBadRequest, domain.CodeInvalidTicker, "Enter a valid ticker, e.g. AAPL.")
}

func TestProtectedEndpointsDenyWithoutSession(t *testing.T) {
	env := newTestEnvWithAuth(t, newFakeStore("AAPL"), &fakeQuotes{}, completeKeys(), denyAllAuth{})
	for _, request := range []struct {
		method string
		target string
		body   string
	}{
		{method: http.MethodGet, target: "/api/watchlist"},
		{method: http.MethodPost, target: "/api/watchlist", body: `{"ticker":"AAPL"}`},
		{method: http.MethodDelete, target: "/api/watchlist/AAPL"},
		{method: http.MethodGet, target: "/api/quotes"},
		{method: http.MethodGet, target: "/api/chart/AAPL"},
		{method: http.MethodGet, target: "/api/auth/me"},
	} {
		recorder := env.do(t, request.method, request.target, request.body)
		requireError(t, recorder, http.StatusUnauthorized, domain.CodeUnauthenticated, "Authentication required.")
		if cleared := findCookie(recorder, sessionCookieName); cleared != nil {
			t.Fatalf("%s %s cleared sd_session although the request carried none", request.method, request.target)
		}
	}

	status := env.do(t, http.MethodGet, "/api/auth/status", "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"authenticated":false`) {
		t.Fatalf("status = %d %q", status.Code, status.Body.String())
	}
	for _, target := range []string{"/healthz", "/", "/chart/AAPL"} {
		if recorder := env.do(t, http.MethodGet, target, ""); recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", target, recorder.Code)
		}
	}
}

func TestAuthResponsesAndLogsHoldNoSecrets(t *testing.T) {
	env := newAuthEnv(t, false)
	material, enroll, _ := env.beginSetup(t)

	// A failed login and a failed code before the successful flow.
	env.post(t, "/api/auth/login", `{"username":"owner","password":"wrong password here"}`, nil)
	env.post(t, "/api/auth/2fa/activate", `{"code":"000000"}`,
		map[string]string{enrollCookieName: enroll.Value})

	session := env.activate(t, material, enroll)
	env.post(t, "/api/auth/logout", "", map[string]string{sessionCookieName: session.Value})

	secrets := []string{testPassword, material.Secret, enroll.Value, session.Value, "wrong password here"}
	secrets = append(secrets, material.RecoveryCodes...)
	for _, secret := range secrets {
		if strings.Contains(env.logs.String(), secret) {
			t.Fatalf("logs leaked a secret: %s", env.logs.String())
		}
	}

	logs := env.logs.String()
	for _, event := range []string{"setup_pending", "activation_failed", "owner_activated", "login_failed", "logout"} {
		if !strings.Contains(logs, `"event":"`+event+`"`) {
			t.Fatalf("missing auth event %q in %s", event, logs)
		}
	}
	if !strings.Contains(logs, `"username":"owner"`) {
		t.Fatalf("auth event missing username: %s", logs)
	}
	if !strings.Contains(logs, `"request_id":"`) {
		t.Fatalf("auth event missing request_id: %s", logs)
	}
}
