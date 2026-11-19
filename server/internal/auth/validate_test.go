package auth

import (
	"strings"
	"testing"
)

func TestValidUsername(t *testing.T) {
	valid := []string{"abc", "owner", "a.b-c_d", "user123", "a1234567890123456789012345678901"}
	for _, username := range valid {
		if !ValidUsername(username) {
			t.Fatalf("%q rejected", username)
		}
	}
	invalid := []string{
		"", "ab", "_owner", "-owner", ".owner", "Owner", "OWNER", "owner!", "own er",
		"owner@example.com", "a12345678901234567890123456789012", "üser",
	}
	for _, username := range invalid {
		if ValidUsername(username) {
			t.Fatalf("%q accepted", username)
		}
	}
}

func TestNormalizeUsernameTrimsOnly(t *testing.T) {
	if got := NormalizeUsername("  owner  "); got != "owner" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeUsername(" Owner "); got != "Owner" {
		t.Fatalf("case was folded: %q", got)
	}
}

func TestValidPassword(t *testing.T) {
	if ValidPassword(strings.Repeat("a", MinPasswordRunes-1)) {
		t.Fatal("11 characters accepted")
	}
	if !ValidPassword(strings.Repeat("a", MinPasswordRunes)) {
		t.Fatal("12 characters rejected")
	}
	if !ValidPassword(strings.Repeat("a", 512)) {
		t.Fatal("a long password was rejected")
	}
	// Length counts runes, not bytes.
	if !ValidPassword(strings.Repeat("é", MinPasswordRunes)) {
		t.Fatal("12 multi-byte runes rejected")
	}
	if ValidPassword(strings.Repeat("é", MinPasswordRunes-1)) {
		t.Fatal("11 multi-byte runes accepted")
	}
	if ValidPassword("") {
		t.Fatal("empty password accepted")
	}
}
