package store

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// After the grace window an unreadable account table must stop authenticating
// from the snapshot, and the failure must be visible as a bounded warning.
func TestAuthReadsFailClosedAfterGrace(t *testing.T) {
	dir := t.TempDir()
	logs := &bytes.Buffer{}
	storage, err := OpenWithLogger(dir, slog.New(slog.NewJSONHandler(logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = storage.Close() }()

	owner := testOwner(t)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}
	if err := storage.db.Close(); err != nil {
		t.Fatal(err)
	}

	storage.Users.mu.Lock()
	storage.Users.lastSuccess = time.Now().Add(-2 * authReadGrace)
	storage.Users.mu.Unlock()

	if got := storage.Users.FindByID(owner.ID); got != nil {
		t.Fatalf("FindByID served a stale account after grace: %+v", got)
	}
	if got := storage.Users.FindByUsername(owner.Username); got != nil {
		t.Fatalf("FindByUsername served a stale account after grace: %+v", got)
	}
	if !strings.Contains(logs.String(), `"op":"FindByID"`) {
		t.Fatalf("missing bounded read warning: %s", logs.String())
	}
}
