package app

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"stock-dashboard/server/internal/config"
	"stock-dashboard/server/internal/domain"
	"stock-dashboard/server/internal/store"
)

const accountPassword = "correct horse battery"

// testPBKDF2Iterations keeps the composition-root tests fast.
const testPBKDF2Iterations uint32 = 1_000

type providerFake struct {
	server        *httptest.Server
	finnhubCalls  atomic.Int32
	twelveCalls   atomic.Int32
	timeSeriesHit atomic.Int32
	acceptAll     atomic.Bool
	failFinnhub   atomic.Bool
}

func newProviderFake(t *testing.T) *providerFake {
	t.Helper()
	fake := &providerFake{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/quote", func(w http.ResponseWriter, r *http.Request) {
		fake.finnhubCalls.Add(1)
		if fake.failFinnhub.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("symbol") != "AAPL" && !fake.acceptAll.Load() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"c":190.12,"d":1.24,"dp":0.66,"pc":188.88,"t":1763303400}`)
	})
	mux.HandleFunc("/quote", func(w http.ResponseWriter, _ *http.Request) {
		fake.twelveCalls.Add(1)
		_, _ = io.WriteString(w, `{"close":190.12,"previous_close":188.88,"timestamp":1763303400}`)
	})
	mux.HandleFunc("/time_series", func(w http.ResponseWriter, _ *http.Request) {
		fake.timeSeriesHit.Add(1)
		_, _ = io.WriteString(w, `{"values":[{"datetime":"2026-11-14","close":191.2},{"datetime":"2026-11-13","close":190.1}],"status":"ok"}`)
	})
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func testConfig(t *testing.T, fake *providerFake) config.Config {
	t.Helper()
	if fake == nil {
		return config.Config{
			Port:              3000,
			DataDir:           t.TempDir(),
			FinnhubBaseURL:    "https://finnhub.invalid",
			TwelveDataBaseURL: "https://twelvedata.invalid",
		}
	}
	return config.Config{
		FinnhubAPIKey:     "fin-key",
		TwelveDataAPIKey:  "td-key",
		Port:              3000,
		DataDir:           t.TempDir(),
		FinnhubBaseURL:    fake.server.URL,
		TwelveDataBaseURL: fake.server.URL,
	}
}

func newTestServer(t *testing.T, cfg config.Config) *httptest.Server {
	t.Helper()
	application, err := NewWithOptions(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		Options{PBKDF2Iterations: testPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = application.Close() })
	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)
	return server
}

func call(t *testing.T, server *httptest.Server, method, target, body string) *http.Response {
	t.Helper()
	return callWith(t, server, nil, method, target, body)
}

func callWith(t *testing.T, server *httptest.Server, session *http.Cookie, method, target, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, server.URL+target, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if session != nil {
		request.AddCookie(session)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func responseCookie(t *testing.T, response *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response has no %s cookie", name)
	return nil
}

type account struct {
	username      string
	password      string
	secret        string
	recoveryCodes []string
}

// setupOwner runs first-run setup and TOTP activation and returns the session.
func setupOwner(t *testing.T, server *httptest.Server) (*account, *http.Cookie) {
	t.Helper()
	body := `{"username":"owner","password":"` + accountPassword + `"}`
	response := call(t, server, http.MethodPost, "/api/auth/setup", body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("setup status = %d (%s)", response.StatusCode, readBody(t, response))
	}
	var material struct {
		Secret        string   `json:"secret"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	if err := json.Unmarshal([]byte(readBody(t, response)), &material); err != nil {
		t.Fatal(err)
	}
	enroll := responseCookie(t, response, "sd_enroll")

	code, err := totp.GenerateCode(material.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	response = callWith(t, server, enroll, http.MethodPost, "/api/auth/2fa/activate",
		`{"code":"`+code+`"}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("activate status = %d (%s)", response.StatusCode, readBody(t, response))
	}
	session := responseCookie(t, response, "sd_session")
	return &account{
		username:      "owner",
		password:      accountPassword,
		secret:        material.Secret,
		recoveryCodes: material.RecoveryCodes,
	}, session
}

// loginWithRecovery signs an existing owner in with a single-use recovery code,
// which keeps app tests free of TOTP replay coupling.
func loginWithRecovery(t *testing.T, server *httptest.Server, owner *account) *http.Cookie {
	t.Helper()
	body := `{"username":"` + owner.username + `","password":"` + owner.password + `"}`
	response := call(t, server, http.MethodPost, "/api/auth/login", body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d (%s)", response.StatusCode, readBody(t, response))
	}
	challenge := responseCookie(t, response, "sd_challenge")
	if len(owner.recoveryCodes) == 0 {
		t.Fatal("no recovery codes left")
	}
	code := owner.recoveryCodes[0]
	owner.recoveryCodes = owner.recoveryCodes[1:]
	response = callWith(t, server, challenge, http.MethodPost, "/api/auth/login/recovery",
		`{"code":"`+code+`"}`)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("recovery status = %d (%s)", response.StatusCode, readBody(t, response))
	}
	return responseCookie(t, response, "sd_session")
}

// seedWatchlist writes through the store API so the budget tests exercise the
// same SQLite path as production. The account must already exist.
func seedWatchlist(t *testing.T, dataDir, username string, tickers []string) {
	t.Helper()
	storage, err := store.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	account := storage.Users.FindByUsername(username)
	if account == nil {
		t.Fatalf("seed account %q is missing", username)
	}
	if _, err := storage.Watchlist.Update(account.ID, func(current domain.Watchlist) (domain.Watchlist, error) {
		var addErr error
		for _, ticker := range tickers {
			current, addErr = current.Add(ticker)
			if addErr != nil {
				return domain.Watchlist{}, addErr
			}
		}
		return current, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func tickerRange(count int) []string {
	tickers := make([]string, count)
	for index := range tickers {
		tickers[index] = fmt.Sprintf("T%02d", index)
	}
	return tickers
}

func TestEndToEndWithFakeProviders(t *testing.T) {
	fake := newProviderFake(t)
	cfg := testConfig(t, fake)
	server := newTestServer(t, cfg)

	health := call(t, server, http.MethodGet, "/healthz", "")
	if health.StatusCode != http.StatusOK || readBody(t, health) != `{"status":"ok"}` {
		t.Fatalf("health failed")
	}

	// Data endpoints are closed before setup.
	if response := call(t, server, http.MethodGet, "/api/watchlist", ""); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated watchlist = %d", response.StatusCode)
	}

	owner, session := setupOwner(t, server)

	add := callWith(t, server, session, http.MethodPost, "/api/watchlist", `{"ticker":"aapl"}`)
	if add.StatusCode != http.StatusCreated {
		t.Fatalf("add status = %d (%s)", add.StatusCode, readBody(t, add))
	}
	if body := readBody(t, add); body != `{"tickers":["AAPL"]}` {
		t.Fatalf("add body = %s", body)
	}
	if fake.finnhubCalls.Load() != 1 {
		t.Fatalf("verification calls = %d", fake.finnhubCalls.Load())
	}

	quotes := callWith(t, server, session, http.MethodGet, "/api/quotes", "")
	if quotes.StatusCode != http.StatusOK {
		t.Fatalf("quotes status = %d", quotes.StatusCode)
	}
	var quotesBody struct {
		Quotes []struct {
			Ticker    string  `json:"ticker"`
			Price     float64 `json:"price"`
			Change    float64 `json:"change"`
			Direction string  `json:"direction"`
			Provider  string  `json:"provider"`
		} `json:"quotes"`
	}
	if err := json.Unmarshal([]byte(readBody(t, quotes)), &quotesBody); err != nil {
		t.Fatal(err)
	}
	if len(quotesBody.Quotes) != 1 || quotesBody.Quotes[0].Ticker != "AAPL" {
		t.Fatalf("quotes = %+v", quotesBody.Quotes)
	}
	if quotesBody.Quotes[0].Price != 190.12 || quotesBody.Quotes[0].Change != 1.24 || quotesBody.Quotes[0].Direction != "up" {
		t.Fatalf("quote = %+v", quotesBody.Quotes[0])
	}
	if fake.finnhubCalls.Load() != 1 {
		t.Fatalf("quote cache miss: finnhub calls = %d", fake.finnhubCalls.Load())
	}

	chart := callWith(t, server, session, http.MethodGet, "/api/chart/AAPL?range=1M", "")
	if chart.StatusCode != http.StatusOK {
		t.Fatalf("chart status = %d (%s)", chart.StatusCode, readBody(t, chart))
	}
	var chartBody struct {
		Ticker string `json:"ticker"`
		Range  string `json:"range"`
		Source string `json:"source"`
		Points []struct {
			Date  string  `json:"date"`
			Close float64 `json:"close"`
		} `json:"points"`
	}
	if err := json.Unmarshal([]byte(readBody(t, chart)), &chartBody); err != nil {
		t.Fatal(err)
	}
	if chartBody.Source != "Twelve Data" || len(chartBody.Points) != 2 {
		t.Fatalf("chart = %+v", chartBody)
	}
	if chartBody.Points[0].Date != "2026-11-13" || chartBody.Points[1].Date != "2026-11-14" {
		t.Fatalf("points not oldest first: %+v", chartBody.Points)
	}
	if fake.timeSeriesHit.Load() != 1 {
		t.Fatalf("time series calls = %d", fake.timeSeriesHit.Load())
	}

	me := callWith(t, server, session, http.MethodGet, "/api/auth/me", "")
	if me.StatusCode != http.StatusOK {
		t.Fatalf("me status = %d", me.StatusCode)
	}

	logout := callWith(t, server, session, http.MethodPost, "/api/auth/logout", "")
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d", logout.StatusCode)
	}

	session = loginWithRecovery(t, server, owner)
	list := callWith(t, server, session, http.MethodGet, "/api/watchlist", "")
	if body := readBody(t, list); body != `{"tickers":["AAPL"]}` {
		t.Fatalf("watchlist after re-login = %s", body)
	}

	spa := call(t, server, http.MethodGet, "/chart/AAPL", "")
	if spa.StatusCode != http.StatusOK {
		t.Fatalf("deep link status = %d", spa.StatusCode)
	}
	if contentType := spa.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("deep link content type = %q", contentType)
	}
	if shell := readBody(t, spa); !strings.Contains(strings.ToLower(shell), "<html") {
		t.Fatalf("deep link did not serve HTML: %q", shell)
	}
}

func TestWatchlistSurvivesRestart(t *testing.T) {
	fake := newProviderFake(t)
	cfg := testConfig(t, fake)
	first := newTestServer(t, cfg)
	owner, session := setupOwner(t, first)

	add := callWith(t, first, session, http.MethodPost, "/api/watchlist", `{"ticker":"AAPL"}`)
	if add.StatusCode != http.StatusCreated {
		t.Fatalf("add status = %d", add.StatusCode)
	}

	// Sessions live in memory; the account and watchlist survive a restart.
	second := newTestServer(t, cfg)
	status := call(t, second, http.MethodGet, "/api/auth/status", "")
	if body := readBody(t, status); !strings.Contains(body, `"setupRequired":false`) {
		t.Fatalf("status after restart = %s", body)
	}
	secondSession := loginWithRecovery(t, second, owner)
	list := callWith(t, second, secondSession, http.MethodGet, "/api/watchlist", "")
	if body := readBody(t, list); body != `{"tickers":["AAPL"]}` {
		t.Fatalf("reloaded watchlist = %s", body)
	}
}

func TestMissingKeysKeepHealthGreen(t *testing.T) {
	cfg := testConfig(t, nil)
	server := newTestServer(t, cfg)

	health := call(t, server, http.MethodGet, "/healthz", "")
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", health.StatusCode)
	}

	_, session := setupOwner(t, server)
	for _, target := range []string{"/api/quotes", "/api/chart/AAPL"} {
		response := callWith(t, server, session, http.MethodGet, target, "")
		if response.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d", target, response.StatusCode)
		}
		if !strings.Contains(readBody(t, response), "MISSING_API_KEY") {
			t.Fatalf("%s did not return MISSING_API_KEY", target)
		}
	}
}

func TestFinnhubBudgetCapsOneBatchAtFiftyFive(t *testing.T) {
	fake := newProviderFake(t)
	fake.acceptAll.Store(true)
	cfg := testConfig(t, fake)
	server := newTestServer(t, cfg)
	_, session := setupOwner(t, server)
	seedWatchlist(t, cfg.DataDir, "owner", tickerRange(60))

	response := callWith(t, server, session, http.MethodGet, "/api/quotes", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d (%s)", response.StatusCode, readBody(t, response))
	}
	var body struct {
		Quotes []struct {
			Ticker string `json:"ticker"`
		} `json:"quotes"`
	}
	if err := json.Unmarshal([]byte(readBody(t, response)), &body); err != nil {
		t.Fatal(err)
	}
	if got := fake.finnhubCalls.Load(); got > 55 {
		t.Fatalf("finnhub calls = %d, want <= 55", got)
	}
	// The five refused primary calls fall through to the (still free) Twelve Data budget.
	if len(body.Quotes) != 60 {
		t.Fatalf("quotes = %d, want 60", len(body.Quotes))
	}
}

func TestTwelveDataBudgetCapsFallbackAtSeven(t *testing.T) {
	fake := newProviderFake(t)
	fake.failFinnhub.Store(true)
	cfg := testConfig(t, fake)
	server := newTestServer(t, cfg)
	_, session := setupOwner(t, server)
	seedWatchlist(t, cfg.DataDir, "owner", tickerRange(10))

	response := callWith(t, server, session, http.MethodGet, "/api/quotes", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d (%s)", response.StatusCode, readBody(t, response))
	}
	var body struct {
		Quotes []struct {
			Provider string `json:"provider"`
		} `json:"quotes"`
	}
	if err := json.Unmarshal([]byte(readBody(t, response)), &body); err != nil {
		t.Fatal(err)
	}
	if got := fake.twelveCalls.Load(); got > 7 {
		t.Fatalf("twelve data calls = %d, want <= 7", got)
	}
	if len(body.Quotes) != 7 {
		t.Fatalf("quotes = %d, want 7", len(body.Quotes))
	}
	for _, quote := range body.Quotes {
		if quote.Provider != "Twelve Data" {
			t.Fatalf("provider = %q", quote.Provider)
		}
	}
}
