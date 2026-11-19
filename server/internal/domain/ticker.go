package domain

import (
	"fmt"
	"strings"
)

// Ticker is a canonical US ticker symbol: uppercase ASCII, one to ten characters,
// starting with a letter.
type Ticker string

func (t Ticker) String() string {
	return string(t)
}

// IsCanonical reports whether t already satisfies the canonical ticker grammar.
func (t Ticker) IsCanonical() bool {
	return isCanonical(string(t))
}

// Canonicalize trims, uppercases ASCII letters only, and validates raw.
//
// ASCII-only uppercasing keeps non-ASCII input (for example "Ä") invalid instead of
// letting a locale-aware fold turn it into something acceptable.
func Canonicalize(raw string) (Ticker, error) {
	t := Ticker(normalise(raw))
	if !t.IsCanonical() {
		return "", NewError(CodeInvalidTicker, DefaultMessage(CodeInvalidTicker))
	}
	return t, nil
}

// MustTicker is for constants and tests; it panics on invalid input.
func MustTicker(raw string) Ticker {
	t, err := Canonicalize(raw)
	if err != nil {
		panic(fmt.Sprintf("domain: invalid ticker %q", raw))
	}
	return t
}

// normalise trims and uppercases ASCII letters without validating.
func normalise(raw string) string {
	raw = strings.TrimSpace(raw)
	out := []byte(raw)
	for i, b := range out {
		if b >= 'a' && b <= 'z' {
			out[i] = b - ('a' - 'A')
		}
	}
	return string(out)
}

func isCanonical(raw string) bool {
	if len(raw) == 0 || len(raw) > 10 {
		return false
	}
	if raw[0] < 'A' || raw[0] > 'Z' {
		return false
	}
	for i := 1; i < len(raw); i++ {
		b := raw[i]
		if (b < 'A' || b > 'Z') && (b < '0' || b > '9') {
			return false
		}
	}
	return true
}
