package auth

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"stock-dashboard/server/internal/domain"
)

const servicePassword = "correct horse battery"

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(delta time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delta)
	c.mu.Unlock()
}

type memStore struct {
	mu        sync.Mutex
	users     map[string]*User
	byName    map[string]string
	updateErr error
}

func newMemStore() *memStore {
	return &memStore{users: map[string]*User{}, byName: map[string]string{}}
}

func cloneUserForTest(user *User) *User {
	if user == nil {
		return nil
	}
	copied := *user
	copied.RecoveryCodeHashes = append([]string(nil), user.RecoveryCodeHashes...)
	return &copied
}

func (m *memStore) FindByID(id string) *User {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneUserForTest(m.users[id])
}

func (m *memStore) FindByUsername(username string) *User {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneUserForTest(m.users[m.byName[username]])
}

func (m *memStore) Create(user User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byName[user.Username]; exists {
		return ErrUserExists
	}
	m.users[user.ID] = cloneUserForTest(&user)
	m.byName[user.Username] = user.ID
	return nil
}

func (m *memStore) Update(userID string, fn func(*User) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	current := m.users[userID]
	if current == nil {
		return errNoOwner
	}
	next := cloneUserForTest(current)
	if err := fn(next); err != nil {
		return err
	}
	m.users[userID] = next
	m.byName[next.Username] = userID
	return nil
}

func (m *memStore) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.users)
}

// owner returns the "owner" account used by most service tests.
func (m *memStore) owner() *User {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneUserForTest(m.users[m.byName["owner"]])
}

func newTestService(t *testing.T) (*Service, *memStore, *testClock) {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)}
	store := newMemStore()
	service, err := New(store, Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}
	return service, store, clock
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, at)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func stepOf(at time.Time) int64 {
	return at.Unix() / int64(TOTPPeriod/time.Second)
}

func TestSetupStaysPending(t *testing.T) {
	service, store, _ := newTestService(t)

	material, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if material.EnrollmentToken == "" || material.Secret == "" || material.OTPAuthURL == "" {
		t.Fatalf("material = %+v", material)
	}
	if len(material.RecoveryCodes) != RecoveryCodeCount {
		t.Fatalf("recovery codes = %d", len(material.RecoveryCodes))
	}
	if store.owner() != nil {
		t.Fatal("setup persisted an owner")
	}
	status := service.Status("")
	if !status.SetupRequired || status.Authenticated || status.User != nil {
		t.Fatalf("status = %+v", status)
	}

	// A resubmission replaces the pending setup.
	second, err := service.BeginSetup("owner", "another strong password")
	if err != nil {
		t.Fatal(err)
	}
	if second.Secret == material.Secret {
		t.Fatal("replacement reused the secret")
	}
	_, err = service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, time.Now()))
	if !domain.IsCode(err, domain.CodeEnrollmentExpired) {
		t.Fatalf("old enrollment error = %v", err)
	}
}

func TestSetupValidation(t *testing.T) {
	service, _, _ := newTestService(t)
	for _, username := range []string{"ab", "_owner", "Owner", "OWNER", "owner!", ""} {
		_, err := service.BeginSetup(username, servicePassword)
		if !domain.IsCode(err, domain.CodeInvalidUsername) {
			t.Fatalf("username %q error = %v", username, err)
		}
	}
	for _, password := range []string{"", strings.Repeat("a", 11)} {
		_, err := service.BeginSetup("owner", password)
		if !domain.IsCode(err, domain.CodeWeakPassword) {
			t.Fatalf("password length %d error = %v", len(password), err)
		}
	}
	if _, err := service.BeginSetup("owner", strings.Repeat("a", 512)); err != nil {
		t.Fatalf("long password rejected: %v", err)
	}
}

func TestActivatePersistsOwnerAndSession(t *testing.T) {
	service, store, clock := newTestService(t)
	material, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}

	session, err := service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now()))
	if err != nil {
		t.Fatal(err)
	}
	owner := store.owner()
	if owner == nil {
		t.Fatal("activation did not persist the owner")
	}
	if owner.Username != "owner" || !owner.TOTPEnabled {
		t.Fatalf("owner = %+v", owner)
	}
	if owner.LastTOTPStep != stepOf(clock.Now()) {
		t.Fatalf("last step = %d", owner.LastTOTPStep)
	}
	if len(owner.RecoveryCodeHashes) != RecoveryCodeCount {
		t.Fatalf("recovery hashes = %d", len(owner.RecoveryCodeHashes))
	}
	if strings.Contains(owner.PasswordHash, servicePassword) {
		t.Fatal("password stored in plaintext")
	}
	if account, ok := service.Authenticate(session.Token); !ok || account.Username != "owner" {
		t.Fatalf("session does not authenticate: %+v", account)
	}
	status := service.Status(session.Token)
	if status.SetupRequired || !status.Authenticated || status.User == nil {
		t.Fatalf("status = %+v", status)
	}
	if status.User.Username != "owner" || !status.User.TOTPEnabled {
		t.Fatalf("user = %+v", status.User)
	}

	// Setup for an existing username is refused.
	if _, err := service.BeginSetup("owner", servicePassword); !domain.IsCode(err, domain.CodeUsernameTaken) {
		t.Fatalf("duplicate setup error = %v", err)
	}
	// Activating twice is not possible.
	if _, err := service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now())); !domain.IsCode(err, domain.CodeEnrollmentExpired) {
		t.Fatalf("second activation error = %v", err)
	}
}

func TestActivateRejectsInvalidCodesAndAttempts(t *testing.T) {
	service, store, clock := newTestService(t)
	material, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}

	for attempt := 0; attempt < EnrollmentMaxAttempts; attempt++ {
		_, err := service.Activate(material.EnrollmentToken, "000000")
		if !domain.IsCode(err, domain.CodeInvalidCode) {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
	}
	if store.owner() != nil {
		t.Fatal("invalid activation persisted an owner")
	}
	_, err = service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now()))
	if !domain.IsCode(err, domain.CodeEnrollmentExpired) {
		t.Fatalf("destroyed enrollment error = %v", err)
	}

	// Restarting setup while the username is blocked is rate limited.
	fresh, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Activate(fresh.EnrollmentToken, codeAt(t, fresh.Secret, clock.Now())); !domain.IsCode(err, domain.CodeRateLimited) {
		t.Fatalf("blocked activation error = %v", err)
	}
	clock.Advance(RateBlock + time.Second)
	if _, err := service.Activate(fresh.EnrollmentToken, codeAt(t, fresh.Secret, clock.Now())); err != nil {
		t.Fatalf("activation after block = %v", err)
	}
}

func TestEnrollmentExpiry(t *testing.T) {
	service, _, clock := newTestService(t)
	material, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(EnrollmentTTL + time.Second)
	_, err = service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now()))
	if !domain.IsCode(err, domain.CodeEnrollmentExpired) {
		t.Fatalf("error = %v", err)
	}
}

func activateOwner(t *testing.T, service *Service, clock *testClock) (*User, *Session) {
	t.Helper()
	material, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now()))
	if err != nil {
		t.Fatal(err)
	}
	// Activation consumed the current TOTP step; move past it so later logins
	// exercise a fresh step.
	clock.Advance(TOTPPeriod)
	return &User{TOTPSecret: material.Secret}, session
}

func TestLoginIsGeneric(t *testing.T) {
	service, _, clock := newTestService(t)
	activateOwner(t, service, clock)

	_, unknown := service.Login("nobody", servicePassword)
	_, wrong := service.Login("owner", "some other password")
	for _, err := range []error{unknown, wrong} {
		if !domain.IsCode(err, domain.CodeInvalidCredentials) {
			t.Fatalf("error = %v", err)
		}
		var failure *domain.Error
		errors.As(err, &failure)
		if failure.Message != "Invalid username or password." {
			t.Fatalf("message = %q", failure.Message)
		}
	}

	if _, err := service.Login("", servicePassword); !domain.IsCode(err, domain.CodeInvalidRequest) {
		t.Fatalf("missing username error = %v", err)
	}
	if _, err := service.Login("owner", ""); !domain.IsCode(err, domain.CodeInvalidRequest) {
		t.Fatalf("missing password error = %v", err)
	}
}

func TestBusyVerificationSlotDoesNotConsumeFailureBudget(t *testing.T) {
	service, _, clock := newTestService(t)
	activateOwner(t, service, clock)

	// Saturate the semaphore so Login cannot verify this attempt.
	for i := 0; i < cap(service.verifySlots); i += 1 {
		service.verifySlots <- struct{}{}
	}
	if _, err := service.Login("owner", servicePassword); !domain.IsCode(err, domain.CodeRateLimited) {
		t.Fatalf("busy slot error = %v, want RATE_LIMITED", err)
	}
	for i := 0; i < cap(service.verifySlots); i += 1 {
		<-service.verifySlots
	}

	// The reservation must have been released, so the correct password still works
	// instead of being reported as a wrong password or a spent budget.
	if _, err := service.Login("owner", servicePassword); err != nil {
		t.Fatalf("login after slot release: %v", err)
	}
}

func TestSetupAndActivateAreSerialised(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)}
	base := newMemStore()
	blocking := &blockingStore{
		memStore: base,
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
	service, err := New(blocking, Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}

	activation := make(chan error, 1)
	go func() {
		_, activateErr := service.Activate(first.EnrollmentToken, codeAt(t, first.Secret, clock.Now()))
		activation <- activateErr
	}()
	<-blocking.started

	replacement := make(chan error, 1)
	go func() {
		_, setupErr := service.BeginSetup("owner", servicePassword)
		replacement <- setupErr
	}()

	select {
	case err := <-replacement:
		t.Fatalf("replacement setup returned before activation finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(blocking.release)
	if err := <-activation; err != nil {
		t.Fatalf("activation = %v", err)
	}
	if err := <-replacement; !domain.IsCode(err, domain.CodeUsernameTaken) {
		t.Fatalf("replacement setup error = %v, want USERNAME_TAKEN", err)
	}
}

type blockingStore struct {
	*memStore
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingStore) Create(user User) error {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return b.memStore.Create(user)
}

func TestLoginRehashesLegacyArgon2Hash(t *testing.T) {
	service, store, clock := newTestService(t)
	legacy, err := HashPasswordArgon2(servicePassword, fastArgon2, nil)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := GenerateTOTPSecret(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(User{
		ID:           "usr_legacy",
		Username:     "owner",
		PasswordHash: legacy,
		TOTPSecret:   secret,
		TOTPEnabled:  true,
		CreatedAt:    clock.now,
		UpdatedAt:    clock.now,
	}); err != nil {
		t.Fatal(err)
	}

	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	updated := store.owner()
	if !strings.HasPrefix(updated.PasswordHash, "$pbkdf2-sha256$") {
		t.Fatalf("legacy hash was not upgraded: %q", updated.PasswordHash)
	}
	if updated.PasswordHash == legacy {
		t.Fatal("stored hash still equals the legacy Argon2 value")
	}
	if matches, err := VerifyPassword(updated.PasswordHash, servicePassword); err != nil || !matches {
		t.Fatalf("upgraded hash does not verify: %v", err)
	}
	if _, err := service.CompleteTOTP(challenge.Token, codeAt(t, secret, clock.now)); err != nil {
		t.Fatalf("login after rehash failed: %v", err)
	}
}

func TestPasswordRateLimitWindowSurvivesTheBlock(t *testing.T) {
	service, _, clock := newTestService(t)
	activateOwner(t, service, clock)

	for attempt := 0; attempt < RateFailures; attempt++ {
		_, _ = service.Login("owner", "wrong password")
	}
	clock.Advance(RateBlock + time.Second)

	// The block elapsed, but the five failures are still inside the 15-minute
	// window. A fresh burst must re-block rather than reset the budget.
	for attempt := 0; attempt < RateFailures; attempt++ {
		_, _ = service.Login("owner", "wrong password")
	}
	if _, err := service.Login("owner", "wrong password"); !domain.IsCode(err, domain.CodeRateLimited) {
		t.Fatalf("error = %v, want RATE_LIMITED", err)
	}
}

func TestPasswordRateLimit(t *testing.T) {
	service, _, clock := newTestService(t)
	activateOwner(t, service, clock)

	for attempt := 0; attempt < RateFailures; attempt++ {
		if _, err := service.Login("owner", "wrong password"); !domain.IsCode(err, domain.CodeInvalidCredentials) {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
	}
	_, err := service.Login("owner", "wrong password")
	if !domain.IsCode(err, domain.CodeRateLimited) {
		t.Fatalf("sixth attempt error = %v", err)
	}
	var limited *domain.Error
	errors.As(err, &limited)
	if limited.RetryAfter != RateBlock {
		t.Fatalf("retry after = %s", limited.RetryAfter)
	}
	if limited.Message != "Too many requests. Try again in 60 seconds." {
		t.Fatalf("message = %q", limited.Message)
	}

	// Another username has its own window.
	if _, err := service.Login("other", "wrong password"); !domain.IsCode(err, domain.CodeInvalidCredentials) {
		t.Fatalf("other username error = %v", err)
	}

	clock.Advance(RateBlock + time.Second)
	if _, err := service.Login("owner", servicePassword); err != nil {
		t.Fatalf("login after block = %v", err)
	}
	// A success clears the window.
	for attempt := 0; attempt < RateFailures-1; attempt++ {
		_, _ = service.Login("owner", "wrong password")
	}
	if _, err := service.Login("owner", servicePassword); err != nil {
		t.Fatalf("failures were not cleared: %v", err)
	}
}

func TestTOTPChallengeAndReplay(t *testing.T) {
	service, store, clock := newTestService(t)
	_, _ = activateOwner(t, service, clock)

	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if challenge.ExpiresInSeconds != int(ChallengeTTL/time.Second) {
		t.Fatalf("expires = %d", challenge.ExpiresInSeconds)
	}

	code := codeAt(t, store.owner().TOTPSecret, clock.Now())
	session, err := service.CompleteTOTP(challenge.Token, code)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := service.Authenticate(session.Token); !ok {
		t.Fatal("session does not authenticate")
	}

	// The same step cannot be replayed, even from a fresh challenge.
	replay, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteTOTP(replay.Token, code); !domain.IsCode(err, domain.CodeInvalidCode) {
		t.Fatalf("replay error = %v", err)
	}
	// And a reused challenge token is gone.
	if _, err := service.CompleteTOTP(challenge.Token, code); !domain.IsCode(err, domain.CodeChallengeExpired) {
		t.Fatalf("consumed challenge error = %v", err)
	}

	// A later step works.
	clock.Advance(TOTPPeriod)
	if _, err := service.CompleteTOTP(replay.Token, codeAt(t, store.owner().TOTPSecret, clock.Now())); err != nil {
		t.Fatal(err)
	}
}

func TestTOTPReplaySurvivesRestart(t *testing.T) {
	service, store, clock := newTestService(t)
	activateOwner(t, service, clock)

	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	code := codeAt(t, store.owner().TOTPSecret, clock.Now())
	if _, err := service.CompleteTOTP(challenge.Token, code); err != nil {
		t.Fatal(err)
	}

	// A new service over the same store models a process restart: sessions are
	// gone but the replay step is persisted.
	restarted, err := New(store, Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}
	challenge, err = restarted.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.CompleteTOTP(challenge.Token, code); !domain.IsCode(err, domain.CodeInvalidCode) {
		t.Fatalf("replay after restart error = %v", err)
	}
}

func TestChallengeExpiry(t *testing.T) {
	service, _, clock := newTestService(t)
	_, _ = activateOwner(t, service, clock)
	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(ChallengeTTL + time.Second)
	if _, err := service.CompleteTOTP(challenge.Token, "123456"); !domain.IsCode(err, domain.CodeChallengeExpired) {
		t.Fatalf("error = %v", err)
	}
}

func TestChallengeAttemptLimit(t *testing.T) {
	service, _, clock := newTestService(t)
	_, _ = activateOwner(t, service, clock)
	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < ChallengeMaxAttempts; attempt++ {
		if _, err := service.CompleteTOTP(challenge.Token, "000000"); !domain.IsCode(err, domain.CodeInvalidCode) {
			t.Fatalf("attempt %d error = %v", attempt, err)
		}
	}
	if _, err := service.CompleteTOTP(challenge.Token, "123456"); !domain.IsCode(err, domain.CodeChallengeExpired) {
		t.Fatalf("error = %v", err)
	}
}

func TestRecoveryCodeIsSingleUse(t *testing.T) {
	service, store, clock := newTestService(t)
	material, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now())); err != nil {
		t.Fatal(err)
	}

	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	messy := strings.ToLower(strings.ReplaceAll(material.RecoveryCodes[0], "-", " "))
	if _, err := service.CompleteRecovery(challenge.Token, messy); err != nil {
		t.Fatal(err)
	}
	if got := len(store.owner().RecoveryCodeHashes); got != RecoveryCodeCount-1 {
		t.Fatalf("recovery hashes = %d", got)
	}

	challenge, err = service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteRecovery(challenge.Token, material.RecoveryCodes[0]); !domain.IsCode(err, domain.CodeInvalidRecoveryCode) {
		t.Fatalf("reused code error = %v", err)
	}
	if _, err := service.CompleteRecovery(challenge.Token, "not-a-code"); !domain.IsCode(err, domain.CodeInvalidRecoveryCode) {
		t.Fatalf("malformed code error = %v", err)
	}
}

func TestSessionLifetimesAndLogout(t *testing.T) {
	service, _, clock := newTestService(t)
	_, session := activateOwner(t, service, clock)

	clock.Advance(SessionIdleTTL - time.Minute)
	if _, ok := service.Authenticate(session.Token); !ok {
		t.Fatal("session expired early")
	}
	clock.Advance(SessionIdleTTL - time.Minute)
	if _, ok := service.Authenticate(session.Token); !ok {
		t.Fatal("idle window did not slide")
	}
	clock.Advance(SessionIdleTTL + time.Minute)
	if _, ok := service.Authenticate(session.Token); ok {
		t.Fatal("idle session stayed valid")
	}

	// Absolute expiry wins over activity.
	_, absolute := activateOwner2(t, service, clock)
	for elapsed := time.Duration(0); elapsed < SessionAbsoluteTTL; elapsed += SessionIdleTTL / 2 {
		clock.Advance(SessionIdleTTL / 2)
		service.Authenticate(absolute.Token)
	}
	clock.Advance(SessionIdleTTL)
	if _, ok := service.Authenticate(absolute.Token); ok {
		t.Fatal("absolute session stayed valid")
	}

	_, logoutSession := activateOwner2(t, service, clock)
	service.Logout(logoutSession.Token)
	if _, ok := service.Authenticate(logoutSession.Token); ok {
		t.Fatal("logout did not revoke")
	}
	service.Logout("") // idempotent
}

// activateOwner2 logs in a fresh session without touching the store: the owner
// already exists, so this drives password + TOTP.
func activateOwner2(t *testing.T, service *Service, clock *testClock) (*User, *Session) {
	t.Helper()
	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(TOTPPeriod)
	owner := service.users.FindByUsername("owner")
	if owner == nil {
		t.Fatal("owner missing")
	}
	session, err := service.CompleteTOTP(challenge.Token, codeAt(t, owner.TOTPSecret, clock.Now()))
	if err != nil {
		t.Fatal(err)
	}
	return owner, session
}

func TestTOTPPersistenceFailureFailsClosed(t *testing.T) {
	service, store, clock := newTestService(t)
	activateOwner(t, service, clock)
	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.updateErr = errors.New("disk on fire")
	store.mu.Unlock()

	if _, err := service.CompleteTOTP(challenge.Token, codeAt(t, store.owner().TOTPSecret, clock.Now())); err == nil {
		t.Fatal("login succeeded despite a persistence failure")
	} else if domain.IsCode(err, domain.CodeInvalidCode) {
		t.Fatalf("persistence failure was hidden as a code failure: %v", err)
	}

	store.mu.Lock()
	store.updateErr = nil
	store.mu.Unlock()
	clock.Advance(TOTPPeriod)
	if _, err := service.CompleteTOTP(challenge.Token, codeAt(t, store.owner().TOTPSecret, clock.Now())); err != nil {
		t.Fatalf("retry after persistence recovery = %v", err)
	}
}

func TestTOTPAccountMissingIsClassified(t *testing.T) {
	service, store, clock := newTestService(t)
	activateOwner(t, service, clock)
	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.updateErr = ErrUserNotFound
	store.mu.Unlock()

	_, err = service.CompleteTOTP(challenge.Token, codeAt(t, store.owner().TOTPSecret, clock.Now()))
	if !domain.IsCode(err, domain.CodeChallengeExpired) {
		t.Fatalf("error = %v, want CHALLENGE_EXPIRED", err)
	}
}

func TestStatusWhenNoOwner(t *testing.T) {
	service, _, _ := newTestService(t)
	status := service.Status("anything")
	if !status.SetupRequired || status.Authenticated || status.User != nil {
		t.Fatalf("status = %+v", status)
	}
	if _, ok := service.Authenticate("anything"); ok {
		t.Fatal("forged session accepted")
	}
}

func TestPendingSetupIsDiscardedOnRestart(t *testing.T) {
	service, store, clock := newTestService(t)
	material, err := service.BeginSetup("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}

	// A new process starts with empty in-memory state.
	restarted, err := New(store, Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}
	if status := restarted.Status(""); !status.SetupRequired || status.Authenticated {
		t.Fatalf("status after restart = %+v", status)
	}
	_, err = restarted.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now()))
	if !domain.IsCode(err, domain.CodeEnrollmentExpired) {
		t.Fatalf("error = %v", err)
	}
	// The owner can set up again from scratch.
	if _, err := restarted.BeginSetup("owner", servicePassword); err != nil {
		t.Fatalf("restart setup error = %v", err)
	}
}

func TestAccountsAreIsolatedAndSessionsCoexist(t *testing.T) {
	service, store, clock := newTestService(t)

	signup := func(username string) (*Session, string) {
		t.Helper()
		material, err := service.BeginSetup(username, servicePassword)
		if err != nil {
			t.Fatalf("%s setup: %v", username, err)
		}
		session, err := service.Activate(material.EnrollmentToken, codeAt(t, material.Secret, clock.Now()))
		if err != nil {
			t.Fatalf("%s activate: %v", username, err)
		}
		clock.Advance(TOTPPeriod)
		return session, material.Secret
	}

	aliceSession, aliceSecret := signup("alice")
	bobSession, bobSecret := signup("bob")
	if got := store.Count(); got != 2 {
		t.Fatalf("count = %d", got)
	}
	if aliceSession.User.ID == "" || bobSession.User.ID == "" || aliceSession.User.ID == bobSession.User.ID {
		t.Fatalf("session ids = %q / %q", aliceSession.User.ID, bobSession.User.ID)
	}

	alice, ok := service.Authenticate(aliceSession.Token)
	if !ok || alice.Username != "alice" || alice.ID != aliceSession.User.ID {
		t.Fatalf("alice session = %+v, %v", alice, ok)
	}
	bob, ok := service.Authenticate(bobSession.Token)
	if !ok || bob.Username != "bob" || bob.ID != bobSession.User.ID {
		t.Fatalf("bob session = %+v, %v", bob, ok)
	}

	// Duplicate usernames are refused before they become pending.
	if _, err := service.BeginSetup("alice", servicePassword); !domain.IsCode(err, domain.CodeUsernameTaken) {
		t.Fatalf("duplicate setup error = %v", err)
	}

	// Logging one account out leaves the other session alive.
	service.Logout(aliceSession.Token)
	if _, ok := service.Authenticate(aliceSession.Token); ok {
		t.Fatal("alice session survived logout")
	}
	if _, ok := service.Authenticate(bobSession.Token); !ok {
		t.Fatal("bob session was revoked by alice's logout")
	}

	// Each account logs in with its own secret and TOTP replay state.
	aliceChallenge, err := service.Login("alice", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteTOTP(aliceChallenge.Token, codeAt(t, aliceSecret, clock.Now())); err != nil {
		t.Fatal(err)
	}
	bobChallenge, err := service.Login("bob", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteTOTP(bobChallenge.Token, codeAt(t, bobSecret, clock.Now())); err != nil {
		t.Fatalf("bob login: %v", err)
	}
}

type countingStore struct {
	*memStore
	findUsername atomic.Int32
	findID       atomic.Int32
}

func (c *countingStore) FindByUsername(username string) *User {
	c.findUsername.Add(1)
	return c.memStore.FindByUsername(username)
}

func (c *countingStore) FindByID(id string) *User {
	c.findID.Add(1)
	return c.memStore.FindByID(id)
}

func TestConcurrentLoginBurstCapsVerifications(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)}
	store := &countingStore{memStore: newMemStore()}
	service, err := New(store, Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}
	activateOwner(t, service, clock)
	store.findUsername.Store(0)

	const burst = 20
	start := make(chan struct{})
	var group sync.WaitGroup
	group.Add(burst)
	for range burst {
		go func() {
			defer group.Done()
			<-start
			_, _ = service.Login("owner", "wrong password")
		}()
	}
	close(start)
	group.Wait()

	if got := store.findUsername.Load(); got > RateFailures {
		t.Fatalf("password verifications = %d, want at most %d", got, RateFailures)
	}
}

func TestConcurrentTOTPBurstCapsVerifications(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)}
	store := &countingStore{memStore: newMemStore()}
	service, err := New(store, Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations})
	if err != nil {
		t.Fatal(err)
	}
	activateOwner(t, service, clock)
	challenge, err := service.Login("owner", servicePassword)
	if err != nil {
		t.Fatal(err)
	}
	store.findID.Store(0)

	const burst = 20
	start := make(chan struct{})
	var group sync.WaitGroup
	group.Add(burst)
	for range burst {
		go func() {
			defer group.Done()
			<-start
			_, _ = service.CompleteTOTP(challenge.Token, "000000")
		}()
	}
	close(start)
	group.Wait()

	if got := store.findID.Load(); got > ChallengeMaxAttempts {
		t.Fatalf("totp verifications = %d, want at most %d", got, ChallengeMaxAttempts)
	}
}

// gatedReader blocks the first read after armed is set and signals when the read
// has entered. It produces deterministic bytes, so the test does not depend on
// crypto/rand or timing sleeps.
type gatedReader struct {
	armed   atomic.Bool
	entered chan struct{}
	gate    chan struct{}
	signal  sync.Once
}

func (g *gatedReader) Read(p []byte) (int, error) {
	if g.armed.CompareAndSwap(true, false) {
		g.signal.Do(func() { close(g.entered) })
		<-g.gate
	}
	for index := range p {
		p[index] = byte(index + 1)
	}
	return len(p), nil
}

type setupResult struct {
	material *SetupMaterial
	err      error
}

// TestSecondConcurrentSetupIsRefused drives two real BeginSetup calls: the first
// is parked inside HashPassword, so the second must be refused with 429 before the
// first completes. It fails if hashing moves under setupMu (the second would queue)
// or if the setup slot ever admits two hashes.
func TestSecondConcurrentSetupIsRefused(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)}
	random := &gatedReader{entered: make(chan struct{}), gate: make(chan struct{})}
	service, err := New(newMemStore(), Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations, Rand: random})
	if err != nil {
		t.Fatal(err)
	}

	random.armed.Store(true)
	first := make(chan setupResult, 1)
	go func() {
		material, err := service.BeginSetup("owner", servicePassword)
		first <- setupResult{material: material, err: err}
	}()
	select {
	case <-random.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first BeginSetup never reached the password hash")
	}

	second := make(chan error, 1)
	go func() {
		_, err := service.BeginSetup("second", servicePassword)
		second <- err
	}()
	select {
	case err := <-second:
		if !domain.IsCode(err, domain.CodeRateLimited) {
			t.Fatalf("second setup error = %v, want RATE_LIMITED", err)
		}
	case <-time.After(5 * time.Second):
		close(random.gate)
		t.Fatal("second setup queued behind the first hash; want an immediate 429")
	}

	close(random.gate)
	if result := <-first; result.err != nil || result.material == nil {
		t.Fatalf("first setup = material %v, error %v", result.material, result.err)
	}
	if _, err := service.BeginSetup("third", servicePassword); err != nil {
		t.Fatalf("setup after the first finished = %v", err)
	}
}

func TestGlobalAuthBudgetCapsDistinctUsernames(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)}
	store := newMemStore()
	service, err := New(store, Options{Now: clock.Now, PBKDF2Iterations: fastPBKDF2Iterations, GlobalAuthAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login("alice", "wrong password"); !domain.IsCode(err, domain.CodeInvalidCredentials) {
		t.Fatalf("first error = %v", err)
	}
	if _, err := service.Login("bob", "wrong password"); !domain.IsCode(err, domain.CodeRateLimited) {
		t.Fatalf("second error = %v, want RATE_LIMITED", err)
	}
}
