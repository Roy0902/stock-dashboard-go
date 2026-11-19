package domain

import (
	"errors"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Ticker
		wantErr bool
	}{
		{name: "single letter", raw: "A", want: "A"},
		{name: "four letters", raw: "AAPL", want: "AAPL"},
		{name: "ten characters", raw: "A123456789", want: "A123456789"},
		{name: "lowercase", raw: "aapl", want: "AAPL"},
		{name: "mixed case", raw: " MsFt ", want: "MSFT"},
		{name: "empty", raw: "", wantErr: true},
		{name: "leading digit", raw: "1AAPL", wantErr: true},
		{name: "hyphen", raw: "AA-PL", wantErr: true},
		{name: "dot", raw: "A.B", wantErr: true},
		{name: "internal space", raw: "AA PL", wantErr: true},
		{name: "eleven characters", raw: "A1234567890", wantErr: true},
		{name: "non-ascii", raw: "Ä", wantErr: true},
		{name: "newline trimmed", raw: "AAPL\n", want: "AAPL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Canonicalize(test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Canonicalize(%q) = %q, want error", test.raw, got)
				}
				if !IsCode(err, CodeInvalidTicker) {
					t.Fatalf("Canonicalize(%q) error code = %v, want INVALID_TICKER", test.raw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Canonicalize(%q) error = %v", test.raw, err)
			}
			if got != test.want {
				t.Fatalf("Canonicalize(%q) = %q, want %q", test.raw, got, test.want)
			}
			if !got.IsCanonical() {
				t.Fatalf("%q should report canonical", got)
			}
		})
	}
}

func TestTickerIsCanonical(t *testing.T) {
	if Ticker("aapl").IsCanonical() {
		t.Fatal("lowercase ticker reported canonical")
	}
	if Ticker("").IsCanonical() {
		t.Fatal("empty ticker reported canonical")
	}
}

func TestDomainError(t *testing.T) {
	err := NewError(CodeInvalidTicker, DefaultMessage(CodeInvalidTicker))
	if !IsCode(err, CodeInvalidTicker) {
		t.Fatal("IsCode did not recognise the wrapped code")
	}
	if IsCode(err, CodeUnknownSymbol) {
		t.Fatal("IsCode matched the wrong code")
	}
	wrapped := errors.Join(errors.New("context"), err)
	if !IsCode(wrapped, CodeInvalidTicker) {
		t.Fatal("IsCode did not unwrap")
	}
	if err.Error() == "" {
		t.Fatal("Error() should be non-empty")
	}
}

func TestCopyStrings(t *testing.T) {
	tests := map[Code]string{
		CodeInvalidTicker:       "Enter a valid ticker, e.g. AAPL.",
		CodeUnknownSymbol:       "Symbol not found.",
		CodeInvalidRange:        "Range must be 1M, 3M or 1Y.",
		CodeMissingAPIKey:       "Missing API key: set FINNHUB_API_KEY and TWELVEDATA_API_KEY.",
		CodeProviderUnavailable: "Data provider is unavailable. Try again shortly.",
		CodeChartUnavailable:    "Chart unavailable",
		CodeInternalError:       "Something went wrong. Try again shortly.",
		CodeNotFound:            "Not found.",
	}
	for code, want := range tests {
		if got := DefaultMessage(code); got != want {
			t.Fatalf("DefaultMessage(%s) = %q, want %q", code, got, want)
		}
	}
	if got := DuplicateMessage("AAPL"); got != "AAPL is already in your watchlist." {
		t.Fatalf("duplicate copy = %q", got)
	}
	if got := NotInWatchlistMessage("AAPL"); got != "AAPL is not in your watchlist." {
		t.Fatalf("not-in-watchlist copy = %q", got)
	}
	if got := TooManyRequests(7); got != "Too many requests. Try again in 7 seconds." {
		t.Fatalf("rate copy = %q", got)
	}
	if got := TooManyRequests(0); got != "Too many requests. Try again in 1 second." {
		t.Fatalf("rate copy floor = %q", got)
	}
}
