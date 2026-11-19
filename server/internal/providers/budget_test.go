package providers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"stock-dashboard/server/internal/domain"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 11, 16, 14, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

func TestBudgetSlidingWindow(t *testing.T) {
	clock := newFakeClock()
	budget := NewBudget(domain.ProviderFinnhub, 2, clock.Now)

	for i := 0; i < 2; i++ {
		ok, _ := budget.TryReserve()
		if !ok {
			t.Fatalf("reservation %d refused", i)
		}
	}

	ok, retry := budget.TryReserve()
	if ok {
		t.Fatal("third reservation should be refused")
	}
	if retry != time.Minute {
		t.Fatalf("retry = %v, want 1m", retry)
	}

	clock.Advance(30 * time.Second)
	ok, retry = budget.TryReserve()
	if ok || retry != 30*time.Second {
		t.Fatalf("after 30s: ok=%v retry=%v", ok, retry)
	}

	clock.Advance(30 * time.Second)
	ok, retry = budget.TryReserve()
	if !ok || retry != 0 {
		t.Fatalf("slot did not free at the window edge: ok=%v retry=%v", ok, retry)
	}
}

func TestBudgetLimitZero(t *testing.T) {
	budget := NewBudget(domain.ProviderTwelveData, 0, newFakeClock().Now)
	ok, retry := budget.TryReserve()
	if ok || retry != 0 {
		t.Fatalf("limit zero: ok=%v retry=%v", ok, retry)
	}

	err := budget.Reserve(context.Background())
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureRateLimited {
		t.Fatalf("Reserve error = %v, want rate-limited failure", err)
	}
	if failure.Provider != domain.ProviderTwelveData {
		t.Fatalf("provider = %q", failure.Provider)
	}
	if failure.RetryAfter <= 0 {
		t.Fatalf("retry after = %v", failure.RetryAfter)
	}
}

func TestBudgetReserveRefusesLongWait(t *testing.T) {
	budget := NewBudget(domain.ProviderFinnhub, 1, newFakeClock().Now)
	if err := budget.Reserve(context.Background()); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	err := budget.Reserve(context.Background())
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != FailureRateLimited {
		t.Fatalf("error = %v", err)
	}
	if failure.RetryAfter < 59*time.Second {
		t.Fatalf("retry after = %v", failure.RetryAfter)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Reserve blocked for %v on a long wait", elapsed)
	}
}

func TestBudgetReserveWaitsForShortSlot(t *testing.T) {
	clock := newFakeClock()
	waited := time.Duration(0)
	budget := newBudgetWithWait(domain.ProviderFinnhub, 1, clock.Now, func(_ context.Context, duration time.Duration) error {
		waited = duration
		clock.Advance(duration)
		return nil
	})

	if err := budget.Reserve(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(59 * time.Second)
	if err := budget.Reserve(context.Background()); err != nil {
		t.Fatalf("Reserve after 59s = %v", err)
	}
	if waited != time.Second {
		t.Fatalf("waited = %v, want 1s", waited)
	}
}

func TestBudgetReserveHonoursCancellation(t *testing.T) {
	clock := newFakeClock()
	budget := newBudgetWithWait(domain.ProviderFinnhub, 1, clock.Now, func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	})
	if err := budget.Reserve(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock.Advance(59 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := budget.Reserve(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestBudgetConcurrentReservations(t *testing.T) {
	budget := NewBudget(domain.ProviderFinnhub, 10, newFakeClock().Now)
	var granted atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 100; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if ok, _ := budget.TryReserve(); ok {
				granted.Add(1)
			}
		}()
	}
	group.Wait()
	if got := granted.Load(); got != 10 {
		t.Fatalf("granted = %d, want 10", got)
	}
	if wait := budget.retryAfterWait(); wait != time.Minute {
		t.Fatalf("RetryAfter = %v, want 1m", wait)
	}
}
