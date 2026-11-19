package auth

import (
	"bytes"
	"testing"
)

func TestNewTokenUsesInjectedReader(t *testing.T) {
	first := bytes.Repeat([]byte{0x01}, tokenBytes)
	second := bytes.Repeat([]byte{0x02}, tokenBytes)

	firstToken, firstDigest, err := newToken(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	secondToken, secondDigest, err := newToken(bytes.NewReader(second))
	if err != nil {
		t.Fatal(err)
	}
	if firstToken == secondToken || firstDigest == secondDigest {
		t.Fatal("injected entropy was not used")
	}
	if firstDigest != hashToken(firstToken) || secondDigest != hashToken(secondToken) {
		t.Fatal("digest does not match its token")
	}
}
