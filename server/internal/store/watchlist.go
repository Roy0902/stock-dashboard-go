package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"stock-dashboard/server/internal/domain"
)

// WatchlistStore persists each account's insertion-ordered list in SQLite. Reads
// come from the maintained snapshot (seeded at Open and rebuilt on every write),
// so the polling path never touches the single SQLite connection.
type WatchlistStore struct {
	mu      sync.Mutex
	db      *sql.DB
	logger  *slog.Logger
	last    map[string]domain.Watchlist
	lastLog time.Time
}

// List returns one account's maintained snapshot in insertion order. The first
// read for an account loads it (and an empty list is remembered); after that the
// polling path never touches the single SQLite connection.
func (s *WatchlistStore) List(userID string) domain.Watchlist {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, known := s.last[userID]
	if known {
		return snapshot
	}
	watchlist, err := loadWatchlist(s.db, userID)
	if err != nil {
		s.logReadFailure("List", err)
		return snapshot
	}
	s.remember(userID, watchlist)
	return watchlist
}

// logReadFailure emits at most one warning per minute and never logs SQL text,
// query arguments or account identifiers.
func (s *WatchlistStore) logReadFailure(operation string, err error) {
	now := time.Now()
	if now.Sub(s.lastLog) < readLogInterval {
		return
	}
	s.lastLog = now
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("store read failed",
		"op", operation,
		"error_type", fmt.Sprintf("%T", err),
	)
}

// Update applies fn inside a transaction scoped to one account. Fn errors roll
// back. A successful result replaces that account's rows in one commit, which
// preserves order and keeps readers from observing a partial list.
func (s *WatchlistStore) Update(userID string, fn func(domain.Watchlist) (domain.Watchlist, error)) (domain.Watchlist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return domain.Watchlist{}, fmt.Errorf("store: begin watchlist transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := loadWatchlist(tx, userID)
	if err != nil {
		return domain.Watchlist{}, err
	}
	next, err := fn(current)
	if err != nil {
		return domain.Watchlist{}, err
	}
	if err := replaceWatchlist(tx, userID, next); err != nil {
		return domain.Watchlist{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Watchlist{}, fmt.Errorf("store: commit watchlist: %w", err)
	}
	s.remember(userID, next)
	return next, nil
}

func (s *WatchlistStore) remember(userID string, watchlist domain.Watchlist) {
	if s.last == nil {
		s.last = make(map[string]domain.Watchlist)
	}
	s.last[userID] = watchlist
}

func loadWatchlist(q queryer, userID string) (domain.Watchlist, error) {
	rows, err := q.Query("SELECT ticker FROM watchlist WHERE user_id = ? ORDER BY position", userID)
	if err != nil {
		return domain.Watchlist{}, fmt.Errorf("store: read watchlist: %w", err)
	}
	defer rows.Close()

	watchlist := domain.Watchlist{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return domain.Watchlist{}, fmt.Errorf("store: scan watchlist: %w", err)
		}
		if watchlist.Contains(domain.Ticker(raw)) {
			continue
		}
		watchlist, err = watchlist.Add(raw)
		if err != nil {
			return domain.Watchlist{}, fmt.Errorf("store: watchlist contains an invalid ticker: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return domain.Watchlist{}, fmt.Errorf("store: read watchlist: %w", err)
	}
	return watchlist, nil
}

// replaceWatchlist deletes one account's rows and reinserts the list. Callers
// hold either the store mutex or an open transaction.
func replaceWatchlist(tx *sql.Tx, userID string, watchlist domain.Watchlist) error {
	if _, err := tx.Exec("DELETE FROM watchlist WHERE user_id = ?", userID); err != nil {
		return fmt.Errorf("store: clear watchlist: %w", err)
	}
	for position, ticker := range watchlist.Tickers() {
		if _, err := tx.Exec(
			"INSERT INTO watchlist (user_id, position, ticker) VALUES (?, ?, ?)",
			userID, position, ticker.String(),
		); err != nil {
			return fmt.Errorf("store: insert watchlist entry: %w", err)
		}
	}
	return nil
}

// insertOrphanWatchlist imports legacy rows with a NULL user_id so the first
// account created later can claim them.
func insertOrphanWatchlist(tx *sql.Tx, watchlist domain.Watchlist) error {
	for position, ticker := range watchlist.Tickers() {
		if _, err := tx.Exec(
			"INSERT INTO watchlist (user_id, position, ticker) VALUES (NULL, ?, ?)",
			position, ticker.String(),
		); err != nil {
			return fmt.Errorf("store: insert migrated watchlist entry: %w", err)
		}
	}
	return nil
}
