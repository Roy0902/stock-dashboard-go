package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateTOTPSecretShape(t *testing.T) {
	secret, err := GenerateTOTPSecret(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 32 {
		t.Fatalf("secret length = %d", len(secret))
	}
	if secret != strings.ToUpper(secret) {
		t.Fatalf("secret is not uppercase base32: %q", secret)
	}
	if !ValidTOTPSecret(secret) {
		t.Fatal("generated secret is invalid")
	}
	other, err := GenerateTOTPSecret(nil)
	if err != nil {
		t.Fatal(err)
	}
	if other == secret {
		t.Fatal("two secrets are identical")
	}
}

func TestOTPAuthURL(t *testing.T) {
	url := OTPAuthURL("owner", "JBSWY3DPEHPK3PXP")
	want := "otpauth://totp/Stock%20Dashboard:owner?secret=JBSWY3DPEHPK3PXP" +
		"&issuer=Stock%20Dashboard&algorithm=SHA1&digits=6&period=30"
	if url != want {
		t.Fatalf("url = %q\nwant %q", url, want)
	}
}

func TestValidateTOTPWindow(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)
	step := now.Unix() / 30

	current, err := totp.GenerateCode(secret, now)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := totp.GenerateCode(secret, time.Unix((step-1)*30, 0))
	if err != nil {
		t.Fatal(err)
	}
	next, err := totp.GenerateCode(secret, time.Unix((step+1)*30, 0))
	if err != nil {
		t.Fatal(err)
	}
	stale, err := totp.GenerateCode(secret, time.Unix((step-2)*30, 0))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		code string
		want int64
		ok   bool
	}{
		{name: "current", code: current, want: step, ok: true},
		{name: "previous step", code: previous, want: step - 1, ok: true},
		{name: "next step", code: next, want: step + 1, ok: true},
		{name: "two steps back", code: stale, ok: false},
		{name: "empty", code: "", ok: false},
		{name: "five digits", code: current[:5], ok: false},
		{name: "letters", code: "abcdef", ok: false},
		{name: "padded", code: " " + current + " ", want: step, ok: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ValidateTOTP(secret, test.code, now)
			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}
			if ok && got != test.want {
				t.Fatalf("step = %d, want %d", got, test.want)
			}
		})
	}
}

func TestValidateTOTPRejectsBadSecret(t *testing.T) {
	if _, ok := ValidateTOTP("not base32!", "123456", time.Now()); ok {
		t.Fatal("accepted a code for a malformed secret")
	}
}
