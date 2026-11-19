package providers

import (
	"context"
	"sync"
	"time"

	"stock-dashboard/server/internal/domain"
)

const (
	budgetWindow  = time.Minute
	maxBudgetWait = 2 * time.Second
)

// Budget is a sliding one-minute window of upstream calls for one provider.
type Budget struct {
	mu       sync.Mutex
	provider domain.ProviderAttribution
	limit    int
	window   time.Duration
	now      func() time.Time
	wait     func(context.Context, time.Duration) error
	calls    []time.Time
}

// NewBudget builds a budget using the supplied clock (nil means time.Now).
func NewBudget(provider domain.ProviderAttribution, limitPerMinute int, now func() time.Time) *Budget {
	if limitPerMinute < 0 {
		limitPerMinute = 0
	}
	if now == nil {
		now = time.Now
	}
	return &Budget{
		provider: provider,
		limit:    limitPerMinute,
		window:   budgetWindow,
		now:      now,
		wait:     waitFor,
	}
}

func newBudgetWithWait(provider domain.ProviderAttribution, limitPerMinute int, now func() time.Time, wait func(context.Context, time.Duration) error) *Budget {
	budget := NewBudget(provider, limitPerMinute, now)
	budget.wait = wait
	return budget
}

// TryReserve claims a slot without blocking. When no slot is free it returns the
// duration until the oldest call leaves the window.
func (b *Budget) TryReserve() (ok bool, retryAfter time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	b.prune(now)
	if len(b.calls) < b.limit {
		b.calls = append(b.calls, now)
		return true, 0
	}
	return false, b.retryAfter(now)
}

// Reserve waits up to two seconds for a slot and then returns a rate-limited Failure.
func (b *Budget) Reserve(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var waited time.Duration
	for {
		ok, retryAfter := b.TryReserve()
		if ok {
			return nil
		}
		if retryAfter <= 0 {
			return &Failure{Kind: FailureRateLimited, Provider: b.provider, RetryAfter: time.Second}
		}
		if waited+retryAfter > maxBudgetWait {
			return &Failure{Kind: FailureRateLimited, Provider: b.provider, RetryAfter: retryAfter}
		}
		if err := b.wait(ctx, retryAfter); err != nil {
			return err
		}
		waited += retryAfter
	}
}

// retryAfterWait reports the current wait until a slot frees up.
func (b *Budget) retryAfterWait() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	b.prune(now)
	return b.retryAfter(now)
}

func (b *Budget) prune(now time.Time) {
	cutoff := now.Add(-b.window)
	kept := b.calls[:0]
	for _, call := range b.calls {
		if call.After(cutoff) {
			kept = append(kept, call)
		}
	}
	b.calls = kept
}

func (b *Budget) retryAfter(now time.Time) time.Duration {
	if len(b.calls) == 0 {
		return 0
	}
	remaining := b.calls[0].Add(b.window).Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func waitFor(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
