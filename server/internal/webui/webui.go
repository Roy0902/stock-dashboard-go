package webui

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var dist embed.FS

const (
	indexFile   = "index.html"
	assetDir    = "assets/"
	notFound    = "Not found."
	placeholder = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Stock Dashboard</title></head>
<body><main><h1>Stock Dashboard</h1>
<p>The client build is not available yet. Build the web client to populate the embedded assets.</p>
</main></body></html>
`
)

var contentTypes = map[string]string{
	".css":         "text/css; charset=utf-8",
	".htm":         "text/html; charset=utf-8",
	".html":        "text/html; charset=utf-8",
	".ico":         "image/x-icon",
	".jpeg":        "image/jpeg",
	".jpg":         "image/jpeg",
	".js":          "text/javascript; charset=utf-8",
	".json":        "application/json; charset=utf-8",
	".map":         "application/json; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".png":         "image/png",
	".svg":         "image/svg+xml",
	".ttf":         "font/ttf",
	".txt":         "text/plain; charset=utf-8",
	".webmanifest": "application/manifest+json",
	".webp":        "image/webp",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
}

// New serves the embedded client build.
func New() (http.Handler, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}
	return NewFS(sub), nil
}

// NewFS serves any filesystem with the same SPA rules; tests use fstest.MapFS.
func NewFS(fsys fs.FS) http.Handler {
	return &handler{fs: fsys}
}

type handler struct {
	fs fs.FS
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, notFound, http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || name == "." {
		h.serveIndex(w, r)
		return
	}

	if file, err := h.fs.Open(name); err == nil {
		defer file.Close()
		if info, err := file.Stat(); err == nil && !info.IsDir() {
			h.serveFile(w, r, name, file, info)
			return
		}
	}

	// Missing hashed assets are real errors; extensionless paths are SPA routes.
	if extension := path.Ext(name); extension != "" && extension != ".html" {
		http.Error(w, notFound, http.StatusNotFound)
		return
	}
	h.serveIndex(w, r)
}

// serveFile uses http.ServeContent so Range, HEAD, Last-Modified and
// If-None-Match are handled by the standard library. The ETag is derived from
// the immutable build name and size, not by rereading the file.
func (h *handler) serveFile(w http.ResponseWriter, r *http.Request, name string, file fs.File, info fs.FileInfo) {
	header := w.Header()
	header.Set("Content-Type", contentTypeFor(name))
	header.Set("ETag", fmt.Sprintf("%q", fmt.Sprintf("%s-%d", name, info.Size())))
	if strings.HasPrefix(name, assetDir) {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		header.Set("Cache-Control", "public, max-age=3600")
	}

	if seeker, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, name, info.ModTime(), seeker)
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, notFound, http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(data))
}

func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(h.fs, indexFile)
	if err != nil {
		data = []byte(placeholder)
	}

	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("ETag", fmt.Sprintf("%q", fmt.Sprintf("index-%x", sha256.Sum256(data))))
	http.ServeContent(w, r, indexFile, timeZero, bytes.NewReader(data))
}

// timeZero disables Last-Modified for the generated index while still enabling
// conditional requests through the ETag above.
var timeZero time.Time

func contentTypeFor(name string) string {
	extension := strings.ToLower(path.Ext(name))
	if contentType, ok := contentTypes[extension]; ok {
		return contentType
	}
	if contentType := mime.TypeByExtension(extension); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}
