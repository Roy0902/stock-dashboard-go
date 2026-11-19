// Package auth implements accounts: password hashing, TOTP, recovery codes,
// sessions and the in-memory state around them. Every account has its own
// credentials, second factor and recovery codes.
//
// The package is a pure service: it has no net/http, cookie or logging
// knowledge. The HTTP layer owns cookies, status mapping and auth event logs.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// DefaultPBKDF2Iterations is the production work factor for new hashes.
	// PBKDF2 is CPU-hard and uses only kilobytes of memory, so concurrent logins
	// cannot exhaust the process heap.
	DefaultPBKDF2Iterations uint32 = 600_000

	pbkdf2SaltLength = 16
	pbkdf2KeyLength  = 32
	pbkdf2Algorithm  = "pbkdf2-sha256"
)

// Argon2Params are the legacy Argon2id parameters. They are kept only to verify
// hashes created before the 2026-11-19 switch to PBKDF2; new hashes never use them.
type Argon2Params struct {
	Memory     uint32
	Time       uint32
	Threads    uint8
	SaltLength uint32
	KeyLength  uint32
}

// DefaultArgon2Params matches the legacy contract record.
var DefaultArgon2Params = Argon2Params{
	Memory:     64 * 1024,
	Time:       3,
	Threads:    4,
	SaltLength: 16,
	KeyLength:  32,
}

// withDefaults replaces zero fields with the legacy production values.
func (p Argon2Params) withDefaults() Argon2Params {
	if p.Memory == 0 {
		p.Memory = DefaultArgon2Params.Memory
	}
	if p.Time == 0 {
		p.Time = DefaultArgon2Params.Time
	}
	if p.Threads == 0 {
		p.Threads = DefaultArgon2Params.Threads
	}
	if p.SaltLength == 0 {
		p.SaltLength = DefaultArgon2Params.SaltLength
	}
	if p.KeyLength == 0 {
		p.KeyLength = DefaultArgon2Params.KeyLength
	}
	return p
}

var errMalformedHash = errors.New("auth: malformed password hash")

// HashPassword derives a PBKDF2-HMAC-SHA256 PHC string with a fresh random salt.
func HashPassword(password string, iterations uint32, random io.Reader) (string, error) {
	if iterations == 0 {
		iterations = DefaultPBKDF2Iterations
	}
	if random == nil {
		random = rand.Reader
	}
	salt := make([]byte, pbkdf2SaltLength)
	if _, err := io.ReadFull(random, salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}
	key := pbkdf2.Key([]byte(password), salt, int(iterations), pbkdf2KeyLength, sha256.New)
	encoding := base64.RawStdEncoding
	return fmt.Sprintf("$%s$i=%d$%s$%s",
		pbkdf2Algorithm,
		iterations,
		encoding.EncodeToString(salt),
		encoding.EncodeToString(key),
	), nil
}

// HashPasswordArgon2 derives a legacy Argon2id PHC string. It is used for
// verification/seed fixtures only; production hashing goes through HashPassword.
func HashPasswordArgon2(password string, params Argon2Params, random io.Reader) (string, error) {
	params = params.withDefaults()
	if random == nil {
		random = rand.Reader
	}
	salt := make([]byte, params.SaltLength)
	if _, err := io.ReadFull(random, salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, params.KeyLength)
	encoding := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Time,
		params.Threads,
		encoding.EncodeToString(salt),
		encoding.EncodeToString(key),
	), nil
}

// NeedsRehash reports whether an account should be upgraded to the current
// PBKDF2 format after a successful login. Legacy Argon2id hashes return true.
func NeedsRehash(encoded string) bool {
	return !strings.HasPrefix(encoded, "$"+pbkdf2Algorithm+"$")
}

// VerifyPassword reports whether password matches either supported PHC string.
// A malformed hash is an error; a mismatch is (false, nil). All comparisons are
// constant time.
func VerifyPassword(encoded, password string) (bool, error) {
	switch {
	case strings.HasPrefix(encoded, "$argon2id$"):
		return verifyArgon2(encoded, password)
	case strings.HasPrefix(encoded, "$"+pbkdf2Algorithm+"$"):
		return verifyPBKDF2(encoded, password)
	default:
		return false, errMalformedHash
	}
}

func verifyArgon2(encoded, password string) (bool, error) {
	parsed, err := parseArgon2Hash(encoded)
	if err != nil {
		return false, err
	}
	key := argon2.IDKey([]byte(password), parsed.salt, parsed.time, parsed.memory, parsed.threads, uint32(len(parsed.key)))
	return subtle.ConstantTimeCompare(key, parsed.key) == 1, nil
}

func verifyPBKDF2(encoded, password string) (bool, error) {
	parsed, err := parsePBKDF2Hash(encoded)
	if err != nil {
		return false, err
	}
	key := pbkdf2.Key([]byte(password), parsed.salt, int(parsed.iterations), len(parsed.key), sha256.New)
	return subtle.ConstantTimeCompare(key, parsed.key) == 1, nil
}

// IsPasswordHash reports whether encoded is a well-formed supported PHC string
// the service is willing to verify. It is used to validate the account record on load.
func IsPasswordHash(encoded string) bool {
	switch {
	case strings.HasPrefix(encoded, "$argon2id$"):
		_, err := parseArgon2Hash(encoded)
		return err == nil
	case strings.HasPrefix(encoded, "$"+pbkdf2Algorithm+"$"):
		_, err := parsePBKDF2Hash(encoded)
		return err == nil
	default:
		return false
	}
}

type parsedArgon2Hash struct {
	memory  uint32
	time    uint32
	threads uint8
	salt    []byte
	key     []byte
}

type parsedPBKDF2Hash struct {
	iterations uint32
	salt       []byte
	key        []byte
}

// Guard rails against a corrupted or hostile record turning verification into a
// memory or CPU bomb. They are twice the production policy, not an open ceiling.
var (
	maxHashMemory        = 2 * DefaultArgon2Params.Memory
	maxHashTime          = 2 * DefaultArgon2Params.Time
	maxHashThreads       = 2 * DefaultArgon2Params.Threads
	maxPBKDF2Iterations  = 2 * DefaultPBKDF2Iterations
	minDecodedSaltLength = 8
	minDecodedKeyLength  = 16
	maxDecodedKeyLength  = 64
)

func parseArgon2Hash(encoded string) (parsedArgon2Hash, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return parsedArgon2Hash{}, errMalformedHash
	}
	version, err := strconv.Atoi(strings.TrimPrefix(parts[2], "v="))
	if err != nil || version != argon2.Version {
		return parsedArgon2Hash{}, errMalformedHash
	}

	var memory, timeCost uint32
	var threads uint8
	seen := map[string]bool{}
	for _, field := range strings.Split(parts[3], ",") {
		key, value, ok := strings.Cut(field, "=")
		if !ok || value == "" || seen[key] {
			return parsedArgon2Hash{}, errMalformedHash
		}
		seen[key] = true
		switch key {
		case "m":
			parsed, parseErr := strconv.ParseUint(value, 10, 32)
			if parseErr != nil {
				return parsedArgon2Hash{}, errMalformedHash
			}
			memory = uint32(parsed)
		case "t":
			parsed, parseErr := strconv.ParseUint(value, 10, 32)
			if parseErr != nil {
				return parsedArgon2Hash{}, errMalformedHash
			}
			timeCost = uint32(parsed)
		case "p":
			parsed, parseErr := strconv.ParseUint(value, 10, 8)
			if parseErr != nil {
				return parsedArgon2Hash{}, errMalformedHash
			}
			threads = uint8(parsed)
		default:
			return parsedArgon2Hash{}, errMalformedHash
		}
	}
	if !seen["m"] || !seen["t"] || !seen["p"] {
		return parsedArgon2Hash{}, errMalformedHash
	}
	if memory > maxHashMemory || timeCost > maxHashTime || threads > maxHashThreads || memory == 0 || timeCost == 0 || threads == 0 {
		return parsedArgon2Hash{}, errMalformedHash
	}

	salt, key, err := decodeSaltAndKey(parts[4], parts[5])
	if err != nil {
		return parsedArgon2Hash{}, err
	}
	return parsedArgon2Hash{memory: memory, time: timeCost, threads: threads, salt: salt, key: key}, nil
}

func parsePBKDF2Hash(encoded string) (parsedPBKDF2Hash, error) {
	parts := strings.Split(encoded, "$")
	// "", "pbkdf2-sha256", "i=600000", salt, key
	if len(parts) != 5 || parts[0] != "" || parts[1] != pbkdf2Algorithm {
		return parsedPBKDF2Hash{}, errMalformedHash
	}
	name, value, ok := strings.Cut(parts[2], "=")
	if !ok || name != "i" || value == "" {
		return parsedPBKDF2Hash{}, errMalformedHash
	}
	iterations, err := strconv.ParseUint(value, 10, 32)
	if err != nil || iterations == 0 || iterations > uint64(maxPBKDF2Iterations) {
		return parsedPBKDF2Hash{}, errMalformedHash
	}
	salt, key, err := decodeSaltAndKey(parts[3], parts[4])
	if err != nil {
		return parsedPBKDF2Hash{}, err
	}
	return parsedPBKDF2Hash{iterations: uint32(iterations), salt: salt, key: key}, nil
}

func decodeSaltAndKey(encodedSalt, encodedKey string) ([]byte, []byte, error) {
	encoding := base64.RawStdEncoding
	salt, err := encoding.DecodeString(strings.TrimRight(encodedSalt, "="))
	if err != nil || len(salt) < minDecodedSaltLength {
		return nil, nil, errMalformedHash
	}
	key, err := encoding.DecodeString(strings.TrimRight(encodedKey, "="))
	if err != nil || len(key) < minDecodedKeyLength || len(key) > maxDecodedKeyLength {
		return nil, nil, errMalformedHash
	}
	return salt, key, nil
}
