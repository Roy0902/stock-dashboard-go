package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"stock-dashboard/server/internal/providers"
)

const (
	defaultPort              = 3000
	defaultDataDir           = "./data"
	defaultBindHost          = "127.0.0.1"
	defaultFinnhubBaseURL    = "https://finnhub.io"
	defaultTwelveDataBaseURL = "https://api.twelvedata.com"
	minPort                  = 1
	maxPort                  = 65535
)

var defaultAllowedHosts = []string{"127.0.0.1", "localhost", "::1"}

// Config is the validated process configuration. Empty API keys mean data endpoints
// are unavailable but the process still starts.
type Config struct {
	FinnhubAPIKey     string
	TwelveDataAPIKey  string
	Port              int
	DataDir           string
	BindHost          string
	FinnhubBaseURL    string
	TwelveDataBaseURL string
	// AuthCookieSecure sets the Secure flag on auth cookies (HTTPS deployments).
	AuthCookieSecure bool
	// AllowedHosts are the Host header values the API accepts.
	AllowedHosts []string
	// Provider tuning; zero values fall back to providers.DefaultLimits.
	FinnhubRPM      int
	TwelveDataRPM   int
	UpstreamTimeout time.Duration
	QuoteCacheTTL   time.Duration
}

// Load reads configuration through getenv so tests can supply a map-like function.
func Load(getenv func(string) string) (Config, error) {
	port, err := parsePort(strings.TrimSpace(getenv("PORT")))
	if err != nil {
		return Config{}, err
	}

	dataDir := strings.TrimSpace(getenv("DATA_DIR"))
	if dataDir == "" {
		dataDir = defaultDataDir
	}

	bindHost, err := parseBindHost(getenv("BIND_HOST"))
	if err != nil {
		return Config{}, err
	}

	authCookieSecure, err := parseBool("AUTH_COOKIE_SECURE", getenv("AUTH_COOKIE_SECURE"))
	if err != nil {
		return Config{}, err
	}

	finnhubBaseURL, err := parseBaseURL("FINNHUB_BASE_URL", getenv("FINNHUB_BASE_URL"), defaultFinnhubBaseURL)
	if err != nil {
		return Config{}, err
	}
	twelveDataBaseURL, err := parseBaseURL("TWELVEDATA_BASE_URL", getenv("TWELVEDATA_BASE_URL"), defaultTwelveDataBaseURL)
	if err != nil {
		return Config{}, err
	}

	allowedHosts, err := parseAllowedHosts(getenv("ALLOWED_HOSTS"))
	if err != nil {
		return Config{}, err
	}
	finnhubRPM, err := parsePositiveInt("FINNHUB_RPM", getenv("FINNHUB_RPM"), providers.DefaultLimits.FinnhubRPM)
	if err != nil {
		return Config{}, err
	}
	twelveDataRPM, err := parsePositiveInt("TWELVEDATA_RPM", getenv("TWELVEDATA_RPM"), providers.DefaultLimits.TwelveDataRPM)
	if err != nil {
		return Config{}, err
	}
	upstreamTimeout, err := parsePositiveDuration("UPSTREAM_TIMEOUT", getenv("UPSTREAM_TIMEOUT"), providers.DefaultLimits.UpstreamTimeout)
	if err != nil {
		return Config{}, err
	}
	quoteCacheTTL, err := parsePositiveDuration("QUOTE_CACHE_TTL", getenv("QUOTE_CACHE_TTL"), providers.DefaultLimits.CacheTTL)
	if err != nil {
		return Config{}, err
	}

	return Config{
		FinnhubAPIKey:     strings.TrimSpace(getenv("FINNHUB_API_KEY")),
		TwelveDataAPIKey:  strings.TrimSpace(getenv("TWELVEDATA_API_KEY")),
		Port:              port,
		DataDir:           dataDir,
		BindHost:          bindHost,
		FinnhubBaseURL:    finnhubBaseURL,
		TwelveDataBaseURL: twelveDataBaseURL,
		AuthCookieSecure:  authCookieSecure,
		AllowedHosts:      allowedHosts,
		FinnhubRPM:        finnhubRPM,
		TwelveDataRPM:     twelveDataRPM,
		UpstreamTimeout:   upstreamTimeout,
		QuoteCacheTTL:     quoteCacheTTL,
	}, nil
}

// parseBool reads an optional boolean. Blank means the default (false). Only the
// literal true/false (case-insensitive) is accepted, matching the error message
// and the documented values.
func parseBool(name, raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return false, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("config: %s must be true or false", name)
	}
}

func parsePort(raw string) (int, error) {
	if raw == "" {
		return defaultPort, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < minPort || port > maxPort {
		return 0, fmt.Errorf("config: PORT must be an integer between %d and %d", minPort, maxPort)
	}
	return port, nil
}

func parseBaseURL(name, raw, fallback string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("config: %s is not a valid URL: %w", name, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("config: %s must use http or https", name)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("config: %s must include a host", name)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("config: %s must not include credentials", name)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("config: %s must not include a query or fragment", name)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

// parseBindHost selects the listen address. The default keeps the app loopback-only;
// containers set BIND_HOST=0.0.0.0 and must publish only to the host loopback.
func parseBindHost(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultBindHost, nil
	}
	if net.ParseIP(raw) == nil {
		return "", fmt.Errorf("config: BIND_HOST must be an IP address")
	}
	return raw, nil
}

// parseAllowedHosts validates the comma-separated Host allowlist. Entries are
// hostnames or IP literals without a scheme, path or port.
func parseAllowedHosts(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]string(nil), defaultAllowedHosts...), nil
	}
	entries := strings.Split(raw, ",")
	hosts := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		host := strings.ToLower(strings.TrimSpace(entry))
		if host == "" {
			return nil, fmt.Errorf("config: ALLOWED_HOSTS contains an empty entry")
		}
		if err := validateHost(host); err != nil {
			return nil, fmt.Errorf("config: ALLOWED_HOSTS entry %q is invalid: %w", entry, err)
		}
		host = strings.Trim(host, "[]")
		if _, duplicate := seen[host]; duplicate {
			continue
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("config: ALLOWED_HOSTS must name at least one host")
	}
	return hosts, nil
}

func validateHost(host string) error {
	trimmed := strings.Trim(host, "[]")
	if net.ParseIP(trimmed) != nil {
		return nil
	}
	if strings.ContainsAny(host, "/:? \t") {
		return fmt.Errorf("must not include a scheme, path or port")
	}
	if len(host) > 253 {
		return fmt.Errorf("hostname is too long")
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") || strings.Contains(host, "..") {
		return fmt.Errorf("hostname has an empty label")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return fmt.Errorf("hostname has an invalid label")
		}
		for index, symbol := range label {
			switch {
			case symbol >= 'a' && symbol <= 'z':
			case symbol >= '0' && symbol <= '9':
			case symbol == '-' && index > 0 && index < len(label)-1:
			default:
				return fmt.Errorf("hostname has invalid characters")
			}
		}
	}
	return nil
}

func parsePositiveInt(name, raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("config: %s must be a positive integer", name)
	}
	return value, nil
}

func parsePositiveDuration(name, raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("config: %s must be a positive duration such as 2s or 1m", name)
	}
	return value, nil
}
