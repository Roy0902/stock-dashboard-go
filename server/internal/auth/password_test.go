package auth

import (
	"strings"
	"testing"
)

const fastPBKDF2Iterations uint32 = 1_000

var fastArgon2 = Argon2Params{Memory: 8 << 10, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery", fastPBKDF2Iterations, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$pbkdf2-sha256$i=1000$") {
		t.Fatalf("hash = %q", hash)
	}
	if !IsPasswordHash(hash) {
		t.Fatal("IsPasswordHash rejected a valid hash")
	}
	if NeedsRehash(hash) {
		t.Fatal("a current PBKDF2 hash was marked for rehash")
	}

	matches, err := VerifyPassword(hash, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Fatal("correct password did not verify")
	}

	matches, err = VerifyPassword(hash, "correct horse batterY")
	if err != nil {
		t.Fatal(err)
	}
	if matches {
		t.Fatal("wrong password verified")
	}
}

func TestPasswordHashesUseFreshSalts(t *testing.T) {
	first, err := HashPassword("correct horse battery", fastPBKDF2Iterations, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("correct horse battery", fastPBKDF2Iterations, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two hashes are identical; salt is not random")
	}
	parsed, err := parsePBKDF2Hash(first)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.salt) != 16 || len(parsed.key) != 32 {
		t.Fatalf("salt/key = %d/%d", len(parsed.salt), len(parsed.key))
	}
	if parsed.iterations != fastPBKDF2Iterations {
		t.Fatalf("iterations = %d", parsed.iterations)
	}
}

func TestLegacyArgon2HashStillVerifies(t *testing.T) {
	hash, err := HashPasswordArgon2("correct horse battery", fastArgon2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("hash = %q", hash)
	}
	if !IsPasswordHash(hash) {
		t.Fatal("IsPasswordHash rejected a valid legacy hash")
	}
	if !NeedsRehash(hash) {
		t.Fatal("a legacy Argon2 hash was not marked for rehash")
	}
	matches, err := VerifyPassword(hash, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Fatal("legacy Argon2 hash did not verify")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, encoded := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=8192,t=1,p=1$c2FsdA$a2V5",
		"$argon2id$v=18$m=8192,t=1,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=999999999,t=1,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=8192,t=1,p=1$not-base64$a2V5",
		"$argon2id$v=19$m=8192,t=1,p=1$c2FsdA$short",
		"$argon2id$v=19$garbage$c2FsdA$a2V5",
		"$pbkdf2-sha256$i=0$c2FsdA$a2V5",
		"$pbkdf2-sha256$i=999999999$c2FsdA$a2V5",
		"$pbkdf2-sha256$x=1000$c2FsdA$a2V5",
		"$pbkdf2-sha256$i=1000$not-base64$a2V5",
		"$pbkdf2-sha256$i=1000$c2FsdA$short",
		"$pbkdf2-sha512$i=1000$c2FsdA$a2V5",
	} {
		if _, err := VerifyPassword(encoded, "correct horse battery"); err == nil {
			t.Fatalf("accepted malformed hash %q", encoded)
		}
		if IsPasswordHash(encoded) {
			t.Fatalf("IsPasswordHash accepted %q", encoded)
		}
	}
}
