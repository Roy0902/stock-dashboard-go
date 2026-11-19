package auth

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Username and password rules from docs/contracts-auth.md section 3.
const (
	MinUsernameLength = 3
	MaxUsernameLength = 32
	MinPasswordRunes  = 12
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// NormalizeUsername trims surrounding whitespace. Case is not folded: the
// frozen acceptance plan requires uppercase input such as "Owner" to fail
// validation with INVALID_USERNAME.
func NormalizeUsername(raw string) string {
	return strings.TrimSpace(raw)
}

// ValidUsername reports whether username matches the contract pattern.
func ValidUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

// ValidPassword reports whether password is at least the minimum length. There is
// no user-facing maximum; the 4 KiB request body cap bounds the input.
func ValidPassword(password string) bool {
	return utf8.RuneCountInString(password) >= MinPasswordRunes
}
