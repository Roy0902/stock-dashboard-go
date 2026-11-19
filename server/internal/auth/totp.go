package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTP parameters from docs/contracts-auth.md section 3.
const (
	TOTPIssuer = "Stock Dashboard"
	TOTPPeriod = 30 * time.Second
	TOTPDigits = 6
	TOTPSkew   = 1
)

// totpSecretBytes is 20 bytes of entropy (160 bits), base32 encoded.
const totpSecretBytes = 20

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

var totpOptions = totp.ValidateOpts{
	Period:    uint(TOTPPeriod / time.Second),
	Skew:      TOTPSkew,
	Digits:    otp.DigitsSix,
	Algorithm: otp.AlgorithmSHA1,
}

// GenerateTOTPSecret returns a fresh base32 (uppercase, unpadded) secret.
func GenerateTOTPSecret(random io.Reader) (string, error) {
	if random == nil {
		random = rand.Reader
	}
	raw := make([]byte, totpSecretBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", fmt.Errorf("auth: read totp entropy: %w", err)
	}
	return totpEncoding.EncodeToString(raw), nil
}

// ValidTOTPSecret reports whether secret decodes as base32.
func ValidTOTPSecret(secret string) bool {
	raw, err := totpEncoding.DecodeString(secret)
	return err == nil && len(raw) >= 10
}

// OTPAuthURL builds the otpauth://totp URI consumed by authenticator apps and
// the client-side QR renderer.
func OTPAuthURL(username, secret string) string {
	label := url.PathEscape(TOTPIssuer) + ":" + url.PathEscape(username)
	issuer := strings.ReplaceAll(url.QueryEscape(TOTPIssuer), "+", "%20")
	return "otpauth://totp/" + label +
		"?secret=" + secret +
		"&issuer=" + issuer +
		"&algorithm=SHA1&digits=6&period=" + fmt.Sprint(int(TOTPPeriod/time.Second))
}

// ValidateTOTP checks a six-digit code against now, accepting +/- one time
// step. It returns the matched step so the caller can persist replay state.
func ValidateTOTP(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != TOTPDigits || !allDigits(code) {
		return 0, false
	}
	currentStep := now.UTC().Unix() / int64(TOTPPeriod/time.Second)
	for _, delta := range []int64{0, -1, 1} {
		step := currentStep + delta
		at := time.Unix(step*int64(TOTPPeriod/time.Second), 0).UTC()
		expected, err := totp.GenerateCodeCustom(secret, at, totpOptions)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}

func allDigits(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
