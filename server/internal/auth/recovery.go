package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// RecoveryCodeCount is the number of single-use codes issued at setup.
const RecoveryCodeCount = 10

// recoveryCodeBytes is 10 random bytes, which base32-encodes to 16 characters.
const recoveryCodeBytes = 10

var (
	recoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)
	recoveryAlphabet = func() [256]bool {
		var table [256]bool
		for _, symbol := range []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") {
			table[symbol] = true
		}
		return table
	}()
)

// generateRecoveryCodes returns the plaintext codes (shown once) and their
// SHA-256 hex hashes (persisted). Codes render as XXXX-XXXX-XXXX-XXXX.
func generateRecoveryCodes(random io.Reader, count int) (codes, hashes []string, err error) {
	if random == nil {
		random = rand.Reader
	}
	raw := make([]byte, recoveryCodeBytes*count)
	if _, err := io.ReadFull(random, raw); err != nil {
		return nil, nil, fmt.Errorf("auth: read recovery entropy: %w", err)
	}
	codes = make([]string, 0, count)
	hashes = make([]string, 0, count)
	for index := 0; index < count; index++ {
		encoded := recoveryEncoding.EncodeToString(raw[index*recoveryCodeBytes : (index+1)*recoveryCodeBytes])
		codes = append(codes, groupRecoveryCode(encoded))
		hashes = append(hashes, hashRecoveryCode(encoded))
	}
	return codes, hashes, nil
}

func groupRecoveryCode(encoded string) string {
	groups := make([]string, 0, len(encoded)/4)
	for index := 0; index < len(encoded); index += 4 {
		end := index + 4
		if end > len(encoded) {
			end = len(encoded)
		}
		groups = append(groups, encoded[index:end])
	}
	return strings.Join(groups, "-")
}

// normalizeRecoveryCode upper-cases the input and removes grouping characters.
// The result must be 16 base32 symbols, otherwise ok is false.
func normalizeRecoveryCode(raw string) (normalized string, ok bool) {
	normalized = strings.ToUpper(strings.TrimSpace(raw))
	normalized = strings.NewReplacer("-", "", " ", "", "\t", "").Replace(normalized)
	if len(normalized) != recoveryCodeBytes*8/5 {
		return "", false
	}
	for index := 0; index < len(normalized); index++ {
		if !recoveryAlphabet[normalized[index]] {
			return "", false
		}
	}
	return normalized, true
}

// hashRecoveryCode hashes the canonical (grouping-free, uppercase) form.
func hashRecoveryCode(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// hashPresentedRecoveryCode normalises a submitted code and hashes it. The
// boolean reports whether the input had a valid shape.
func hashPresentedRecoveryCode(raw string) (string, bool) {
	normalized, ok := normalizeRecoveryCode(raw)
	if !ok {
		return "", false
	}
	return hashRecoveryCode(normalized), true
}
