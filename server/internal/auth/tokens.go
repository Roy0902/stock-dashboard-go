package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// tokenBytes is the raw entropy for session, challenge and enrollment tokens.
const tokenBytes = 32

// newToken returns a base64url token (43 characters for 32 bytes) and the
// SHA-256 hex digest used as the in-memory map key. The token itself is never
// stored or logged; only the digest lives server-side.
func newToken(random io.Reader) (token, digest string, err error) {
	if random == nil {
		random = rand.Reader
	}
	raw := make([]byte, tokenBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", "", fmt.Errorf("auth: read token entropy: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

// hashToken is the one-way digest stored for a presented token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
