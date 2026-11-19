package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stock-dashboard/server/internal/domain"
)

func TestHTTPClientDecodesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"c":190.12,"ok":true}`))
	}))
	defer server.Close()

	client := NewHTTPClient(2 * time.Second)
	defer client.CloseIdleConnections()
	response, err := client.GetJSON(context.Background(), domain.ProviderFinnhub, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != http.StatusOK {
		t.Fatalf("status = %d", response.Status)
	}
	body, ok := response.Body.(map[string]any)
	if !ok {
		t.Fatalf("body type = %T", response.Body)
	}
	if _, ok := body["c"].(json.Number); !ok {
		t.Fatalf("number lost its json.Number identity: %T", body["c"])
	}
}

func TestHTTPClientMalformedBodyKeepsStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewHTTPClient(2 * time.Second)
	response, err := client.GetJSON(context.Background(), domain.ProviderTwelveData, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != http.StatusNotFound {
		t.Fatalf("status = %d", response.Status)
	}
	if response.Body != nil {
		t.Fatalf("body = %#v, want nil", response.Body)
	}
}

func TestHTTPClientRejectsOversizedBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", int(MaxBodyBytes)+1)))
	}))
	defer server.Close()

	client := NewHTTPClient(5 * time.Second)
	_, err := client.GetJSON(context.Background(), domain.ProviderFinnhub, server.URL)
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureUnavailable {
		t.Fatalf("error = %v, want unavailable failure", err)
	}
}

func TestHTTPClientTimeoutIsTwoSeconds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"c":1}`))
	}))
	defer server.Close()

	client := NewHTTPClient(2 * time.Second)
	defer client.CloseIdleConnections()
	started := time.Now()
	_, err := client.GetJSON(context.Background(), domain.ProviderFinnhub, server.URL)
	elapsed := time.Since(started)

	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureUnavailable {
		t.Fatalf("error = %v, want unavailable failure", err)
	}
	if elapsed < 1900*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("timeout elapsed = %s, want about 2s", elapsed)
	}
}

func TestHTTPClientDoesNotLeakURL(t *testing.T) {
	client := NewHTTPClient(50 * time.Millisecond)
	_, err := client.GetJSON(context.Background(), domain.ProviderFinnhub, "http://127.0.0.1:1/quote?token=SECRET-XYZ")
	if err == nil {
		t.Fatal("expected a transport failure")
	}
	if strings.Contains(err.Error(), "SECRET-XYZ") || strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("error leaked the target: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHTTPClientRetriesTransportFailureOnce(t *testing.T) {
	var calls int
	client := NewHTTPClient(time.Second)
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("connection reset")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
		}, nil
	})
	defer client.CloseIdleConnections()

	response, err := client.GetJSON(context.Background(), domain.ProviderFinnhub, "http://example.test/quote")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || response.Status != http.StatusOK {
		t.Fatalf("calls = %d, status = %d", calls, response.Status)
	}
}

func TestHTTPClientCancellationIsDistinct(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewHTTPClient(time.Second)
	_, err := client.GetJSON(ctx, domain.ProviderFinnhub, "http://example.test/quote")
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureCanceled {
		t.Fatalf("error = %v, want canceled failure", err)
	}
}
