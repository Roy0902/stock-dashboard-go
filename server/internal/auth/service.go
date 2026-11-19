package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"stock-dashboard/server/internal/domain"
)

// Lifetimes and limits from docs/contracts-auth.md sections 3 and 4.
const (
	SessionAbsoluteTTL = 12 * time.Hour
	SessionIdleTTL     = 30 * time.Minute

	ChallengeTTL         = 5 * time.Minute
	ChallengeMaxAttempts = 5

	EnrollmentTTL         = 15 * time.Minute
	EnrollmentMaxAttempts = 5

	RateWindow   = 15 * time.Minute
	RateFailures = 5
	RateBlock    = 60 * time.Second

	// GlobalAuthWindow and GlobalAuthAttempts bound unauthenticated password
	// work across every username, so distinct names cannot multiply the cost.
	GlobalAuthWindow   = time.Minute
	GlobalAuthAttempts = 60

	// maxConcurrentPasswordVerifications bounds simultaneous Argon2 work.
	maxConcurrentPasswordVerifications = 2
	// pruneInterval amortises state pruning instead of scanning on every request.
	pruneInterval = 64
)

// ErrUserExists is returned by a UserStore.Create when the username is already
// registered.
var ErrUserExists = errors.New("auth: username already exists")

// ErrUserNotFound is returned by a UserStore.Update when the addressed account
// no longer exists. Callers classify it instead of reporting a 500.
var ErrUserNotFound = errors.New("auth: account not found")

// User is the persisted account record. It is written only by the store layer
// and never leaves it except through this package.
type User struct {
	ID                 string    `json:"id"`
	Username           string    `json:"username"`
	PasswordHash       string    `json:"passwordHash"`
	TOTPSecret         string    `json:"totpSecret"`
	TOTPEnabled        bool      `json:"totpEnabled"`
	LastTOTPStep       int64     `json:"lastTotpStep"`
	RecoveryCodeHashes []string  `json:"recoveryCodeHashes"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// UserStore is the persistence boundary the service depends on. Lookups return
// copies (nil when absent), Create fails with ErrUserExists for a duplicate
// username, Update applies fn to the addressed account and writes it atomically
// (returning ErrUserNotFound when the account is gone), and Count reports how
// many accounts exist.
type UserStore interface {
	FindByUsername(username string) *User
	FindByID(id string) *User
	Create(User) error
	Update(userID string, fn func(*User) error) error
	Count() int
}

// Options inject time, randomness and hashing cost. Zero values use production
// defaults; tests supply a fixed clock and a cheap PBKDF2 iteration count.
type Options struct {
	Now              func() time.Time
	Rand             io.Reader
	PBKDF2Iterations uint32
	// GlobalAuthAttempts overrides the per-minute unauthenticated password budget.
	GlobalAuthAttempts int
}

// Account is the public view of an account returned to clients. ID is internal
// only and never marshalled: the contract exposes username and TOTP state.
type Account struct {
	ID          string `json:"-"`
	Username    string `json:"username"`
	TOTPEnabled bool   `json:"totpEnabled"`
}

// Status is the payload behind GET /api/auth/status.
type Status struct {
	SetupRequired bool
	Authenticated bool
	User          *Account
}

// SetupMaterial is returned once by setup: the TOTP secret and recovery codes.
type SetupMaterial struct {
	Username        string
	Secret          string
	OTPAuthURL      string
	RecoveryCodes   []string
	EnrollmentToken string
}

// Challenge is a pending second factor after a correct password.
type Challenge struct {
	Token            string
	ExpiresInSeconds int
}

// Session is an authenticated account session. Only the caller sees the token.
type Session struct {
	Token            string
	User             Account
	ExpiresInSeconds int
}

type sessionEntry struct {
	userID      string
	username    string
	totpEnabled bool
	digest      string
	created     time.Time
	lastSeen    time.Time
}

type challengeEntry struct {
	userID   string
	username string
	digest   string
	expires  time.Time
	attempts int
}

type enrollmentEntry struct {
	username       string
	digest         string
	passwordHash   string
	secret         string
	recoveryHashes []string
	expires        time.Time
	attempts       int
}

type attemptEntry struct {
	failures     []time.Time
	blockedUntil time.Time
}

var (
	errReplay          = errors.New("auth: totp step already used")
	errInvalidRecovery = errors.New("auth: recovery code not found")
	errNoOwner         = errors.New("auth: account is missing")
)

// Service owns password verification, TOTP, recovery codes and the in-memory
// session, challenge, enrollment and rate-limit state.
//
// Lock order: setupMu -> mu. setupMu may span store calls during setup and
// activation (the duplicate-username re-check and users.Create are deliberately
// serialised), but mu is never held across store/SQLite I/O. The UserStore
// mutex is only taken while mu is not held. Store Update callbacks must not
// call back into the store.
type Service struct {
	users            UserStore
	now              func() time.Time
	random           io.Reader
	pbkdf2Iterations uint32
	dummyHash        string

	// setupMu serialises setup and activation so a concurrent BeginSetup cannot
	// replace the enrollment between Activate's snapshot and users.Create.
	setupMu sync.Mutex

	// setupSlot admits one setup hash at a time. A second concurrent BeginSetup
	// is refused instead of queued; hashing never runs under setupMu.
	setupSlot chan struct{}
	// verifySlots bounds simultaneous password verifications and dummy hashes.
	verifySlots chan struct{}

	mu          sync.Mutex
	ops         int
	publicCalls []time.Time
	publicLimit int
	sessions    map[string]*sessionEntry
	challenges  map[string]*challengeEntry
	enrollments map[string]*enrollmentEntry
	attempts    map[string]*attemptEntry
}

// New builds the service. The dummy hash gives unknown usernames an Argon2
// verification of comparable cost so timing does not reveal existence.
func New(users UserStore, opts Options) (*Service, error) {
	if users == nil {
		return nil, errors.New("auth: user store is required")
	}
	publicLimit := opts.GlobalAuthAttempts
	if publicLimit <= 0 {
		publicLimit = GlobalAuthAttempts
	}
	iterations := opts.PBKDF2Iterations
	if iterations == 0 {
		iterations = DefaultPBKDF2Iterations
	}
	service := &Service{
		users:            users,
		now:              opts.Now,
		random:           opts.Rand,
		pbkdf2Iterations: iterations,
		setupSlot:        make(chan struct{}, 1),
		verifySlots:      make(chan struct{}, maxConcurrentPasswordVerifications),
		publicLimit:      publicLimit,
		sessions:         make(map[string]*sessionEntry),
		challenges:       make(map[string]*challengeEntry),
		enrollments:      make(map[string]*enrollmentEntry),
		attempts:         make(map[string]*attemptEntry),
	}
	if service.now == nil {
		service.now = time.Now
	}
	if service.random == nil {
		service.random = rand.Reader
	}
	dummy, err := HashPassword("dummy-password-never-matches", service.pbkdf2Iterations, service.random)
	if err != nil {
		return nil, fmt.Errorf("auth: build dummy hash: %w", err)
	}
	service.dummyHash = dummy
	return service, nil
}

func (s *Service) nowUTC() time.Time {
	return s.now().UTC()
}

// Status reports setup state and whether the session token is valid. A valid
// token is also an activity signal and slides the idle window.
func (s *Service) Status(sessionToken string) Status {
	status := Status{SetupRequired: s.users.Count() == 0}
	if account, ok := s.Authenticate(sessionToken); ok {
		status.Authenticated = true
		status.User = account
	}
	return status
}

// BeginSetup validates credentials and holds a pending enrollment in memory.
// It never persists the account; Activate does that. Any number of accounts may
// register, one unique username each.
func (s *Service) BeginSetup(username, password string) (*SetupMaterial, error) {
	username = NormalizeUsername(username)
	if !ValidUsername(username) {
		return nil, errorFor(domain.CodeInvalidUsername)
	}
	if !ValidPassword(password) {
		return nil, errorFor(domain.CodeWeakPassword)
	}
	if s.users.FindByUsername(username) != nil {
		return nil, errorFor(domain.CodeUsernameTaken)
	}

	now := s.nowUTC()
	s.mu.Lock()
	s.maybePruneLocked(now)
	if retry, limited := s.allowPublicLocked(now); limited {
		s.mu.Unlock()
		return nil, domain.NewRateLimited(retry)
	}
	s.mu.Unlock()

	// Hash outside setupMu so a slow hash cannot block activation, and refuse a
	// second concurrent setup instead of building an unbounded queue.
	if !tryAcquire(s.setupSlot) {
		return nil, domain.NewRateLimited(time.Second)
	}
	passwordHash, err := HashPassword(password, s.pbkdf2Iterations, s.random)
	release(s.setupSlot)
	if err != nil {
		return nil, err
	}
	secret, err := GenerateTOTPSecret(s.random)
	if err != nil {
		return nil, err
	}
	codes, hashes, err := generateRecoveryCodes(s.random, RecoveryCodeCount)
	if err != nil {
		return nil, err
	}
	token, digest, err := newToken(s.random)
	if err != nil {
		return nil, err
	}

	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	// An activation may have committed this username while the hash ran. Re-check
	// under the serialising lock so a pending setup cannot shadow a real account.
	if s.users.FindByUsername(username) != nil {
		return nil, errorFor(domain.CodeUsernameTaken)
	}

	now = s.nowUTC()
	s.mu.Lock()
	s.maybePruneLocked(now)
	// A resubmission replaces that username's pending setup without disturbing
	// enrollments for other names.
	for key, pending := range s.enrollments {
		if pending.username == username {
			delete(s.enrollments, key)
		}
	}
	s.enrollments[digest] = &enrollmentEntry{
		username:       username,
		digest:         digest,
		passwordHash:   passwordHash,
		secret:         secret,
		recoveryHashes: hashes,
		expires:        now.Add(EnrollmentTTL),
	}
	s.mu.Unlock()

	return &SetupMaterial{
		Username:        username,
		Secret:          secret,
		OTPAuthURL:      OTPAuthURL(username, secret),
		RecoveryCodes:   codes,
		EnrollmentToken: token,
	}, nil
}

// Activate verifies the code against the pending secret, persists the account,
// and opens its first session.
func (s *Service) Activate(enrollmentToken, code string) (*Session, error) {
	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	if enrollmentToken == "" {
		return nil, errorFor(domain.CodeEnrollmentExpired)
	}
	digest := hashToken(enrollmentToken)
	now := s.nowUTC()

	s.mu.Lock()
	s.maybePruneLocked(now)
	pending, ok := s.enrollments[digest]
	if !ok || !constantTimeEqual(pending.digest, digest) || !now.Before(pending.expires) {
		if ok {
			delete(s.enrollments, digest)
		}
		s.mu.Unlock()
		return nil, errorFor(domain.CodeEnrollmentExpired)
	}
	if pending.attempts >= EnrollmentMaxAttempts {
		delete(s.enrollments, digest)
		s.mu.Unlock()
		return nil, errorFor(domain.CodeEnrollmentExpired)
	}
	username := pending.username
	if retry, limited := s.rateLimitLocked(username, now); limited {
		s.mu.Unlock()
		return nil, domain.NewRateLimited(retry)
	}
	// Reserve the attempt before verifying so concurrent callers cannot exceed
	// the five-attempt cap.
	pending.attempts++
	secret := pending.secret
	passwordHash := pending.passwordHash
	hashes := append([]string(nil), pending.recoveryHashes...)
	s.mu.Unlock()

	step, valid := ValidateTOTP(secret, code, now)
	if !valid {
		s.mu.Lock()
		if current, ok := s.enrollments[digest]; ok && constantTimeEqual(current.digest, digest) {
			if current.attempts >= EnrollmentMaxAttempts {
				delete(s.enrollments, digest)
			}
		}
		s.recordFailureLocked(username, now)
		s.mu.Unlock()
		return nil, errorFor(domain.CodeInvalidCode)
	}

	id, err := newUserID(s.random)
	if err != nil {
		return nil, err
	}
	user := User{
		ID:                 id,
		Username:           username,
		PasswordHash:       passwordHash,
		TOTPSecret:         secret,
		TOTPEnabled:        true,
		LastTOTPStep:       step,
		RecoveryCodeHashes: hashes,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.users.Create(user); err != nil {
		if errors.Is(err, ErrUserExists) {
			return nil, errorFor(domain.CodeUsernameTaken)
		}
		return nil, err
	}

	s.mu.Lock()
	delete(s.enrollments, digest)
	delete(s.attempts, username)
	s.mu.Unlock()
	return s.createSession(user)
}

// Login verifies username and password and returns a five-minute challenge.
// Failures are generic and rate limited per submitted username.
func (s *Service) Login(username, password string) (*Challenge, error) {
	username = NormalizeUsername(username)
	if username == "" || password == "" {
		return nil, errorFor(domain.CodeInvalidRequest)
	}
	now := s.nowUTC()

	s.mu.Lock()
	s.maybePruneLocked(now)
	if retry, limited := s.rateLimitLocked(username, now); limited {
		s.mu.Unlock()
		return nil, domain.NewRateLimited(retry)
	}
	if retry, limited := s.allowPublicLocked(now); limited {
		s.mu.Unlock()
		return nil, domain.NewRateLimited(retry)
	}
	// Reserve the failure slot before the expensive verification. Concurrent
	// bursts therefore see each other's attempts and cannot exceed the cap.
	s.recordFailureLocked(username, now)
	s.mu.Unlock()

	if !tryAcquire(s.verifySlots) {
		// The attempt slot and the global slot were reserved before this check.
		// Release both: a busy semaphore is not a credential failure, and a
		// correct password must not consume the failure budget.
		s.mu.Lock()
		s.releaseFailureLocked(username, now)
		s.releasePublicLocked(now)
		s.mu.Unlock()
		return nil, domain.NewRateLimited(time.Second)
	}
	user := s.users.FindByUsername(username)
	matches := false
	var verifyErr error
	if user == nil {
		// Dummy verification: same cost as a real one, result ignored.
		_, _ = VerifyPassword(s.dummyHash, password)
	} else {
		matches, verifyErr = VerifyPassword(user.PasswordHash, password)
	}
	release(s.verifySlots)

	if user == nil || verifyErr != nil || !matches {
		return nil, errorFor(domain.CodeInvalidCredentials)
	}
	s.upgradePasswordHash(user.ID, user.PasswordHash, password)
	if !user.TOTPEnabled || !ValidTOTPSecret(user.TOTPSecret) {
		return nil, errorFor(domain.CodeInvalidCredentials)
	}
	s.clearFailures(username)

	token, digest, err := newToken(s.random)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.maybePruneLocked(now)
	for key, challenge := range s.challenges {
		if challenge.userID == user.ID {
			delete(s.challenges, key)
		}
	}
	s.challenges[digest] = &challengeEntry{
		userID:   user.ID,
		username: username,
		digest:   digest,
		expires:  now.Add(ChallengeTTL),
	}
	s.mu.Unlock()

	return &Challenge{Token: token, ExpiresInSeconds: int(ChallengeTTL.Seconds())}, nil
}

// upgradePasswordHash migrates a legacy Argon2id record after the password has
// been verified. A failed upgrade is not fatal: the next login retries it.
func (s *Service) upgradePasswordHash(userID, currentHash, password string) {
	if !NeedsRehash(currentHash) {
		return
	}
	upgraded, err := HashPassword(password, s.pbkdf2Iterations, s.random)
	if err != nil {
		return
	}
	_ = s.users.Update(userID, func(user *User) error {
		user.PasswordHash = upgraded
		return nil
	})
}

// CompleteTOTP consumes a challenge with a TOTP code, persists the used step
// so it cannot be replayed, and opens a session.
func (s *Service) CompleteTOTP(challengeToken, code string) (*Session, error) {
	userID, username, digest, now, err := s.reserveChallenge(challengeToken)
	if err != nil {
		return nil, err
	}

	user := s.users.FindByID(userID)
	if user == nil || user.Username != username {
		s.dropChallenge(digest)
		return nil, errorFor(domain.CodeChallengeExpired)
	}

	step, valid := ValidateTOTP(user.TOTPSecret, code, now)
	if !valid {
		s.failChallenge(digest, username, now)
		return nil, errorFor(domain.CodeInvalidCode)
	}

	replayed := false
	updateErr := s.users.Update(userID, func(current *User) error {
		if current.Username != username {
			return errNoOwner
		}
		if current.LastTOTPStep >= step {
			replayed = true
			return errReplay
		}
		current.LastTOTPStep = step
		current.UpdatedAt = now
		return nil
	})
	if updateErr != nil {
		s.failChallenge(digest, username, now)
		if replayed {
			return nil, errorFor(domain.CodeInvalidCode)
		}
		if errors.Is(updateErr, ErrUserNotFound) || errors.Is(updateErr, errNoOwner) {
			// The account vanished or changed after the challenge was issued.
			return nil, errorFor(domain.CodeChallengeExpired)
		}
		// Persistence failure fails the login closed.
		return nil, updateErr
	}

	s.succeedChallenge(digest, username)
	return s.createSession(*user)
}

// CompleteRecovery consumes a single-use recovery code in place of TOTP.
func (s *Service) CompleteRecovery(challengeToken, code string) (*Session, error) {
	hashed, ok := hashPresentedRecoveryCode(code)
	if !ok {
		// A malformed shape is a client error, not a guess at the secret: do not
		// consume one of the five challenge attempts.
		return nil, errorFor(domain.CodeInvalidRecoveryCode)
	}

	userID, username, digest, now, err := s.reserveChallenge(challengeToken)
	if err != nil {
		return nil, err
	}

	updateErr := s.users.Update(userID, func(current *User) error {
		if current.Username != username {
			return errNoOwner
		}
		index := -1
		// Scan the whole slice: stopping at the first match leaks how many codes
		// preceded it through timing.
		for position, stored := range current.RecoveryCodeHashes {
			if constantTimeEqual(stored, hashed) && index < 0 {
				index = position
			}
		}
		if index < 0 {
			return errInvalidRecovery
		}
		current.RecoveryCodeHashes = append(current.RecoveryCodeHashes[:index:index], current.RecoveryCodeHashes[index+1:]...)
		current.UpdatedAt = now
		return nil
	})
	if updateErr != nil {
		s.failChallenge(digest, username, now)
		if errors.Is(updateErr, errInvalidRecovery) {
			return nil, errorFor(domain.CodeInvalidRecoveryCode)
		}
		if errors.Is(updateErr, ErrUserNotFound) || errors.Is(updateErr, errNoOwner) {
			// The account vanished or changed after the challenge was issued.
			return nil, errorFor(domain.CodeChallengeExpired)
		}
		return nil, updateErr
	}

	s.succeedChallenge(digest, username)
	user := s.users.FindByID(userID)
	if user == nil {
		return nil, errorFor(domain.CodeChallengeExpired)
	}
	return s.createSession(*user)
}

// Authenticate validates a session token and slides its idle window. It answers
// entirely from the session entry, so no store/SQLite I/O runs under the lock.
func (s *Service) Authenticate(sessionToken string) (*Account, bool) {
	if sessionToken == "" {
		return nil, false
	}
	digest := hashToken(sessionToken)
	now := s.nowUTC()

	s.mu.Lock()
	s.maybePruneLocked(now)
	entry, ok := s.sessions[digest]
	if !ok || !constantTimeEqual(entry.digest, digest) {
		s.mu.Unlock()
		return nil, false
	}
	if now.Sub(entry.lastSeen) > SessionIdleTTL || now.Sub(entry.created) > SessionAbsoluteTTL {
		delete(s.sessions, digest)
		s.mu.Unlock()
		return nil, false
	}
	entry.lastSeen = now
	account := &Account{ID: entry.userID, Username: entry.username, TOTPEnabled: entry.totpEnabled}
	s.mu.Unlock()
	return account, true
}

// Logout revokes the presented session token. It is idempotent.
func (s *Service) Logout(sessionToken string) {
	if sessionToken == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, hashToken(sessionToken))
	s.mu.Unlock()
}

func (s *Service) createSession(user User) (*Session, error) {
	token, digest, err := newToken(s.random)
	if err != nil {
		return nil, err
	}
	now := s.nowUTC()
	s.mu.Lock()
	s.maybePruneLocked(now)
	s.sessions[digest] = &sessionEntry{
		userID:      user.ID,
		username:    user.Username,
		totpEnabled: user.TOTPEnabled,
		digest:      digest,
		created:     now,
		lastSeen:    now,
	}
	s.mu.Unlock()
	return &Session{
		Token:            token,
		User:             Account{ID: user.ID, Username: user.Username, TOTPEnabled: user.TOTPEnabled},
		ExpiresInSeconds: int(SessionAbsoluteTTL.Seconds()),
	}, nil
}

// reserveChallenge validates the challenge cookie, applies the per-username
// rate limit and reserves one of its five attempts before verification runs.
func (s *Service) reserveChallenge(challengeToken string) (userID, username, digest string, now time.Time, err error) {
	if challengeToken == "" {
		return "", "", "", time.Time{}, errorFor(domain.CodeChallengeExpired)
	}
	digest = hashToken(challengeToken)
	now = s.nowUTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.maybePruneLocked(now)
	challenge, ok := s.challenges[digest]
	if !ok || !constantTimeEqual(challenge.digest, digest) || !now.Before(challenge.expires) {
		if ok {
			delete(s.challenges, digest)
		}
		return "", "", "", time.Time{}, errorFor(domain.CodeChallengeExpired)
	}
	if retry, limited := s.rateLimitLocked(challenge.username, now); limited {
		return "", "", "", time.Time{}, domain.NewRateLimited(retry)
	}
	if challenge.attempts >= ChallengeMaxAttempts {
		delete(s.challenges, digest)
		return "", "", "", time.Time{}, errorFor(domain.CodeChallengeExpired)
	}
	challenge.attempts++
	return challenge.userID, challenge.username, digest, now, nil
}

func (s *Service) dropChallenge(digest string) {
	s.mu.Lock()
	delete(s.challenges, digest)
	s.mu.Unlock()
}

func (s *Service) failChallenge(digest, username string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if challenge, ok := s.challenges[digest]; ok && constantTimeEqual(challenge.digest, digest) {
		if challenge.attempts >= ChallengeMaxAttempts {
			delete(s.challenges, digest)
		}
	}
	s.recordFailureLocked(username, now)
}

func (s *Service) succeedChallenge(digest, username string) {
	s.mu.Lock()
	delete(s.challenges, digest)
	delete(s.attempts, username)
	s.mu.Unlock()
}

func (s *Service) clearFailures(username string) {
	s.mu.Lock()
	delete(s.attempts, username)
	s.mu.Unlock()
}

// recordFailureLocked reserves a failure slot and blocks the username once the
// window is full. Callers hold s.mu so the read-modify-write is atomic.
func (s *Service) recordFailureLocked(username string, now time.Time) {
	entry := s.attempts[username]
	if entry == nil {
		entry = &attemptEntry{}
		s.attempts[username] = entry
	}
	entry.failures = append(pruneFailures(entry.failures, now), now)
	if len(entry.failures) >= RateFailures {
		entry.blockedUntil = now.Add(RateBlock)
	}
}

// releaseFailureLocked removes the reservation made for one attempt. Callers
// hold s.mu. It is used when the attempt never reached verification.
func (s *Service) releaseFailureLocked(username string, reservedAt time.Time) {
	entry := s.attempts[username]
	if entry == nil {
		return
	}
	for i := len(entry.failures) - 1; i >= 0; i -= 1 {
		if entry.failures[i].Equal(reservedAt) {
			entry.failures = append(entry.failures[:i], entry.failures[i+1:]...)
			break
		}
	}
	if len(entry.failures) < RateFailures {
		entry.blockedUntil = time.Time{}
	}
	if len(entry.failures) == 0 {
		delete(s.attempts, username)
	}
}

// releasePublicLocked removes one global slot reserved for an attempt that never
// reached verification. Callers hold s.mu.
func (s *Service) releasePublicLocked(reservedAt time.Time) {
	for i := len(s.publicCalls) - 1; i >= 0; i -= 1 {
		if s.publicCalls[i].Equal(reservedAt) {
			s.publicCalls = append(s.publicCalls[:i], s.publicCalls[i+1:]...)
			return
		}
	}
}

// allowPublicLocked charges one global unauthenticated password slot.
func (s *Service) allowPublicLocked(now time.Time) (time.Duration, bool) {
	cutoff := now.Add(-GlobalAuthWindow)
	kept := s.publicCalls[:0]
	for _, call := range s.publicCalls {
		if call.After(cutoff) {
			kept = append(kept, call)
		}
	}
	s.publicCalls = kept
	if len(s.publicCalls) < s.publicLimit {
		s.publicCalls = append(s.publicCalls, now)
		return 0, false
	}
	retryAfter := s.publicCalls[0].Add(GlobalAuthWindow).Sub(now)
	if retryAfter <= 0 {
		retryAfter = time.Second
	}
	return retryAfter, true
}

// rateLimitLocked reports how long the username is blocked. When a 60-second
// block elapses the failure window resets so the account can try again.
func (s *Service) rateLimitLocked(username string, now time.Time) (time.Duration, bool) {
	entry := s.attempts[username]
	if entry == nil {
		return 0, false
	}
	entry.failures = pruneFailures(entry.failures, now)
	if now.Before(entry.blockedUntil) {
		return entry.blockedUntil.Sub(now), true
	}
	if !entry.blockedUntil.IsZero() {
		// The 60-second block elapsed but the 15-minute failure window remains.
		// Keep the timestamps so another failure re-blocks instead of resetting
		// the budget to a fresh five-per-minute allowance.
		entry.blockedUntil = time.Time{}
	}
	if len(entry.failures) == 0 {
		delete(s.attempts, username)
	}
	return 0, false
}

func pruneFailures(failures []time.Time, now time.Time) []time.Time {
	kept := failures[:0]
	for _, at := range failures {
		if now.Sub(at) < RateWindow {
			kept = append(kept, at)
		}
	}
	return kept
}

func (s *Service) maybePruneLocked(now time.Time) {
	s.ops++
	if s.ops < pruneInterval {
		return
	}
	s.ops = 0
	s.pruneLocked(now)
}

// pruneLocked drops expired entries. It is amortised (see maybePruneLocked) and
// runs at most once per pruneInterval lock acquisitions, so the request path is
// not O(state).
func (s *Service) pruneLocked(now time.Time) {
	for key, entry := range s.sessions {
		if now.Sub(entry.lastSeen) > SessionIdleTTL || now.Sub(entry.created) > SessionAbsoluteTTL {
			delete(s.sessions, key)
		}
	}
	for key, entry := range s.challenges {
		if !now.Before(entry.expires) {
			delete(s.challenges, key)
		}
	}
	for key, entry := range s.enrollments {
		if !now.Before(entry.expires) {
			delete(s.enrollments, key)
		}
	}
	for key, entry := range s.attempts {
		entry.failures = pruneFailures(entry.failures, now)
		if len(entry.failures) == 0 && !now.Before(entry.blockedUntil) {
			delete(s.attempts, key)
		}
	}
	cutoff := now.Add(-GlobalAuthWindow)
	kept := s.publicCalls[:0]
	for _, call := range s.publicCalls {
		if call.After(cutoff) {
			kept = append(kept, call)
		}
	}
	s.publicCalls = kept
}

func tryAcquire(slot chan struct{}) bool {
	select {
	case slot <- struct{}{}:
		return true
	default:
		return false
	}
}

func release(slot chan struct{}) {
	<-slot
}

func constantTimeEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func errorFor(code domain.Code) *domain.Error {
	return domain.NewError(code, domain.DefaultMessage(code))
}

const userIDBytes = 12

func newUserID(random io.Reader) (string, error) {
	raw := make([]byte, userIDBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", fmt.Errorf("auth: read user id entropy: %w", err)
	}
	return "usr_" + hex.EncodeToString(raw), nil
}
