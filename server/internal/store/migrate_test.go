package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"stock-dashboard/server/internal/auth"
)

func writeLegacy(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func legacyUsersPayload(t *testing.T, users ...auth.User) string {
	t.Helper()
	data, err := json.Marshal(struct {
		Version int         `json:"version"`
		Users   []auth.User `json:"users"`
	}{Version: usersFileVersion, Users: users})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func orphanTickers(t *testing.T, storage *Store) []string {
	t.Helper()
	rows, err := storage.db.Query("SELECT ticker FROM watchlist WHERE user_id IS NULL ORDER BY position")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	tickers := []string{}
	for rows.Next() {
		var ticker string
		if err := rows.Scan(&ticker); err != nil {
			t.Fatal(err)
		}
		tickers = append(tickers, ticker)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return tickers
}

// writeV1Database creates the exact version 1 schema so Open has to migrate it
// in place.
func writeV1Database(t *testing.T, dir string, users []auth.User, tickers []string) {
	t.Helper()
	path := filepath.Join(dir, DBFileName)
	db, err := sql.Open(dbDriverName, dataSourceName(path))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	statements := []string{
		`CREATE TABLE watchlist (
			position INTEGER PRIMARY KEY,
			ticker TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			totp_secret TEXT NOT NULL,
			totp_enabled INTEGER NOT NULL,
			last_totp_step INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE recovery_codes (
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			code_hash TEXT NOT NULL,
			PRIMARY KEY (user_id, code_hash)
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, user := range users {
		if _, err := db.Exec(`INSERT INTO users
			(id, username, password_hash, totp_secret, totp_enabled, last_totp_step, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			user.ID, user.Username, user.PasswordHash, user.TOTPSecret, boolToInt(user.TOTPEnabled),
			user.LastTOTPStep, encodeTimestamp(user.CreatedAt), encodeTimestamp(user.UpdatedAt),
		); err != nil {
			t.Fatal(err)
		}
		for _, hash := range user.RecoveryCodeHashes {
			if _, err := db.Exec("INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)", user.ID, hash); err != nil {
				t.Fatal(err)
			}
		}
	}
	for position, ticker := range tickers {
		if _, err := db.Exec("INSERT INTO watchlist (position, ticker) VALUES (?, ?)", position, ticker); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
}

func schemaVersionOf(t *testing.T, storage *Store) int {
	t.Helper()
	var version int
	if err := storage.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func TestMigrateWatchlistJSON(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir, watchlistFileName, `{"tickers":["aapl","AAPL","MSFT","goog"]}`)

	storage := mustOpen(t, dir)
	if got := orphanTickers(t, storage); !equalStrings(got, []string{"AAPL", "MSFT", "GOOG"}) {
		t.Fatalf("migrated orphan rows = %v", got)
	}
	mustClose(t, storage)

	if _, err := os.Stat(filepath.Join(dir, watchlistFileName)); !os.IsNotExist(err) {
		t.Fatalf("legacy watchlist still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, watchlistFileName+migratedSuffix)); err != nil {
		t.Fatalf("migrated marker missing: %v", err)
	}

	// Reopening must not import twice or resurrect the legacy file.
	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := orphanTickers(t, reopened); !equalStrings(got, []string{"AAPL", "MSFT", "GOOG"}) {
		t.Fatalf("reloaded orphan rows = %v", got)
	}
}

func TestMigrateUsersJSON(t *testing.T) {
	dir := t.TempDir()
	owner := testOwner(t)
	writeLegacy(t, dir, usersFileName, legacyUsersPayload(t, owner))
	// A legacy file from the previous implementation was typically 0644.
	if err := os.Chmod(filepath.Join(dir, usersFileName), 0o644); err != nil {
		t.Fatal(err)
	}

	storage := mustOpen(t, dir)
	loaded := storage.Users.FindByUsername("owner")
	if loaded == nil || loaded.ID != owner.ID || loaded.Username != owner.Username {
		t.Fatalf("migrated account = %+v", loaded)
	}
	if loaded.PasswordHash != owner.PasswordHash || loaded.TOTPSecret != owner.TOTPSecret {
		t.Fatal("credentials did not survive migration")
	}
	if loaded.LastTOTPStep != owner.LastTOTPStep || !loaded.CreatedAt.Equal(owner.CreatedAt) {
		t.Fatalf("account state = %+v", loaded)
	}
	if !equalStrings(fullProjection(t, storage, owner.ID).RecoveryCodeHashes, owner.RecoveryCodeHashes) {
		t.Fatalf("recovery hashes = %v", fullProjection(t, storage, owner.ID).RecoveryCodeHashes)
	}
	mustClose(t, storage)

	if _, err := os.Stat(filepath.Join(dir, usersFileName)); !os.IsNotExist(err) {
		t.Fatalf("legacy users file still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, usersFileName+migratedSuffix)); err != nil {
		t.Fatalf("migrated marker missing: %v", err)
	}
	requireMigratedPrivate(t, filepath.Join(dir, usersFileName+migratedSuffix))

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := reopened.Users.FindByUsername("owner"); got == nil || got.PasswordHash != owner.PasswordHash {
		t.Fatalf("reloaded account = %+v", got)
	}
}

func TestMigrateBothLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	owner := testOwner(t)
	writeLegacy(t, dir, watchlistFileName, `{"tickers":["AAPL"]}`)
	writeLegacy(t, dir, usersFileName, legacyUsersPayload(t, owner))

	storage := mustOpen(t, dir)
	if got := listStrings(storage.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("claimed list = %v", got)
	}
	if got := orphanTickers(t, storage); len(got) != 0 {
		t.Fatalf("orphan rows = %v", got)
	}
	if got := storage.Users.FindByUsername("owner"); got == nil || got.ID != owner.ID {
		t.Fatalf("account = %+v", got)
	}
	mustClose(t, storage)
}

func TestMigrateMultipleLegacyUsersLeavesRowsOrphaned(t *testing.T) {
	dir := t.TempDir()
	first := testOwner(t)
	second := testOwner(t)
	second.ID = "usr_ffffffffffffffff"
	second.Username = "other"
	writeLegacy(t, dir, watchlistFileName, `{"tickers":["AAPL"]}`)
	writeLegacy(t, dir, usersFileName, legacyUsersPayload(t, first, second))

	storage := mustOpen(t, dir)
	defer mustClose(t, storage)
	if got := storage.Users.Count(); got != 2 {
		t.Fatalf("count = %d", got)
	}
	if got := listStrings(storage.Watchlist.List(first.ID)); len(got) != 0 {
		t.Fatalf("first account claimed rows: %v", got)
	}
	if got := orphanTickers(t, storage); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("orphan rows = %v", got)
	}
}

func TestMigrationSkipsExistingData(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	addTickers(t, storage, owner.ID, "AAPL")
	mustClose(t, storage)

	// Different legacy contents must not replace the authoritative database.
	other := testOwner(t)
	other.ID = "usr_ffffffffffffffff"
	other.Username = "other"
	writeLegacy(t, dir, watchlistFileName, `{"tickers":["TSLA"]}`)
	writeLegacy(t, dir, usersFileName, legacyUsersPayload(t, other))

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("list = %v", got)
	}
	if got := reopened.Users.FindByUsername("owner"); got == nil || got.Username != "owner" {
		t.Fatalf("account = %+v", got)
	}

	for _, name := range []string{watchlistFileName, usersFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("legacy %s was touched: %v", name, err)
		}
	}
}

func TestMigrationSkipsCorruptLegacyWhenDatabaseHasData(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	addTickers(t, storage, owner.ID, "AAPL")
	mustClose(t, storage)

	writeLegacy(t, dir, watchlistFileName, `{`)
	writeLegacy(t, dir, usersFileName, `{`)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("list = %v", got)
	}
}

func TestMigrateCorruptWatchlistFailsClosed(t *testing.T) {
	tests := map[string]string{
		"invalid json":    `{"tickers":[`,
		"wrong shape":     `{"tickers":"AAPL"}`,
		"missing field":   `{}`,
		"null field":      `{"tickers":null}`,
		"invalid ticker":  `{"tickers":["AA-PL"]}`,
		"invalid payload": `not json`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeLegacy(t, dir, watchlistFileName, content)
			_, err := Open(dir)
			if err == nil {
				t.Fatal("corrupt legacy watchlist accepted")
			}
			if !strings.Contains(err.Error(), watchlistFileName) {
				t.Fatalf("error does not name the file: %v", err)
			}
		})
	}
}

func TestMigrateCorruptUsersFailsClosed(t *testing.T) {
	owner := testOwner(t)
	invalidUsername := owner
	invalidUsername.Username = "Owner"
	invalidHash := owner
	invalidHash.PasswordHash = "plaintext"
	invalidSecret := owner
	invalidSecret.TOTPSecret = "not base32!"
	disabled := owner
	disabled.TOTPEnabled = false
	badRecovery := owner
	badRecovery.RecoveryCodeHashes = []string{"abc"}
	noID := owner
	noID.ID = ""
	noTimestamps := owner
	noTimestamps.CreatedAt = time.Time{}

	tests := map[string]string{
		"not json":         `{`,
		"missing version":  marshalLegacyUsers(t, map[string]any{"users": []auth.User{owner}}),
		"wrong version":    marshalLegacyUsers(t, map[string]any{"version": 2, "users": []auth.User{owner}}),
		"missing users":    marshalLegacyUsers(t, map[string]any{"version": 1}),
		"duplicate id":     legacyUsersPayload(t, owner, owner),
		"invalid username": legacyUsersPayload(t, invalidUsername),
		"invalid hash":     legacyUsersPayload(t, invalidHash),
		"invalid secret":   legacyUsersPayload(t, invalidSecret),
		"totp disabled":    legacyUsersPayload(t, disabled),
		"bad recovery":     legacyUsersPayload(t, badRecovery),
		"no id":            legacyUsersPayload(t, noID),
		"no timestamps":    legacyUsersPayload(t, noTimestamps),
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeLegacy(t, dir, usersFileName, content)
			_, err := Open(dir)
			if err == nil {
				t.Fatalf("%s accepted", name)
			}
			if !strings.Contains(err.Error(), usersFileName) {
				t.Fatalf("error does not name the file: %v", err)
			}
		})
	}
}

func TestMigrateEmptyUsersList(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir, usersFileName, `{"version":1,"users":[]}`)

	storage := mustOpen(t, dir)
	if storage.Users.Count() != 0 {
		t.Fatal("empty legacy list created an account")
	}
	mustClose(t, storage)
	if _, err := os.Stat(filepath.Join(dir, usersFileName+migratedSuffix)); err != nil {
		t.Fatalf("migrated marker missing: %v", err)
	}
}

func TestMigrationRenameIsBestEffort(t *testing.T) {
	dir := t.TempDir()
	writeLegacy(t, dir, watchlistFileName, `{"tickers":["AAPL"]}`)
	// A directory at the target path makes the rename fail on every platform.
	if err := os.Mkdir(filepath.Join(dir, watchlistFileName+migratedSuffix), 0o755); err != nil {
		t.Fatal(err)
	}

	storage, err := Open(dir)
	if err != nil {
		t.Fatalf("rename failure broke startup: %v", err)
	}
	defer mustClose(t, storage)
	if got := orphanTickers(t, storage); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("orphan rows = %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, watchlistFileName)); err != nil {
		t.Fatalf("legacy file was removed without a rename: %v", err)
	}
}

func TestMigrateV1DatabaseWithOwnerAssignsWatchlist(t *testing.T) {
	dir := t.TempDir()
	owner := testOwner(t)
	writeV1Database(t, dir, []auth.User{owner}, []string{"AAPL", "MSFT"})

	storage := mustOpen(t, dir)
	defer mustClose(t, storage)
	if got := schemaVersionOf(t, storage); got != schemaVersion {
		t.Fatalf("user_version = %d, want %d", got, schemaVersion)
	}
	if got := listStrings(storage.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("owner list = %v", got)
	}
	if got := orphanTickers(t, storage); len(got) != 0 {
		t.Fatalf("orphan rows = %v", got)
	}
	if got := storage.Users.FindByUsername("owner"); got == nil || got.ID != owner.ID {
		t.Fatalf("account = %+v", got)
	}
}

func TestMigrateV1DatabaseWithoutOwnerLeavesOrphansForFirstSignup(t *testing.T) {
	dir := t.TempDir()
	writeV1Database(t, dir, nil, []string{"AAPL", "MSFT"})

	storage := mustOpen(t, dir)
	defer mustClose(t, storage)
	if got := schemaVersionOf(t, storage); got != schemaVersion {
		t.Fatalf("user_version = %d, want %d", got, schemaVersion)
	}
	if storage.Users.Count() != 0 {
		t.Fatal("v1 database without users gained an account")
	}
	if got := orphanTickers(t, storage); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("orphan rows = %v", got)
	}

	first := addAccount(t, storage, "usr_0123456789abcdef", "first")
	if got := listStrings(storage.Watchlist.List(first.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("first account list = %v", got)
	}
	second := addAccount(t, storage, "usr_ffffffffffffffff", "second")
	if got := storage.Watchlist.List(second.ID).Len(); got != 0 {
		t.Fatalf("second account inherited %d rows", got)
	}
}

func marshalLegacyUsers(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// requireMigratedPrivate asserts the renamed legacy file is owner-only on hosts
// where POSIX mode bits are meaningful. Windows reports synthetic bits.
func requireMigratedPrivate(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if perm := info.Mode().Perm(); perm != migratedFileMode {
		t.Fatalf("migrated file mode = %04o, want %04o", perm, migratedFileMode)
	}
}

func TestTightenMigratedFile(t *testing.T) {
	tests := []struct {
		name      string
		chmodErr  error
		wantMode  os.FileMode
		wantWarns bool
	}{
		{name: "success sets owner-only", wantMode: migratedFileMode},
		{name: "failure warns without failing", chmodErr: errors.New("permission denied"), wantWarns: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			var gotMode os.FileMode
			chmod := func(_ string, mode os.FileMode) error {
				gotMode = mode
				return test.chmodErr
			}
			tightenMigrated("users.json.migrated", logger, chmod)
			if test.wantMode != 0 && gotMode != test.wantMode {
				t.Fatalf("chmod mode = %04o, want %04o", gotMode, test.wantMode)
			}
			if warned := strings.Contains(logs.String(), "could not tighten migrated file permissions"); warned != test.wantWarns {
				t.Fatalf("warning logged = %v, want %v (logs %q)", warned, test.wantWarns, logs.String())
			}
		})
	}
}
