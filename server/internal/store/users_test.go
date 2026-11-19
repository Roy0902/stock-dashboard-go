package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"stock-dashboard/server/internal/auth"
)

const testPBKDF2Iterations uint32 = 1_000

func recoveryHash(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:])
}

func recoveryHashes(count int) []string {
	hashes := make([]string, count)
	for index := range hashes {
		hashes[index] = recoveryHash(strings.Repeat("code", index+1))
	}
	return hashes
}

// fullProjection loads the recovery hashes the store keeps for the update and
// recovery path. FindByUsername and FindByID deliberately omit them.
func fullProjection(t *testing.T, storage *Store, id string) *auth.User {
	t.Helper()
	user, err := loadUserFullByID(storage.db, id)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatal("full projection: account not found")
	}
	return user
}

func testOwner(t *testing.T) auth.User {
	t.Helper()
	passwordHash, err := auth.HashPassword("correct horse battery", testPBKDF2Iterations, nil)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := auth.GenerateTOTPSecret(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)
	return auth.User{
		ID:                 "usr_0123456789abcdef",
		Username:           "owner",
		PasswordHash:       passwordHash,
		TOTPSecret:         secret,
		TOTPEnabled:        true,
		LastTOTPStep:       1763500000,
		RecoveryCodeHashes: []string{recoveryHash("one"), recoveryHash("two")},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func TestUsersMissingAccount(t *testing.T) {
	storage := mustOpen(t, t.TempDir())
	defer mustClose(t, storage)
	if storage.Users.Count() != 0 {
		t.Fatal("empty database has accounts")
	}
	if storage.Users.FindByUsername("owner") != nil || storage.Users.FindByID("usr_0123456789abcdef") != nil {
		t.Fatal("empty database resolves an account")
	}
}

func TestCreateLoadsAndPersists(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)

	owner := testOwner(t)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}

	loaded := storage.Users.FindByUsername("owner")
	if loaded == nil || loaded.ID != owner.ID || loaded.Username != "owner" {
		t.Fatalf("loaded = %+v", loaded)
	}
	if byID := storage.Users.FindByID(owner.ID); byID == nil || byID.Username != owner.Username {
		t.Fatalf("loaded by id = %+v", byID)
	}
	if loaded.PasswordHash != owner.PasswordHash {
		t.Fatal("password hash did not round-trip")
	}
	if loaded.TOTPSecret != owner.TOTPSecret || !loaded.TOTPEnabled {
		t.Fatal("totp state did not round-trip")
	}
	if loaded.LastTOTPStep != owner.LastTOTPStep {
		t.Fatalf("last step = %d", loaded.LastTOTPStep)
	}
	if len(loaded.RecoveryCodeHashes) != 0 {
		t.Fatalf("FindByUsername loaded recovery hashes: %v", loaded.RecoveryCodeHashes)
	}
	if full := fullProjection(t, storage, owner.ID); !equalStrings(full.RecoveryCodeHashes, owner.RecoveryCodeHashes) {
		t.Fatalf("recovery hashes = %v", full.RecoveryCodeHashes)
	}
	if !loaded.CreatedAt.Equal(owner.CreatedAt) || !loaded.UpdatedAt.Equal(owner.UpdatedAt) {
		t.Fatalf("timestamps = %s / %s", loaded.CreatedAt, loaded.UpdatedAt)
	}

	// Lookups return independent copies.
	loaded.Username = "hacker"
	again := storage.Users.FindByUsername("owner")
	if again.Username != "owner" {
		t.Fatal("FindByUsername leaked its internal state")
	}
	full := fullProjection(t, storage, owner.ID)
	full.RecoveryCodeHashes[0] = "tampered"
	if fresh := fullProjection(t, storage, owner.ID); fresh.RecoveryCodeHashes[0] != owner.RecoveryCodeHashes[0] {
		t.Fatal("full projection leaked its internal state")
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	persisted := reopened.Users.FindByUsername("owner")
	if persisted == nil || persisted.Username != "owner" || persisted.LastTOTPStep != owner.LastTOTPStep {
		t.Fatalf("reopened = %+v", persisted)
	}
	if len(persisted.RecoveryCodeHashes) != 0 {
		t.Fatalf("reopened FindByUsername loaded recovery hashes: %v", persisted.RecoveryCodeHashes)
	}
	if full := fullProjection(t, reopened, owner.ID); !equalStrings(full.RecoveryCodeHashes, owner.RecoveryCodeHashes) {
		t.Fatalf("reopened recovery hashes = %v", full.RecoveryCodeHashes)
	}
}

func TestCreateAllowsMultipleAccounts(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := testOwner(t)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}
	second := testOwner(t)
	second.ID = "usr_ffffffffffffffff"
	second.Username = "other"
	if err := storage.Users.Create(second); err != nil {
		t.Fatal(err)
	}
	if got := storage.Users.Count(); got != 2 {
		t.Fatalf("count = %d", got)
	}
	if got := storage.Users.FindByUsername("other"); got == nil || got.ID != second.ID {
		t.Fatalf("second account = %+v", got)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := reopened.Users.Count(); got != 2 {
		t.Fatalf("count after reopen = %d", got)
	}
}

func TestCreateRejectsDuplicateUsername(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := testOwner(t)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}
	duplicate := testOwner(t)
	duplicate.ID = "usr_ffffffffffffffff"
	if err := storage.Users.Create(duplicate); !errors.Is(err, auth.ErrUserExists) {
		t.Fatalf("duplicate create error = %v", err)
	}
	if got := storage.Users.Count(); got != 1 {
		t.Fatalf("count = %d", got)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if err := reopened.Users.Create(duplicate); !errors.Is(err, auth.ErrUserExists) {
		t.Fatalf("reopen duplicate error = %v", err)
	}
}

func TestCreateClaimsOrphanWatchlistOnFirstAccount(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	if _, err := storage.db.Exec(`INSERT INTO watchlist (user_id, position, ticker)
		VALUES (NULL, 0, 'AAPL'), (NULL, 1, 'MSFT')`); err != nil {
		t.Fatal(err)
	}

	first := testOwner(t)
	if err := storage.Users.Create(first); err != nil {
		t.Fatal(err)
	}
	if got := listStrings(storage.Watchlist.List(first.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("claimed list = %v", got)
	}

	second := testOwner(t)
	second.ID = "usr_ffffffffffffffff"
	second.Username = "other"
	if err := storage.Users.Create(second); err != nil {
		t.Fatal(err)
	}
	if got := storage.Watchlist.List(second.ID).Len(); got != 0 {
		t.Fatalf("second account inherited %d rows", got)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := listStrings(reopened.Watchlist.List(first.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("claimed list after reopen = %v", got)
	}
	if got := reopened.Watchlist.List(second.ID).Len(); got != 0 {
		t.Fatalf("second account list after reopen = %d", got)
	}
}

func TestCreateRejectsInvalidAccount(t *testing.T) {
	base := testOwner(t)

	tests := map[string]func(*auth.User){
		"empty id":          func(user *auth.User) { user.ID = "" },
		"malformed id":      func(user *auth.User) { user.ID = "owner" },
		"invalid username":  func(user *auth.User) { user.Username = "Owner" },
		"invalid hash":      func(user *auth.User) { user.PasswordHash = "plaintext" },
		"invalid secret":    func(user *auth.User) { user.TOTPSecret = "not base32!" },
		"totp disabled":     func(user *auth.User) { user.TOTPEnabled = false },
		"negative step":     func(user *auth.User) { user.LastTOTPStep = -1 },
		"missing created":   func(user *auth.User) { user.CreatedAt = time.Time{} },
		"missing updated":   func(user *auth.User) { user.UpdatedAt = time.Time{} },
		"bad recovery":      func(user *auth.User) { user.RecoveryCodeHashes = []string{"abc"} },
		"too many recovery": func(user *auth.User) { user.RecoveryCodeHashes = recoveryHashes(auth.RecoveryCodeCount + 1) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			storage := mustOpen(t, t.TempDir())
			defer mustClose(t, storage)
			account := base
			mutate(&account)
			if err := storage.Users.Create(account); err == nil {
				t.Fatal("invalid account accepted")
			}
			if storage.Users.Count() != 0 {
				t.Fatal("invalid account was persisted")
			}
		})
	}
}

func TestUpdateIsTransactional(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	if err := storage.Users.Update("usr_missing", func(*auth.User) error { return nil }); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("update without an account = %v, want ErrUserNotFound", err)
	}

	owner := testOwner(t)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}

	sentinel := errors.New("refused")
	if err := storage.Users.Update(owner.ID, func(user *auth.User) error {
		user.LastTOTPStep = 99
		user.RecoveryCodeHashes = user.RecoveryCodeHashes[1:]
		user.UpdatedAt = time.Now().UTC()
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	if got := storage.Users.FindByUsername(owner.Username); got.LastTOTPStep != owner.LastTOTPStep {
		t.Fatalf("failed update changed state: %+v", got)
	}
	if full := fullProjection(t, storage, owner.ID); !equalStrings(full.RecoveryCodeHashes, owner.RecoveryCodeHashes) {
		t.Fatalf("failed update changed recovery hashes: %v", full.RecoveryCodeHashes)
	}

	updatedAt := owner.UpdatedAt.Add(time.Minute)
	if err := storage.Users.Update(owner.ID, func(user *auth.User) error {
		user.LastTOTPStep = 12345
		user.RecoveryCodeHashes = user.RecoveryCodeHashes[1:]
		user.UpdatedAt = updatedAt
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	persisted := reopened.Users.FindByUsername(owner.Username)
	if persisted.LastTOTPStep != 12345 {
		t.Fatalf("last step = %d", persisted.LastTOTPStep)
	}
	hashes := fullProjection(t, reopened, owner.ID).RecoveryCodeHashes
	if len(hashes) != len(owner.RecoveryCodeHashes)-1 {
		t.Fatalf("recovery hashes = %d", len(hashes))
	}
	if !persisted.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updated_at = %s, want %s", persisted.UpdatedAt, updatedAt)
	}
}

func TestUpdateDoesNotAffectOtherAccounts(t *testing.T) {
	storage := mustOpen(t, t.TempDir())
	defer mustClose(t, storage)
	owner := testOwner(t)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}
	second := testOwner(t)
	second.ID = "usr_ffffffffffffffff"
	second.Username = "other"
	if err := storage.Users.Create(second); err != nil {
		t.Fatal(err)
	}

	if err := storage.Users.Update(second.ID, func(user *auth.User) error {
		user.LastTOTPStep = 777
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := storage.Users.FindByID(owner.ID); got.LastTOTPStep != owner.LastTOTPStep {
		t.Fatalf("first account changed: %+v", got)
	}
	if got := storage.Users.FindByID(second.ID); got.LastTOTPStep != 777 {
		t.Fatalf("second account = %+v", got)
	}
}

func TestUpdateRejectsUsernameAndInvalidRecord(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := testOwner(t)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}

	tests := map[string]func(*auth.User){
		"username":      func(user *auth.User) { user.Username = "other" },
		"id":            func(user *auth.User) { user.ID = "usr_ffffffffffffffff" },
		"password hash": func(user *auth.User) { user.PasswordHash = "plaintext" },
		"totp secret":   func(user *auth.User) { user.TOTPSecret = "not base32!" },
		"totp enabled":  func(user *auth.User) { user.TOTPEnabled = false },
		"last step":     func(user *auth.User) { user.LastTOTPStep = -5 },
		"timestamps":    func(user *auth.User) { user.UpdatedAt = time.Time{} },
		"recovery":      func(user *auth.User) { user.RecoveryCodeHashes = []string{"abc"} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if err := storage.Users.Update(owner.ID, func(user *auth.User) error {
				mutate(user)
				return nil
			}); err == nil {
				t.Fatalf("%s mutation accepted", name)
			}
			loaded := storage.Users.FindByID(owner.ID)
			if loaded.Username != owner.Username || loaded.LastTOTPStep != owner.LastTOTPStep {
				t.Fatalf("failed update changed state: %+v", loaded)
			}
		})
	}
	mustClose(t, storage)
}

func TestRecoveryCodeReplacementIsPersisted(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := testOwner(t)
	owner.RecoveryCodeHashes = recoveryHashes(auth.RecoveryCodeCount)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}

	consumed := owner.RecoveryCodeHashes[0]
	if err := storage.Users.Update(owner.ID, func(user *auth.User) error {
		user.RecoveryCodeHashes = append(user.RecoveryCodeHashes[:0:0], user.RecoveryCodeHashes[1:]...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	hashes := fullProjection(t, reopened, owner.ID).RecoveryCodeHashes
	if len(hashes) != auth.RecoveryCodeCount-1 {
		t.Fatalf("hashes = %d", len(hashes))
	}
	if !equalStrings(hashes, owner.RecoveryCodeHashes[1:]) {
		t.Fatalf("hashes = %v", hashes)
	}
	for _, hash := range hashes {
		if hash == consumed {
			t.Fatal("consumed recovery code is still stored")
		}
	}
}

func TestTimestampsAreRFC3339NanoUTC(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := testOwner(t)
	owner.CreatedAt = time.Date(2026, 11, 18, 5, 0, 0, 123456789, time.FixedZone("CET", 3600))
	owner.UpdatedAt = owner.CreatedAt.Add(90 * time.Second)
	if err := storage.Users.Create(owner); err != nil {
		t.Fatal(err)
	}

	var created, updated string
	if err := storage.db.QueryRow("SELECT created_at, updated_at FROM users").Scan(&created, &updated); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{created, updated} {
		if !strings.HasSuffix(value, "Z") {
			t.Fatalf("timestamp %q is not UTC text", value)
		}
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			t.Fatalf("timestamp %q is not RFC3339Nano: %v", value, err)
		}
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	loaded := reopened.Users.FindByID(owner.ID)
	if !loaded.CreatedAt.Equal(owner.CreatedAt) || !loaded.UpdatedAt.Equal(owner.UpdatedAt) {
		t.Fatalf("timestamps = %s / %s", loaded.CreatedAt, loaded.UpdatedAt)
	}
	if loaded.CreatedAt.Location() != time.UTC {
		t.Fatalf("created_at location = %s", loaded.CreatedAt.Location())
	}
}

func TestValidateUserDedupesRecoveryHashes(t *testing.T) {
	owner := testOwner(t)
	owner.RecoveryCodeHashes = []string{recoveryHash("one"), recoveryHash("one"), recoveryHash("two")}
	if err := validateUser(&owner); err != nil {
		t.Fatal(err)
	}
	if len(owner.RecoveryCodeHashes) != 2 {
		t.Fatalf("hashes = %v", owner.RecoveryCodeHashes)
	}
}
