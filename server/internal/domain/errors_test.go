package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewRateLimitedRoundsUpToWholeSeconds(t *testing.T) {
	for _, test := range []struct {
		after   time.Duration
		seconds int
		want    string
	}{
		{after: 200 * time.Millisecond, seconds: 1, want: "Too many requests. Try again in 1 second."},
		{after: time.Second, seconds: 1, want: "Too many requests. Try again in 1 second."},
		{after: 5 * time.Second, seconds: 5, want: "Too many requests. Try again in 5 seconds."},
		{after: 5200 * time.Millisecond, seconds: 6, want: "Too many requests. Try again in 6 seconds."},
	} {
		t.Run(fmt.Sprint(test.after), func(t *testing.T) {
			failure := NewRateLimited(test.after)
			if failure.Code != CodeRateLimited {
				t.Fatalf("code = %q", failure.Code)
			}
			if failure.Message != test.want {
				t.Fatalf("message = %q, want %q", failure.Message, test.want)
			}
			if failure.RetryAfter != time.Duration(test.seconds)*time.Second {
				t.Fatalf("retry after = %s", failure.RetryAfter)
			}
		})
	}
}

func TestDefaultRateLimitedMessageIsGrammatical(t *testing.T) {
	if got := DefaultMessage(CodeRateLimited); got != "Too many requests. Try again in 1 second." {
		t.Fatalf("message = %q", got)
	}
	for _, seconds := range []int{1, 2, 60} {
		if got := TooManyRequests(seconds); strings.Contains(got, "1 seconds") {
			t.Fatalf("TooManyRequests(%d) = %q", seconds, got)
		}
	}
}

func TestIsCodeSeesWrappedErrors(t *testing.T) {
	root := NewError(CodeUnknownSymbol, "Symbol not found.")
	wrapped := fmt.Errorf("lookup: %w", root)
	if !IsCode(wrapped, CodeUnknownSymbol) {
		t.Fatal("wrapped domain error was not recognised")
	}
	if IsCode(wrapped, CodeInvalidTicker) {
		t.Fatal("matched the wrong code")
	}
	if IsCode(errors.New("plain"), CodeUnknownSymbol) {
		t.Fatal("matched a non-domain error")
	}
}
