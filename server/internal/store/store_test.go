package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"stock-dashboard/server/internal/auth"
	"stock-dashboard/server/internal/domain"
)

func mustOpen(t *testing.T, dir string) *Store {
	t.Helper()
	storage, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return storage
}

func mustClose(t *testing.T, storage *Store) {
	t.Helper()
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}
}

func listStrings(watchlist domain.Watchlist) []string {
	tickers := watchlist.Tickers()
	out := make([]string, len(tickers))
	for index, ticker := range tickers {
		out[index] = ticker.String()
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func addAccount(t *testing.T, storage *Store, id, username string) auth.User {
	t.Helper()
	account := testOwner(t)
	account.ID = id
	account.Username = username
	if err := storage.Users.Create(account); err != nil {
		t.Fatal(err)
	}
	return account
}

func addTickers(t *testing.T, storage *Store, userID string, tickers ...string) domain.Watchlist {
	t.Helper()
	next, err := storage.Watchlist.Update(userID, func(current domain.Watchlist) (domain.Watchlist, error) {
		var addErr error
		for _, ticker := range tickers {
			current, addErr = current.Add(ticker)
			if addErr != nil {
				return domain.Watchlist{}, addErr
			}
		}
		return current, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestOpenCreatesPrivateDatabase(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	defer mustClose(t, storage)

	path := filepath.Join(dir, DBFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Fatal("database path is a directory")
	}
	if runtime.GOOS != "windows" {
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("mode = %04o, want 0600", mode)
		}
	}
	if storage.Watchlist.List("").Len() != 0 {
		t.Fatal("new database has watchlist entries")
	}
	if storage.Users.Count() != 0 || storage.Users.FindByUsername("owner") != nil {
		t.Fatal("new database has an account")
	}
}

func TestSchemaVersionIsSetAndStable(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	var version int
	if err := storage.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version = %d, want %d", version, schemaVersion)
	}
	mustClose(t, storage)

	// Reopening must not re-run a destructive migration or change the version.
	storage = mustOpen(t, dir)
	defer mustClose(t, storage)
	if err := storage.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Fatalf("user_version after reopen = %d", version)
	}
}

func TestPragmasAreApplied(t *testing.T) {
	storage := mustOpen(t, t.TempDir())
	defer mustClose(t, storage)

	tests := []struct {
		query string
		want  int
	}{
		{query: "PRAGMA foreign_keys", want: 1},
		{query: "PRAGMA busy_timeout", want: 5000},
		{query: "PRAGMA synchronous", want: 2},
	}
	for _, test := range tests {
		var got int
		if err := storage.db.QueryRow(test.query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", test.query, err)
		}
		if got != test.want {
			t.Fatalf("%s = %d, want %d", test.query, got, test.want)
		}
	}
	var journalMode string
	if err := storage.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if journalMode != "delete" {
		t.Fatalf("journal_mode = %q, want delete", journalMode)
	}
}

// TestPragmasApplyToEveryConnection forces a second pooled connection. Pragmas
// set with PRAGMA statements after Open would be missing here; the DSN keeps
// them for every new connection.
func TestPragmasApplyToEveryConnection(t *testing.T) {
	storage := mustOpen(t, t.TempDir())
	defer mustClose(t, storage)
	storage.db.SetMaxOpenConns(2)
	storage.db.SetMaxIdleConns(0)

	ctx := context.Background()
	first, err := storage.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()
	second, err := storage.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()

	for index, connection := range []*sql.Conn{first, second} {
		var foreignKeys, busyTimeout, synchronous int
		if err := connection.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if err := connection.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatal(err)
		}
		if err := connection.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
			t.Fatal(err)
		}
		if foreignKeys != 1 || busyTimeout != 5000 || synchronous != 2 {
			t.Fatalf("connection %d pragmas = foreign_keys:%d busy_timeout:%d synchronous:%d",
				index, foreignKeys, busyTimeout, synchronous)
		}
	}
}

func TestOpenTightensExistingDatabasePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are advisory on Windows")
	}
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	mustClose(t, storage)

	path := filepath.Join(dir, DBFileName)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	storage = mustOpen(t, dir)
	defer mustClose(t, storage)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode = %04o, want 0600", mode)
	}
}

// TestOpenSeedsLastKnownState proves the snapshot is primed from the database at
// Open, so a query failure after startup cannot look like an empty watchlist or a
// missing owner.
func TestOpenHandlesPathWithSpacesAndHash(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data dir #1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	addTickers(t, storage, owner.ID, "AAPL")
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("list = %v", got)
	}
}

func TestOpenSeedsLastKnownState(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	addTickers(t, storage, owner.ID, "AAPL")
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	if err := reopened.db.Close(); err != nil {
		t.Fatal(err)
	}

	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("list on query failure = %v", got)
	}
	loaded := reopened.Users.FindByID(owner.ID)
	if loaded == nil || loaded.ID != owner.ID || loaded.Username != owner.Username {
		t.Fatalf("account on query failure = %+v", loaded)
	}
	if got := reopened.Users.Count(); got != 1 {
		t.Fatalf("count on query failure = %d", got)
	}
}

func TestDataDirectoryContainsOnlyDatabase(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	addTickers(t, storage, owner.ID, "AAPL")
	mustClose(t, storage)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != DBFileName {
		t.Fatalf("data directory entries = %v", entryNames(entries))
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	if _, err := storage.db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	mustClose(t, storage)

	if _, err := Open(dir); err == nil {
		t.Fatal("newer schema version accepted")
	}
}

func TestOpenRejectsInvalidAccountRow(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	if _, err := storage.db.Exec(`INSERT INTO users
		(id, username, password_hash, totp_secret, totp_enabled, last_totp_step, created_at, updated_at)
		VALUES ('usr_x', 'owner', 'not-a-hash', 'JBSWY3DPEHPK3PXP', 1, 0, '2026-11-18T03:00:00Z', '2026-11-18T03:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	mustClose(t, storage)

	_, err := Open(dir)
	if err == nil {
		t.Fatal("invalid account row accepted")
	}
	if !strings.Contains(err.Error(), "invalid account") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenLoadsMultipleAccountsAndSeparateWatchlists(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	second := addAccount(t, storage, "usr_ffffffffffffffff", "other")
	addTickers(t, storage, owner.ID, "AAPL")
	addTickers(t, storage, second.ID, "MSFT")
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := reopened.Users.Count(); got != 2 {
		t.Fatalf("count = %d", got)
	}
	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("owner list = %v", got)
	}
	if got := listStrings(reopened.Watchlist.List(second.ID)); !equalStrings(got, []string{"MSFT"}) {
		t.Fatalf("second list = %v", got)
	}
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name()
	}
	return names
}
