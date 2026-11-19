package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stock-dashboard/server/internal/domain"
)

// MaxBodyBytes bounds how much of an upstream response is read. 1 MiB is far more
// than any quote or 365-point series needs and stops a runaway body.
const MaxBodyBytes int64 = 1 << 20

// Response is a decoded upstream response. Status is preserved even when Body could
// not be decoded (Body is then nil), so adapters can map a 404 correctly.
type Response struct {
	Status     int
	Body       any
	RetryAfter time.Duration
}

// HTTPClient performs the shared upstream GET. Transport failures become a
// Failure without any URL text, because URLs carry API keys.
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient builds a client with a hard overall timeout.
func NewHTTPClient(timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = DefaultLimits.UpstreamTimeout
	}
	return &HTTPClient{client: &http.Client{Timeout: timeout}}
}

// GetJSON fetches target and decodes a single JSON value with numbers preserved as
// json.Number. A decode failure is not an error: Body is nil and Status is returned.
// One bounded, jittered retry covers transient transport failures for this
// idempotent GET; timeouts and cancellation are not retried.
func (c *HTTPClient) GetJSON(ctx context.Context, provider domain.ProviderAttribution, target string) (Response, error) {
	response, err := c.getOnce(ctx, provider, target)
	if err == nil {
		return response, nil
	}
	var transport *transportError
	if !errors.As(err, &transport) || !transport.retry {
		return Response{}, err
	}
	if ctx.Err() != nil {
		return Response{}, failure(provider, FailureCanceled)
	}
	if err := waitJitter(ctx); err != nil {
		return Response{}, failure(provider, FailureCanceled)
	}
	return c.getOnce(ctx, provider, target)
}

// transportError marks whether a transport failure may be retried. It unwraps to
// the public *Failure so callers only ever see provider-safe kinds.
type transportError struct {
	failure *Failure
	retry   bool
}

func (e *transportError) Error() string { return e.failure.Error() }

func (e *transportError) Unwrap() error { return e.failure }

func (c *HTTPClient) getOnce(ctx context.Context, provider domain.ProviderAttribution, target string) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, failure(provider, FailureCanceled)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Response{}, failure(provider, FailureUnavailable)
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, failure(provider, FailureCanceled)
		}
		return Response{}, &transportError{
			failure: failure(provider, FailureUnavailable),
			retry:   !isTimeout(err),
		}
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, MaxBodyBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, failure(provider, FailureCanceled)
		}
		return Response{}, &transportError{failure: failure(provider, FailureUnavailable)}
	}
	if int64(len(data)) > MaxBodyBytes {
		return Response{}, &transportError{failure: failure(provider, FailureUnavailable)}
	}

	result := Response{
		Status:     response.StatusCode,
		RetryAfter: parseRetryAfter(response.Header.Get("Retry-After")),
	}
	body, ok := decodeSingleValue(data)
	if ok {
		result.Body = body
	}
	return result, nil
}

// CloseIdleConnections releases transport state at shutdown.
func (c *HTTPClient) CloseIdleConnections() {
	if c != nil && c.client != nil {
		c.client.CloseIdleConnections()
	}
}

func waitJitter(ctx context.Context) error {
	delay := 25*time.Millisecond + time.Duration(rand.IntN(50))*time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	var candidate timeout
	return errors.As(err, &candidate) && candidate.Timeout()
}

// parseRetryAfter accepts either delta-seconds or an HTTP date.
func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return time.Second
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(raw); err == nil {
		wait := time.Until(at)
		if wait <= 0 {
			return time.Second
		}
		return wait
	}
	return 0
}

func decodeSingleValue(data []byte) (any, bool) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return value, true
}

func failure(provider domain.ProviderAttribution, kind FailureKind) *Failure {
	return &Failure{Kind: kind, Provider: provider}
}

// authOrLimitFailure maps upstream throttling and credential rejection without
// letting provider text escape. Every other status is left to the adapter.
func authOrLimitFailure(provider domain.ProviderAttribution, response Response) *Failure {
	switch response.Status {
	case http.StatusTooManyRequests:
		retryAfter := response.RetryAfter
		if retryAfter <= 0 {
			retryAfter = time.Second
		}
		return &Failure{Kind: FailureRateLimited, Provider: provider, RetryAfter: retryAfter}
	case http.StatusUnauthorized, http.StatusForbidden:
		return failure(provider, FailureAuth)
	default:
		return nil
	}
}

func asObject(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok
}

// jsonNumber accepts only JSON numbers. Finnhub fields are numeric in its contract.
func jsonNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case json.Number:
		f, err := number.Float64()
		return f, err == nil && finite(f)
	case float64:
		return number, finite(number)
	default:
		return 0, false
	}
}

// flexNumber also accepts numeric strings, which Twelve Data emits inconsistently.
func flexNumber(value any) (float64, bool) {
	if number, ok := jsonNumber(value); ok {
		return number, true
	}
	text, ok := value.(string)
	if !ok {
		return 0, false
	}
	return parseFloat(text)
}

func parseFloat(text string) (float64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || !finite(f) {
		return 0, false
	}
	return f, true
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
