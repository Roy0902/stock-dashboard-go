package domain

// Watchlist is an insertion-ordered, duplicate-free list of canonical tickers.
// Its methods are pure: every transition returns a new value and the receiver is
// never mutated. There is no maximum size.
type Watchlist struct {
	tickers []Ticker
}

// NewWatchlist returns a watchlist in the given order. Duplicate tickers are not
// removed here; Add and the store use Contains to enforce uniqueness.
func NewWatchlist(tickers ...Ticker) Watchlist {
	cp := make([]Ticker, len(tickers))
	copy(cp, tickers)
	return Watchlist{tickers: cp}
}

// Add canonicalises raw and appends it. An already-listed ticker is DUPLICATE_TICKER.
func (w Watchlist) Add(raw string) (Watchlist, error) {
	ticker, err := Canonicalize(raw)
	if err != nil {
		return Watchlist{}, err
	}
	if w.Contains(ticker) {
		return Watchlist{}, NewError(CodeDuplicateTicker, DuplicateMessage(ticker))
	}
	next := make([]Ticker, len(w.tickers), len(w.tickers)+1)
	copy(next, w.tickers)
	next = append(next, ticker)
	return Watchlist{tickers: next}, nil
}

// Remove drops raw after normalisation. Removing a ticker that is not present is
// NOT_IN_WATCHLIST; raw does not have to be a valid ticker, only a non-member.
func (w Watchlist) Remove(raw string) (Watchlist, error) {
	ticker := Ticker(normalise(raw))
	index := -1
	for i, candidate := range w.tickers {
		if candidate == ticker {
			index = i
			break
		}
	}
	if index < 0 {
		return Watchlist{}, NewError(CodeNotInWatchlist, NotInWatchlistMessage(ticker))
	}
	next := make([]Ticker, 0, len(w.tickers)-1)
	next = append(next, w.tickers[:index]...)
	next = append(next, w.tickers[index+1:]...)
	return Watchlist{tickers: next}, nil
}

// Contains reports whether ticker is listed.
func (w Watchlist) Contains(ticker Ticker) bool {
	for _, candidate := range w.tickers {
		if candidate == ticker {
			return true
		}
	}
	return false
}

// Tickers returns a copy of the listed tickers in insertion order.
func (w Watchlist) Tickers() []Ticker {
	out := make([]Ticker, len(w.tickers))
	copy(out, w.tickers)
	return out
}

// Len returns the number of listed tickers.
func (w Watchlist) Len() int {
	return len(w.tickers)
}
