package auth

import (
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, hashes, err := generateRecoveryCodes(nil, RecoveryCodeCount)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != RecoveryCodeCount || len(hashes) != RecoveryCodeCount {
		t.Fatalf("codes/hashes = %d/%d", len(codes), len(hashes))
	}
	shape := regexp.MustCompile(`^[A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4}-[A-Z2-7]{4}$`)
	seen := map[string]bool{}
	for index, code := range codes {
		if !shape.MatchString(code) {
			t.Fatalf("code %q has the wrong shape", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
		hash := hashes[index]
		if len(hash) != 64 {
			t.Fatalf("hash %q is not 64 characters", hash)
		}
		if _, err := hex.DecodeString(hash); err != nil {
			t.Fatalf("hash %q is not hex", hash)
		}
		if hash != hashRecoveryCode(strings.ReplaceAll(code, "-", "")) {
			t.Fatalf("hash does not match the canonical code %q", code)
		}
	}
}

func TestNormalizeRecoveryCode(t *testing.T) {
	canonical := "abcd-efgh-ijkl-mnop"
	// Not real base32 in the first group; normalize only checks the alphabet.
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "ABCD-EFGH-IJKL-MNOP", want: "ABCDEFGHIJKLMNOP", ok: true},
		{input: "abcd-efgh-ijkl-mnop", want: "ABCDEFGHIJKLMNOP", ok: true},
		{input: "abcdefghijklmnop", want: "ABCDEFGHIJKLMNOP", ok: true},
		{input: "  ABCD EFGH IJKL MNOP  ", want: "ABCDEFGHIJKLMNOP", ok: true},
		{input: "ABCD-EFGH-IJKL", ok: false},
		{input: "ABCD-EFGH-IJKL-MNOPQ", ok: false},
		{input: "ABCD-EFGH-IJKL-MN0P", ok: false},
	}
	for _, test := range tests {
		got, ok := normalizeRecoveryCode(test.input)
		if ok != test.ok {
			t.Fatalf("%q ok = %v, want %v", test.input, ok, test.ok)
		}
		if ok && got != test.want {
			t.Fatalf("%q normalized to %q, want %q", test.input, got, test.want)
		}
	}

	hashed, ok := hashPresentedRecoveryCode(strings.ToLower(canonical))
	if !ok {
		t.Fatal("lowercase grouped code rejected")
	}
	if hashed != hashRecoveryCode("ABCDEFGHIJKLMNOP") {
		t.Fatal("presented hash does not match the canonical hash")
	}
	if _, ok := hashPresentedRecoveryCode("not-a-code"); ok {
		t.Fatal("malformed code accepted")
	}
}

func TestTokenShape(t *testing.T) {
	token, digest, err := newToken(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 {
		t.Fatalf("token length = %d", len(token))
	}
	if token == digest {
		t.Fatal("digest equals the token")
	}
	if digest != hashToken(token) {
		t.Fatal("digest is not the SHA-256 of the token")
	}
	if len(digest) != 64 {
		t.Fatalf("digest length = %d", len(digest))
	}
	if strings.ContainsAny(token, "+/=") {
		t.Fatalf("token is not base64url: %q", token)
	}
}
