package store

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"stock-dashboard/server/internal/auth"
)

// authReadGrace is how long a failed account read may keep serving the last
// successful snapshot. After that it fails closed (nil account / zero count).
const (
	authReadGrace   = 30 * time.Second
	readLogInterval = time.Minute
)

// Users persists accounts and their recovery-code hashes. It keeps the
// auth.UserStore port: lookups return copies or nil, Create enforces unique
// usernames, Update applies fn atomically to one account, and Count reports how
// many accounts exist. A read error is logged (bounded) and falls back to the
// last snapshot only inside authReadGrace.
type Users struct {
	mu          sync.Mutex
	db          *sql.DB
	logger      *slog.Logger
	now         func() time.Time
	lastByID    map[string]*auth.User
	lastCount   int
	lastSuccess time.Time
	lastLog     time.Time
}

// seed primes the snapshots from the values loaded at Open.
func (s *Users) seed(users map[string]*auth.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastByID = cloneUsers(users)
	s.lastCount = len(users)
	s.lastSuccess = s.nowUTC()
}

func (s *Users) nowUTC() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// FindByID returns a copy of the account, or nil when it does not exist. It
// deliberately omits recovery hashes: session/TOTP paths never need them, and
// the hot path must not pay for that second query.
func (s *Users) FindByID(id string) *auth.User {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, err := loadUserByID(s.db, id)
	if err != nil {
		s.logReadFailure("FindByID", err)
		if s.nowUTC().Sub(s.lastSuccess) > authReadGrace {
			return nil
		}
		return cloneUser(s.lastByID[id])
	}
	s.lastSuccess = s.nowUTC()
	// Do not overwrite the full snapshot with the hash-free projection.
	return cloneUser(user)
}

// FindByUsername returns a copy of the account, or nil when it does not exist.
// Like FindByID it omits recovery hashes: login and setup only need the
// credential and TOTP fields, and recovery consumes hashes through Update.
func (s *Users) FindByUsername(username string) *auth.User {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, err := loadUserByUsername(s.db, username)
	if err != nil {
		s.logReadFailure("FindByUsername", err)
		if s.nowUTC().Sub(s.lastSuccess) > authReadGrace {
			return nil
		}
		for _, candidate := range s.lastByID {
			if candidate.Username == username {
				return cloneUser(candidate)
			}
		}
		return nil
	}
	s.rememberLocked(user)
	return cloneUser(user)
}

// Count reports the number of accounts. A query failure falls back to the last
// known count so setupRequired cannot flip on a transient error.
func (s *Users) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		s.logReadFailure("Count", err)
		return s.lastCount
	}
	s.lastCount = count
	s.lastSuccess = s.nowUTC()
	return count
}

// Create persists an account. Recovery hashes are written in the same
// transaction, so a partially created account can never be observed. The first
// account also claims legacy watchlist rows that have no owner yet.
func (s *Users) Create(user auth.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin account transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	candidate := cloneUser(&user)
	if err := validateUser(candidate); err != nil {
		return fmt.Errorf("store: invalid account: %w", err)
	}

	var existing int
	if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", candidate.Username).Scan(&existing); err != nil {
		return fmt.Errorf("store: check username: %w", err)
	}
	if existing > 0 {
		return auth.ErrUserExists
	}

	var count int
	if err := tx.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("store: count accounts: %w", err)
	}

	if err := insertUser(tx, candidate); err != nil {
		return err
	}
	if count == 0 {
		// Insert first, then claim: the foreign key requires the account row.
		if _, err := tx.Exec(
			"UPDATE OR IGNORE watchlist SET user_id = ? WHERE user_id IS NULL",
			candidate.ID,
		); err != nil {
			return fmt.Errorf("store: claim migrated watchlist: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit account: %w", err)
	}
	s.rememberLocked(candidate)
	s.lastCount = count + 1
	s.lastSuccess = s.nowUTC()
	return nil
}

// Update applies fn to a copy of one account and persists the result. An fn
// error or validation failure leaves both the database and the recovery codes
// unchanged. Updating lastTotpStep and removing a recovery code therefore commit
// together with the user row.
func (s *Users) Update(userID string, fn func(*auth.User) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin account transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := loadUserFullByID(tx, userID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("store: update account: %w", auth.ErrUserNotFound)
	}

	next := cloneUser(current)
	if err := fn(next); err != nil {
		return err
	}
	if next.ID != current.ID {
		return errors.New("store: account id is immutable")
	}
	if next.Username != current.Username {
		return errors.New("store: username is immutable")
	}
	if err := validateUser(next); err != nil {
		return fmt.Errorf("store: invalid account: %w", err)
	}
	if err := updateUser(tx, next); err != nil {
		return err
	}
	if err := replaceRecoveryHashes(tx, next); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit account: %w", err)
	}
	s.rememberLocked(next)
	s.lastSuccess = s.nowUTC()
	return nil
}

func (s *Users) rememberLocked(user *auth.User) {
	if user == nil {
		return
	}
	if s.lastByID == nil {
		s.lastByID = make(map[string]*auth.User)
	}
	s.lastByID[user.ID] = cloneUser(user)
}

// logReadFailure emits one bounded warning per readLogInterval. It never logs
// SQL text or arguments.
func (s *Users) logReadFailure(operation string, err error) {
	now := s.nowUTC()
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

func loadUsers(q queryer) (map[string]*auth.User, error) {
	rows, err := q.Query("SELECT id FROM users ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("store: read accounts: %w", err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("store: scan account: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("store: read accounts: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("store: read accounts: %w", err)
	}

	users := make(map[string]*auth.User, len(ids))
	for _, id := range ids {
		user, err := loadUserFullByID(q, id)
		if err != nil {
			return nil, err
		}
		if user != nil {
			users[id] = user
		}
	}
	return users, nil
}

// loadUserByID is the hot-path projection: session and TOTP checks do not need
// recovery-code hashes.
func loadUserByID(q queryer, id string) (*auth.User, error) {
	return loadUser(q, `SELECT id, username, password_hash, totp_secret, totp_enabled,
		last_totp_step, created_at, updated_at FROM users WHERE id = ?`, id, false)
}

func loadUserFullByID(q queryer, id string) (*auth.User, error) {
	return loadUser(q, `SELECT id, username, password_hash, totp_secret, totp_enabled,
		last_totp_step, created_at, updated_at FROM users WHERE id = ?`, id, true)
}

func loadUserByUsername(q queryer, username string) (*auth.User, error) {
	// Login resolves the account by username but verifies only the password and
	// TOTP state; recovery hashes are loaded by the full update path that consumes
	// them.
	return loadUser(q, `SELECT id, username, password_hash, totp_secret, totp_enabled,
		last_totp_step, created_at, updated_at FROM users WHERE username = ?`, username, false)
}

func loadUser(q queryer, query string, argument string, withRecovery bool) (*auth.User, error) {
	row := q.QueryRow(query, argument)

	var (
		user                 auth.User
		totpEnabled          int
		createdAt, updatedAt string
	)
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.TOTPSecret,
		&totpEnabled,
		&user.LastTOTPStep,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: read account: %w", err)
	}
	user.TOTPEnabled = totpEnabled != 0

	var err error
	if user.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return nil, fmt.Errorf("store: account created_at: %w", err)
	}
	if user.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return nil, fmt.Errorf("store: account updated_at: %w", err)
	}

	if withRecovery {
		hashes, err := loadRecoveryHashes(q, user.ID)
		if err != nil {
			return nil, err
		}
		user.RecoveryCodeHashes = hashes
	}

	if err := validateUser(&user); err != nil {
		return nil, fmt.Errorf("store: invalid account: %w", err)
	}
	return &user, nil
}

func loadRecoveryHashes(q queryer, userID string) ([]string, error) {
	rows, err := q.Query(
		"SELECT code_hash FROM recovery_codes WHERE user_id = ? ORDER BY rowid",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("store: read recovery codes: %w", err)
	}
	defer rows.Close()

	hashes := []string{}
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, fmt.Errorf("store: scan recovery code: %w", err)
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: read recovery codes: %w", err)
	}
	return hashes, nil
}

func insertUser(tx *sql.Tx, user *auth.User) error {
	if _, err := tx.Exec(`INSERT INTO users
		(id, username, password_hash, totp_secret, totp_enabled, last_totp_step, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID,
		user.Username,
		user.PasswordHash,
		user.TOTPSecret,
		boolToInt(user.TOTPEnabled),
		user.LastTOTPStep,
		encodeTimestamp(user.CreatedAt),
		encodeTimestamp(user.UpdatedAt),
	); err != nil {
		return fmt.Errorf("store: insert account: %w", err)
	}
	return replaceRecoveryHashes(tx, user)
}

func updateUser(tx *sql.Tx, user *auth.User) error {
	result, err := tx.Exec(`UPDATE users SET
		username = ?, password_hash = ?, totp_secret = ?, totp_enabled = ?,
		last_totp_step = ?, created_at = ?, updated_at = ?
		WHERE id = ?`,
		user.Username,
		user.PasswordHash,
		user.TOTPSecret,
		boolToInt(user.TOTPEnabled),
		user.LastTOTPStep,
		encodeTimestamp(user.CreatedAt),
		encodeTimestamp(user.UpdatedAt),
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("store: update account: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update account: %w", err)
	}
	if affected != 1 {
		return errors.New("store: account row disappeared")
	}
	return nil
}

func replaceRecoveryHashes(tx *sql.Tx, user *auth.User) error {
	if _, err := tx.Exec("DELETE FROM recovery_codes WHERE user_id = ?", user.ID); err != nil {
		return fmt.Errorf("store: clear recovery codes: %w", err)
	}
	for _, hash := range user.RecoveryCodeHashes {
		if _, err := tx.Exec(
			"INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)",
			user.ID, hash,
		); err != nil {
			return fmt.Errorf("store: insert recovery code: %w", err)
		}
	}
	return nil
}

func cloneUser(user *auth.User) *auth.User {
	if user == nil {
		return nil
	}
	copied := *user
	copied.RecoveryCodeHashes = append([]string(nil), user.RecoveryCodeHashes...)
	return &copied
}

func cloneUsers(users map[string]*auth.User) map[string]*auth.User {
	cloned := make(map[string]*auth.User, len(users))
	for id, user := range users {
		cloned[id] = cloneUser(user)
	}
	return cloned
}

func validateUser(user *auth.User) error {
	if !strings.HasPrefix(user.ID, "usr_") || len(user.ID) <= len("usr_") {
		return errors.New("missing or malformed id")
	}
	if !auth.ValidUsername(user.Username) {
		return errors.New("invalid username")
	}
	if !auth.IsPasswordHash(user.PasswordHash) {
		return errors.New("malformed password hash")
	}
	if !auth.ValidTOTPSecret(user.TOTPSecret) {
		return errors.New("malformed totp secret")
	}
	if !user.TOTPEnabled {
		return errors.New("totp must be enabled")
	}
	if user.LastTOTPStep < 0 {
		return errors.New("invalid last totp step")
	}
	if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
		return errors.New("missing timestamps")
	}
	if len(user.RecoveryCodeHashes) > auth.RecoveryCodeCount {
		return errors.New("too many recovery codes")
	}
	seen := make(map[string]struct{}, len(user.RecoveryCodeHashes))
	hashes := make([]string, 0, len(user.RecoveryCodeHashes))
	for _, stored := range user.RecoveryCodeHashes {
		normalized := strings.ToLower(stored)
		if len(normalized) != 64 {
			return errors.New("malformed recovery code hash")
		}
		if _, err := hex.DecodeString(normalized); err != nil {
			return errors.New("malformed recovery code hash")
		}
		if _, duplicate := seen[normalized]; duplicate {
			continue
		}
		seen[normalized] = struct{}{}
		hashes = append(hashes, normalized)
	}
	user.RecoveryCodeHashes = hashes
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// timestampLayouts accepts this store's RFC3339Nano output plus common SQLite
// text forms, so a hand-edited or externally migrated database still loads.
var timestampLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
}

func encodeTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTimestamp(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	for _, layout := range timestampLayouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", value)
}
