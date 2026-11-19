package config

import (
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 3000 {
		t.Fatalf("port = %d", cfg.Port)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("data dir = %q", cfg.DataDir)
	}
	if cfg.BindHost != "127.0.0.1" {
		t.Fatalf("bind host = %q", cfg.BindHost)
	}
	if cfg.FinnhubBaseURL != "https://finnhub.io" {
		t.Fatalf("finnhub base = %q", cfg.FinnhubBaseURL)
	}
	if cfg.TwelveDataBaseURL != "https://api.twelvedata.com" {
		t.Fatalf("twelve data base = %q", cfg.TwelveDataBaseURL)
	}
	if cfg.FinnhubAPIKey != "" || cfg.TwelveDataAPIKey != "" {
		t.Fatal("keys should default to empty")
	}
}

func TestLoadCustom(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"FINNHUB_API_KEY":     "  fin-key  ",
		"TWELVEDATA_API_KEY":  "td-key",
		"PORT":                "8081",
		"DATA_DIR":            `C:\tmp\data`,
		"BIND_HOST":           "0.0.0.0",
		"FINNHUB_BASE_URL":    "http://127.0.0.1:9001/",
		"TWELVEDATA_BASE_URL": "http://127.0.0.1:9002/td/",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FinnhubAPIKey != "fin-key" {
		t.Fatalf("finnhub key = %q", cfg.FinnhubAPIKey)
	}
	if cfg.TwelveDataAPIKey != "td-key" {
		t.Fatalf("twelve data key = %q", cfg.TwelveDataAPIKey)
	}
	if cfg.Port != 8081 {
		t.Fatalf("port = %d", cfg.Port)
	}
	if cfg.DataDir != `C:\tmp\data` {
		t.Fatalf("data dir = %q", cfg.DataDir)
	}
	if cfg.BindHost != "0.0.0.0" {
		t.Fatalf("bind host = %q", cfg.BindHost)
	}
	if cfg.FinnhubBaseURL != "http://127.0.0.1:9001" {
		t.Fatalf("finnhub base = %q", cfg.FinnhubBaseURL)
	}
	if cfg.TwelveDataBaseURL != "http://127.0.0.1:9002/td" {
		t.Fatalf("twelve data base = %q", cfg.TwelveDataBaseURL)
	}
}

func TestLoadBindHostValidation(t *testing.T) {
	for _, raw := range []string{"localhost", "127.0.0.1:3000", "not-an-ip"} {
		if _, err := Load(env(map[string]string{"BIND_HOST": raw})); err == nil {
			t.Fatalf("BIND_HOST=%q accepted", raw)
		}
	}
	cfg, err := Load(env(map[string]string{"BIND_HOST": "  ::1  "}))
	if err != nil || cfg.BindHost != "::1" {
		t.Fatalf("loopback bind = %q, %v", cfg.BindHost, err)
	}
}

func TestLoadPortValidation(t *testing.T) {
	for _, raw := range []string{"0", "-1", "65536", "abc", "3000.5", "  "} {
		if raw == "  " {
			continue
		}
		_, err := Load(env(map[string]string{"PORT": raw}))
		if err == nil {
			t.Fatalf("PORT=%q accepted", raw)
		}
		if !strings.Contains(err.Error(), "PORT") {
			t.Fatalf("PORT=%q error = %v, want PORT copy", raw, err)
		}
	}

	cfg, err := Load(env(map[string]string{"PORT": "  "}))
	if err != nil || cfg.Port != 3000 {
		t.Fatalf("blank PORT = %d, %v; want default", cfg.Port, err)
	}
}

func TestLoadURLValidation(t *testing.T) {
	tests := []struct {
		name string
		env  string
		raw  string
	}{
		{name: "scheme", env: "FINNHUB_BASE_URL", raw: "ftp://example.com"},
		{name: "relative", env: "FINNHUB_BASE_URL", raw: "example.com"},
		{name: "missing host", env: "TWELVEDATA_BASE_URL", raw: "https:///quote"},
		{name: "query", env: "TWELVEDATA_BASE_URL", raw: "https://example.com?token=x"},
		{name: "fragment", env: "FINNHUB_BASE_URL", raw: "https://example.com/#x"},
		{name: "credentials", env: "FINNHUB_BASE_URL", raw: "https://user:pass@example.com"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(env(map[string]string{test.env: test.raw}))
			if err == nil {
				t.Fatalf("%s=%q accepted", test.env, test.raw)
			}
		})
	}
}

func TestAuthCookieSecure(t *testing.T) {
	tests := map[string]bool{
		"":      false,
		"true":  true,
		"false": false,
		"TRUE":  true,
		"False": false,
	}
	for raw, want := range tests {
		cfg, err := Load(env(map[string]string{"AUTH_COOKIE_SECURE": raw}))
		if err != nil {
			t.Fatalf("AUTH_COOKIE_SECURE=%q error = %v", raw, err)
		}
		if cfg.AuthCookieSecure != want {
			t.Fatalf("AUTH_COOKIE_SECURE=%q = %v, want %v", raw, cfg.AuthCookieSecure, want)
		}
	}
	// Only the literals true/false are documented; Go's ParseBool extensions are
	// rejected so the error message stays truthful.
	for _, raw := range []string{"1", "0", "t", "f", "yes", "on", "maybe"} {
		_, err := Load(env(map[string]string{"AUTH_COOKIE_SECURE": raw}))
		if err == nil || !strings.Contains(err.Error(), "AUTH_COOKIE_SECURE") {
			t.Fatalf("AUTH_COOKIE_SECURE=%q error = %v", raw, err)
		}
	}
}

func TestAllowedHosts(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AllowedHosts) != 3 || cfg.AllowedHosts[0] != "127.0.0.1" || cfg.AllowedHosts[2] != "::1" {
		t.Fatalf("default allowed hosts = %v", cfg.AllowedHosts)
	}

	cfg, err = Load(env(map[string]string{"ALLOWED_HOSTS": " Example.Test , 127.0.0.2, ::1 "}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"example.test", "127.0.0.2", "::1"}
	if len(cfg.AllowedHosts) != len(want) {
		t.Fatalf("allowed hosts = %v", cfg.AllowedHosts)
	}
	for index := range want {
		if cfg.AllowedHosts[index] != want[index] {
			t.Fatalf("allowed hosts = %v, want %v", cfg.AllowedHosts, want)
		}
	}

	for _, raw := range []string{"http://example.test", "example.test:3000", "bad host", "host/path", ","} {
		if _, err := Load(env(map[string]string{"ALLOWED_HOSTS": raw})); err == nil {
			t.Fatalf("ALLOWED_HOSTS=%q accepted", raw)
		}
	}
}

func TestProviderOverrides(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FinnhubRPM != 55 || cfg.TwelveDataRPM != 7 || cfg.UpstreamTimeout != 2*time.Second || cfg.QuoteCacheTTL != time.Minute {
		t.Fatalf("provider defaults = %+v", cfg)
	}

	cfg, err = Load(env(map[string]string{
		"FINNHUB_RPM":      "12",
		"TWELVEDATA_RPM":   "3",
		"UPSTREAM_TIMEOUT": "750ms",
		"QUOTE_CACHE_TTL":  "2m",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FinnhubRPM != 12 || cfg.TwelveDataRPM != 3 || cfg.UpstreamTimeout != 750*time.Millisecond || cfg.QuoteCacheTTL != 2*time.Minute {
		t.Fatalf("provider overrides = %+v", cfg)
	}

	for key, raw := range map[string]string{
		"FINNHUB_RPM":      "0",
		"TWELVEDATA_RPM":   "nope",
		"UPSTREAM_TIMEOUT": "-1s",
		"QUOTE_CACHE_TTL":  "soon",
	} {
		if _, err := Load(env(map[string]string{key: raw})); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("%s=%q error = %v", key, raw, err)
		}
	}
}
