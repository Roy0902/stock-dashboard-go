package domain

import (
	"fmt"
	"testing"
)

func tickersOf(watchlist Watchlist) []string {
	out := make([]string, 0, watchlist.Len())
	for _, ticker := range watchlist.Tickers() {
		out = append(out, ticker.String())
	}
	return out
}

func TestWatchlistAddKeepsInsertionOrder(t *testing.T) {
	watchlist := NewWatchlist()
	watchlist, err := watchlist.Add("aapl")
	if err != nil {
		t.Fatal(err)
	}
	watchlist, err = watchlist.Add("MSFT")
	if err != nil {
		t.Fatal(err)
	}

	got := tickersOf(watchlist)
	if len(got) != 2 || got[0] != "AAPL" || got[1] != "MSFT" {
		t.Fatalf("order = %v, want [AAPL MSFT]", got)
	}
}

func TestWatchlistAddIsPure(t *testing.T) {
	original := NewWatchlist(MustTicker("AAPL"))
	next, err := original.Add("MSFT")
	if err != nil {
		t.Fatal(err)
	}
	if original.Len() != 1 {
		t.Fatalf("receiver mutated: length = %d", original.Len())
	}
	if next.Len() != 2 {
		t.Fatalf("result length = %d", next.Len())
	}
}

func TestWatchlistAddDuplicate(t *testing.T) {
	watchlist := NewWatchlist(MustTicker("AAPL"))
	_, err := watchlist.Add("aapl")
	if err == nil {
		t.Fatal("duplicate add succeeded")
	}
	if !IsCode(err, CodeDuplicateTicker) {
		t.Fatalf("code = %v, want DUPLICATE_TICKER", err)
	}
	if err.Error() != "DUPLICATE_TICKER: AAPL is already in your watchlist." {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestWatchlistAddInvalid(t *testing.T) {
	watchlist := NewWatchlist()
	for _, raw := range []string{"", "1AAPL", "AA-PL", "A1234567890"} {
		if _, err := watchlist.Add(raw); !IsCode(err, CodeInvalidTicker) {
			t.Fatalf("Add(%q) error = %v, want INVALID_TICKER", raw, err)
		}
	}
}

func TestWatchlistRemove(t *testing.T) {
	watchlist := NewWatchlist(MustTicker("AAPL"), MustTicker("MSFT"), MustTicker("TSLA"))
	next, err := watchlist.Remove("msft")
	if err != nil {
		t.Fatal(err)
	}
	got := tickersOf(next)
	if len(got) != 2 || got[0] != "AAPL" || got[1] != "TSLA" {
		t.Fatalf("remaining = %v", got)
	}
	if watchlist.Len() != 3 {
		t.Fatal("Remove mutated its receiver")
	}
}

func TestWatchlistRemoveMissing(t *testing.T) {
	watchlist := NewWatchlist(MustTicker("MSFT"))
	_, err := watchlist.Remove("AAPL")
	if !IsCode(err, CodeNotInWatchlist) {
		t.Fatalf("error = %v, want NOT_IN_WATCHLIST", err)
	}
	if err.Error() != "NOT_IN_WATCHLIST: AAPL is not in your watchlist." {
		t.Fatalf("message = %q", err.Error())
	}
}

func TestWatchlistRemoveIsLenientAboutForm(t *testing.T) {
	watchlist := NewWatchlist(MustTicker("AAPL"))
	_, err := watchlist.Remove("AA-PL")
	if !IsCode(err, CodeNotInWatchlist) {
		t.Fatalf("error = %v, want NOT_IN_WATCHLIST", err)
	}
}

func TestWatchlistHasNoCap(t *testing.T) {
	watchlist := NewWatchlist()
	for index := 0; index < 60; index++ {
		var err error
		watchlist, err = watchlist.Add(fmt.Sprintf("T%09d", index))
		if err != nil {
			t.Fatalf("add %d: %v", index, err)
		}
	}
	if watchlist.Len() != 60 {
		t.Fatalf("length = %d, want 60", watchlist.Len())
	}
}

func TestWatchlistTickersReturnsCopy(t *testing.T) {
	watchlist := NewWatchlist(MustTicker("AAPL"))
	copyOfTickers := watchlist.Tickers()
	copyOfTickers[0] = MustTicker("MSFT")
	if got := watchlist.Tickers()[0]; got != "AAPL" {
		t.Fatalf("Tickers exposed internal storage: %q", got)
	}
}
