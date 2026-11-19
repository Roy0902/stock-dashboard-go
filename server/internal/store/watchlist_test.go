package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"stock-dashboard/server/internal/domain"
)

func TestWatchlistRoundTripAndOrdering(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")

	addTickers(t, storage, owner.ID, "aapl", "MSFT", "goog")
	if got := listStrings(storage.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL", "MSFT", "GOOG"}) {
		t.Fatalf("tickers = %v", got)
	}

	next, err := storage.Watchlist.Update(owner.ID, func(current domain.Watchlist) (domain.Watchlist, error) {
		return current.Remove("msft")
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := listStrings(next); !equalStrings(got, []string{"AAPL", "GOOG"}) {
		t.Fatalf("removed list = %v", got)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL", "GOOG"}) {
		t.Fatalf("reloaded = %v", got)
	}
}

func TestWatchlistUpdateReplacesList(t *testing.T) {
	storage := mustOpen(t, t.TempDir())
	defer mustClose(t, storage)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")

	addTickers(t, storage, owner.ID, "AAPL", "MSFT")
	next, err := storage.Watchlist.Update(owner.ID, func(domain.Watchlist) (domain.Watchlist, error) {
		return domain.NewWatchlist(domain.Ticker("TSLA")), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := listStrings(next); !equalStrings(got, []string{"TSLA"}) {
		t.Fatalf("replaced = %v", got)
	}
	if got := listStrings(storage.Watchlist.List(owner.ID)); !equalStrings(got, []string{"TSLA"}) {
		t.Fatalf("stored = %v", got)
	}
}

func TestWatchlistUpdateErrorRollsBack(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	addTickers(t, storage, owner.ID, "AAPL")

	sentinel := errors.New("refused")
	if _, err := storage.Watchlist.Update(owner.ID, func(current domain.Watchlist) (domain.Watchlist, error) {
		updated, addErr := current.Add("MSFT")
		if addErr != nil {
			return domain.Watchlist{}, addErr
		}
		return updated, sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	if got := listStrings(storage.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("memory changed on error: %v", got)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("database changed on error: %v", got)
	}
}

// TestWatchlistConstraintRollsBack proves a bad replacement is rejected by the
// database, not just by the callback: duplicates hit the UNIQUE index and the
// delete+insert transaction reverts.
func TestWatchlistConstraintRollsBack(t *testing.T) {
	dir := t.TempDir()
	storage := mustOpen(t, dir)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	addTickers(t, storage, owner.ID, "AAPL", "MSFT")

	if _, err := storage.Watchlist.Update(owner.ID, func(domain.Watchlist) (domain.Watchlist, error) {
		return domain.NewWatchlist(domain.Ticker("AAPL"), domain.Ticker("AAPL")), nil
	}); err == nil {
		t.Fatal("duplicate tickers accepted")
	}
	if got := listStrings(storage.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("database changed after constraint failure: %v", got)
	}
	mustClose(t, storage)

	reopened := mustOpen(t, dir)
	defer mustClose(t, reopened)
	if got := listStrings(reopened.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("reloaded after constraint failure = %v", got)
	}
}

func TestWatchlistIsolatedPerAccount(t *testing.T) {
	storage := mustOpen(t, t.TempDir())
	defer mustClose(t, storage)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")
	second := addAccount(t, storage, "usr_ffffffffffffffff", "other")

	addTickers(t, storage, owner.ID, "AAPL")
	addTickers(t, storage, second.ID, "AAPL", "MSFT")

	if got := listStrings(storage.Watchlist.List(owner.ID)); !equalStrings(got, []string{"AAPL"}) {
		t.Fatalf("owner list = %v", got)
	}
	if got := listStrings(storage.Watchlist.List(second.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("second list = %v", got)
	}

	if _, err := storage.Watchlist.Update(owner.ID, func(current domain.Watchlist) (domain.Watchlist, error) {
		return current.Remove("AAPL")
	}); err != nil {
		t.Fatal(err)
	}
	if got := listStrings(storage.Watchlist.List(second.ID)); !equalStrings(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("second list changed by owner removal: %v", got)
	}
	if got := storage.Watchlist.List("usr_unknown").Len(); got != 0 {
		t.Fatalf("unknown account list length = %d", got)
	}
}

func TestWatchlistConcurrentUpdates(t *testing.T) {
	storage := mustOpen(t, t.TempDir())
	defer mustClose(t, storage)
	owner := addAccount(t, storage, "usr_0123456789abcdef", "owner")

	var group sync.WaitGroup
	for index := 0; index < 20; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			ticker := fmt.Sprintf("T%09d", index)
			if _, err := storage.Watchlist.Update(owner.ID, func(current domain.Watchlist) (domain.Watchlist, error) {
				return current.Add(ticker)
			}); err != nil {
				t.Errorf("update %d: %v", index, err)
			}
		}(index)
	}
	group.Wait()

	if got := storage.Watchlist.List(owner.ID).Len(); got != 20 {
		t.Fatalf("length = %d, want 20", got)
	}
}
