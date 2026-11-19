package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                {Data: []byte("<!doctype html><title>app shell</title>")},
		"assets/index-abc12345.js":  {Data: []byte("console.log('app')")},
		"assets/index-abc12345.css": {Data: []byte("body{color:red}")},
		"favicon.svg":               {Data: []byte("<svg></svg>")},
	}
}

func request(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

func TestServesIndexAtRoot(t *testing.T) {
	handler := NewFS(testFS())
	response := request(t, handler, http.MethodGet, "/")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "app shell") {
		t.Fatalf("body = %q", response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q", got)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content-type = %q", got)
	}
}

func TestDeepLinkFallsBackToIndex(t *testing.T) {
	handler := NewFS(testFS())
	for _, target := range []string{"/chart/AAPL", "/chart/aapl?range=3M", "/some/deep/path"} {
		response := request(t, handler, http.MethodGet, target)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", target, response.Code)
		}
		if !strings.Contains(response.Body.String(), "app shell") {
			t.Fatalf("%s did not serve the shell", target)
		}
	}
}

func TestServesHashedAssetsWithLongCache(t *testing.T) {
	handler := NewFS(testFS())
	response := request(t, handler, http.MethodGet, "/assets/index-abc12345.js")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("cache-control = %q", got)
	}
	if response.Body.String() != "console.log('app')" {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestServesRootAssetWithMIME(t *testing.T) {
	handler := NewFS(testFS())
	response := request(t, handler, http.MethodGet, "/favicon.svg")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestMissingAssetIs404(t *testing.T) {
	handler := NewFS(testFS())
	response := request(t, handler, http.MethodGet, "/assets/missing.js")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestHeadHasNoBody(t *testing.T) {
	handler := NewFS(testFS())
	response := request(t, handler, http.MethodHead, "/assets/index-abc12345.js")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q", response.Body.String())
	}
	if response.Header().Get("Content-Length") == "" {
		t.Fatal("HEAD did not set Content-Length")
	}
}

func TestNonGetIsMethodNotAllowed(t *testing.T) {
	handler := NewFS(testFS())
	response := request(t, handler, http.MethodPost, "/chart/AAPL")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("allow = %q", response.Header().Get("Allow"))
	}
}

func TestMissingIndexStillServesAPlaceholder(t *testing.T) {
	handler := NewFS(fstest.MapFS{})
	response := request(t, handler, http.MethodGet, "/chart/AAPL")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "client build is not available") {
		t.Fatalf("body = %q", response.Body.String())
	}
}

func TestUnknownFileWithExtensionIs404(t *testing.T) {
	handler := NewFS(testFS())
	response := request(t, handler, http.MethodGet, "/../../etc/passwd.txt")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}
