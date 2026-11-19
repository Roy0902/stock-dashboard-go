package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // pure-Go driver; CGO is disabled on the target host

	"stock-dashboard/server/internal/auth"
	"stock-dashboard/server/internal/domain"
)

const (
	// DBFileName is the single runtime data file. Legacy JSON files are read
	// once for migration and then renamed.
	DBFileName = "stock-dashboard.db"

	dbFileMode   os.FileMode = 0o600
	dbDriverName             = "sqlite"
	// schemaVersion lives in PRAGMA user_version. Version 2 gives every account
	// its own watchlist rows via watchlist.user_id.
	schemaVersion = 2
)

// userStatements create the account tables. They are idempotent so Open can
// repair a database whose version is already current.
var userStatements = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		totp_secret TEXT NOT NULL,
		totp_enabled INTEGER NOT NULL,
		last_totp_step INTEGER NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS recovery_codes (
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		code_hash TEXT NOT NULL,
		PRIMARY KEY (user_id, code_hash)
	)`,
}

// watchlistV2 is the per-account watchlist. Rows with a NULL user_id are legacy
// rows no account has claimed yet.
const watchlistV2Body = `(
	user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
	position INTEGER NOT NULL,
	ticker TEXT NOT NULL,
	PRIMARY KEY (user_id, position),
	UNIQUE (user_id, ticker)
)`

const watchlistV2 = "CREATE TABLE IF NOT EXISTS watchlist " + watchlistV2Body

// Store owns the SQLite database and the two persistence adapters built on it.
// The local tool keeps the pool at one connection, so writers queue in
// SQLite itself; the adapters add a mutex only to keep a read-then-write
// sequence inside one transaction.
type Store struct {
	db *sql.DB

	Watchlist *WatchlistStore
	Users     *Users
}

// Open creates $DATA_DIR/stock-dashboard.db when absent, applies the schema and
// imports legacy JSON files once. A corrupt legacy file fails startup, naming
// the file, because credentials must never be silently dropped.
func Open(dataDir string) (*Store, error) {
	return OpenWithLogger(dataDir, nil)
}

// OpenWithLogger is Open with a logger for bounded read-failure warnings.
func OpenWithLogger(dataDir string, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("store: create data directory: %w", err)
	}
	path := filepath.Join(dataDir, DBFileName)
	if err := ensurePrivateFile(path, logger); err != nil {
		return nil, err
	}

	db, err := sql.Open(dbDriverName, dataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// One connection serialises writers; requests are local and infrequent.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	storage := &Store{
		db:        db,
		Watchlist: &WatchlistStore{db: db, logger: logger},
		Users:     &Users{db: db, logger: logger},
	}

	closeOnError := func(cause error) (*Store, error) {
		_ = db.Close()
		return nil, cause
	}

	if err := verifyPragmas(db); err != nil {
		return closeOnError(fmt.Errorf("store: %s: %w", path, err))
	}
	if err := ensureSchema(db); err != nil {
		return closeOnError(fmt.Errorf("store: %s: %w", path, err))
	}
	if err := migrateLegacy(db, dataDir, logger); err != nil {
		return closeOnError(err)
	}
	watchlist, users, err := validateStored(db)
	if err != nil {
		return closeOnError(fmt.Errorf("store: %s is invalid: %w", path, err))
	}
	// Seed the last-known snapshots so a later query error cannot masquerade as
	// an empty watchlist or a missing account.
	storage.Watchlist.last = watchlist
	storage.Users.seed(users)
	return storage, nil
}

// Close releases the database handle. There is no WAL to checkpoint because the
// journal mode is DELETE; SQLite flushes the rollback journal on every commit.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	_, _ = s.db.Exec("PRAGMA optimize")
	return s.db.Close()
}

// ensurePrivateFile pre-creates the database with mode 0600 so the file is not
// briefly world-readable before SQLite touches it, and tightens an existing file
// on every open. Chmod is best effort: Windows ignores POSIX modes.
func ensurePrivateFile(path string, logger *slog.Logger) error {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return fmt.Errorf("store: %s exists but is not a regular file", path)
		}
		if chmodErr := os.Chmod(path, dbFileMode); chmodErr != nil {
			logger.Warn("store: could not tighten database permissions",
				"error_type", fmt.Sprintf("%T", chmodErr))
		}
		return nil
	case errors.Is(err, fs.ErrNotExist):
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, dbFileMode)
		if createErr != nil {
			return fmt.Errorf("store: create %s: %w", path, createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("store: close %s: %w", path, closeErr)
		}
		return nil
	default:
		return fmt.Errorf("store: stat %s: %w", path, err)
	}
}

// dataSourceName keeps the pragmas on every pooled connection. modernc/sqlite
// splits the query from a non-file DSN, so the raw path is not URI-escaped; a
// POSIX path containing '?' falls back to a properly escaped file: URI.
func dataSourceName(path string) string {
	params := url.Values{}
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "journal_mode(DELETE)")
	params.Add("_pragma", "synchronous(FULL)")

	if strings.Contains(path, "?") {
		uri := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
		return uri.String() + "?" + params.Encode()
	}
	return path + "?" + params.Encode()
}

// verifyPragmas checks that the DSN pragmas reached the connection. If one is
// missing the database would silently run without foreign keys or durability.
func verifyPragmas(db *sql.DB) error {
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "delete") {
		return fmt.Errorf("journal_mode is %q, want delete", journalMode)
	}

	checks := []struct {
		name  string
		query string
		want  int
	}{
		{name: "foreign_keys", query: "PRAGMA foreign_keys", want: 1},
		{name: "busy_timeout", query: "PRAGMA busy_timeout", want: 5000},
		{name: "synchronous", query: "PRAGMA synchronous", want: 2},
	}
	for _, check := range checks {
		var got int
		if err := db.QueryRow(check.query).Scan(&got); err != nil {
			return fmt.Errorf("read %s: %w", check.name, err)
		}
		if got != check.want {
			return fmt.Errorf("%s is %d, want %d", check.name, got, check.want)
		}
	}

	var integrity string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil {
		return fmt.Errorf("read quick_check: %w", err)
	}
	if !strings.EqualFold(integrity, "ok") {
		return fmt.Errorf("quick_check failed: %s", integrity)
	}
	return nil
}

func ensureSchema(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("schema version %d is newer than supported version %d", version, schemaVersion)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range userStatements {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	if err := ensureWatchlistSchema(tx); err != nil {
		return err
	}
	if version < schemaVersion {
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema: %w", err)
	}
	return nil
}

// ensureWatchlistSchema creates the version 2 table, or migrates the version 1
// table in place. Rows go to the single version 1 account when one exists;
// otherwise they keep a NULL user_id so the first new account can claim them.
func ensureWatchlistSchema(tx *sql.Tx) error {
	columns, exists, err := watchlistColumns(tx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := tx.Exec(watchlistV2); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		return nil
	}
	if columns["user_id"] {
		// Already version 2.
		return nil
	}
	if !columns["position"] || !columns["ticker"] {
		return errors.New("unrecognised watchlist schema")
	}

	ownerID, err := singleUserID(tx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec("CREATE TABLE watchlist_v2 " + watchlistV2Body); err != nil {
		return fmt.Errorf("create version 2 watchlist: %w", err)
	}
	if ownerID != "" {
		if _, err := tx.Exec(`INSERT INTO watchlist_v2 (user_id, position, ticker)
			SELECT ?, position, ticker FROM watchlist ORDER BY position`, ownerID); err != nil {
			return fmt.Errorf("assign watchlist rows: %w", err)
		}
	} else {
		if _, err := tx.Exec(`INSERT INTO watchlist_v2 (user_id, position, ticker)
			SELECT NULL, position, ticker FROM watchlist ORDER BY position`); err != nil {
			return fmt.Errorf("copy watchlist rows: %w", err)
		}
	}
	if _, err := tx.Exec("DROP TABLE watchlist"); err != nil {
		return fmt.Errorf("drop version 1 watchlist: %w", err)
	}
	if _, err := tx.Exec("ALTER TABLE watchlist_v2 RENAME TO watchlist"); err != nil {
		return fmt.Errorf("rename version 2 watchlist: %w", err)
	}
	return nil
}

// watchlistColumns reports the column names of the watchlist table and whether
// the table exists at all.
func watchlistColumns(q queryer) (map[string]bool, bool, error) {
	rows, err := q.Query("PRAGMA table_info(watchlist)")
	if err != nil {
		return nil, false, fmt.Errorf("read watchlist schema: %w", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	exists := false
	for rows.Next() {
		var (
			cid, notNull, primaryKey int
			name, columnType         string
			defaultValue             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, false, fmt.Errorf("read watchlist schema: %w", err)
		}
		exists = true
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("read watchlist schema: %w", err)
	}
	return columns, exists, nil
}

func singleUserID(q queryer) (string, error) {
	var count int
	if err := q.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return "", fmt.Errorf("count users: %w", err)
	}
	if count != 1 {
		return "", nil
	}
	var id string
	if err := q.QueryRow("SELECT id FROM users").Scan(&id); err != nil {
		return "", fmt.Errorf("read single account: %w", err)
	}
	return id, nil
}

// queryer is satisfied by both *sql.DB and *sql.Tx, letting loaders run inside or
// outside a transaction.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// validateStored re-checks rows written outside this process so a tampered or
// partially migrated database fails closed instead of surfacing as "no
// accounts". The loaded values seed the adapters' last-known snapshots.
func validateStored(db *sql.DB) (map[string]domain.Watchlist, map[string]*auth.User, error) {
	users, err := loadUsers(db)
	if err != nil {
		return nil, nil, fmt.Errorf("account records: %w", err)
	}
	watchlists := make(map[string]domain.Watchlist, len(users))
	for id := range users {
		watchlist, err := loadWatchlist(db, id)
		if err != nil {
			return nil, nil, fmt.Errorf("watchlist for %s: %w", id, err)
		}
		watchlists[id] = watchlist
	}
	return watchlists, users, nil
}
