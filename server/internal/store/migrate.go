package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"stock-dashboard/server/internal/auth"
	"stock-dashboard/server/internal/domain"
)

const (
	watchlistFileName = "watchlist.json"
	usersFileName     = "users.json"
	usersFileVersion  = 1
	// migratedSuffix marks a legacy file whose contents are now in SQLite.
	migratedSuffix = ".migrated"
	// migratedFileMode is owner-only: the copy may still hold credential material.
	migratedFileMode os.FileMode = 0o600
)

// migrateLegacy imports the JSON files from the previous store exactly once. An
// empty table plus a present file means "not migrated yet"; a non-empty table
// means the database is authoritative and the legacy file is left untouched.
// Watchlist rows arrive without an owner; a single migrated account claims them.
func migrateLegacy(db *sql.DB, dataDir string, logger *slog.Logger) error {
	if err := migrateWatchlistFile(db, dataDir, logger); err != nil {
		return err
	}
	if err := migrateUsersFile(db, dataDir, logger); err != nil {
		return err
	}
	return claimOrphanWatchlist(db)
}

func migrateWatchlistFile(db *sql.DB, dataDir string, logger *slog.Logger) error {
	path := filepath.Join(dataDir, watchlistFileName)
	exists, err := legacyExists(path)
	if err != nil || !exists {
		return err
	}
	empty, err := isTableEmpty(db, "SELECT COUNT(*) FROM watchlist")
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("store: read %s: %w", path, err)
	}
	var payload struct {
		Tickers *[]string `json:"tickers"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("store: %s is not valid JSON: %w", path, err)
	}
	if payload.Tickers == nil {
		return fmt.Errorf("store: %s has an unexpected shape", path)
	}

	watchlist := domain.Watchlist{}
	for _, raw := range *payload.Tickers {
		ticker, err := domain.Canonicalize(raw)
		if err != nil {
			return fmt.Errorf("store: %s contains an invalid ticker: %w", path, err)
		}
		if watchlist.Contains(ticker) {
			continue
		}
		watchlist, err = watchlist.Add(ticker.String())
		if err != nil {
			return fmt.Errorf("store: %s contains an invalid ticker: %w", path, err)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin watchlist migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	empty, err = isTableEmpty(tx, "SELECT COUNT(*) FROM watchlist")
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	if err := insertOrphanWatchlist(tx, watchlist); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit watchlist migration: %w", err)
	}
	renameLegacy(path, logger)
	return nil
}

func migrateUsersFile(db *sql.DB, dataDir string, logger *slog.Logger) error {
	path := filepath.Join(dataDir, usersFileName)
	exists, err := legacyExists(path)
	if err != nil || !exists {
		return err
	}
	empty, err := isTableEmpty(db, "SELECT COUNT(*) FROM users")
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("store: read %s: %w", path, err)
	}
	var payload struct {
		Version *int         `json:"version"`
		Users   *[]auth.User `json:"users"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("store: %s is not valid JSON: %w", path, err)
	}
	if payload.Version == nil || *payload.Version != usersFileVersion {
		return fmt.Errorf("store: %s has an unsupported version", path)
	}
	if payload.Users == nil {
		return fmt.Errorf("store: %s has an unexpected shape", path)
	}
	users := *payload.Users
	validated := make([]auth.User, 0, len(users))
	for _, candidate := range users {
		if err := validateUser(&candidate); err != nil {
			return fmt.Errorf("store: %s contains an invalid account: %w", path, err)
		}
		validated = append(validated, candidate)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin account migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	empty, err = isTableEmpty(tx, "SELECT COUNT(*) FROM users")
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	for index := range validated {
		if err := insertUser(tx, &validated[index]); err != nil {
			return fmt.Errorf("store: %s contains an invalid account: %w", path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit account migration: %w", err)
	}
	renameLegacy(path, logger)
	return nil
}

// claimOrphanWatchlist gives legacy rows without an owner to the only account
// that exists. A newer database with several accounts leaves them untouched.
func claimOrphanWatchlist(db *sql.DB) error {
	ownerID, err := singleUserID(db)
	if err != nil {
		return err
	}
	if ownerID == "" {
		return nil
	}
	if _, err := db.Exec(
		"UPDATE OR IGNORE watchlist SET user_id = ? WHERE user_id IS NULL",
		ownerID,
	); err != nil {
		return fmt.Errorf("store: claim legacy watchlist: %w", err)
	}
	return nil
}

func legacyExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("store: stat %s: %w", path, err)
}

func isTableEmpty(q queryer, query string) (bool, error) {
	var count int
	if err := q.QueryRow(query).Scan(&count); err != nil {
		return false, fmt.Errorf("store: count rows: %w", err)
	}
	return count == 0, nil
}

// renameLegacy is best effort: keeping a readable copy is harmless, so a failed
// rename must not stop startup after the transaction committed. A successful
// rename tightens the copy, which still holds password hashes, TOTP seeds and
// recovery hashes. It is never deleted silently.
func renameLegacy(path string, logger *slog.Logger) {
	target := path + migratedSuffix
	if err := os.Rename(path, target); err != nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	tightenMigrated(target, logger, os.Chmod)
}

// tightenMigrated makes a migrated legacy file owner-only. A failure is a warning,
// not a startup failure: the data is already in SQLite and the operator may still
// want to inspect the copy.
func tightenMigrated(path string, logger *slog.Logger, chmod func(string, os.FileMode) error) {
	if err := chmod(path, migratedFileMode); err != nil {
		logger.Warn("store: could not tighten migrated file permissions",
			"error_type", fmt.Sprintf("%T", err))
	}
}
