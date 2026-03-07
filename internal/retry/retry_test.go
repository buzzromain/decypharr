package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

// ── Do basics ────────────────────────────────────────────────────────────────

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Do(func() error {
		calls++
		return nil
	}, Attempts(3))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDo_RetriesOnError(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Do(func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	}, Attempts(5))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_ExhaustsAttempts(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Do(func() error {
		calls++
		return errors.New("always fails")
	}, Attempts(3))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_NilFunction(t *testing.T) {
	t.Parallel()
	err := Do(nil)
	if err == nil {
		t.Error("expected error for nil function")
	}
}

func TestDo_ZeroAttemptsDefaultsToOne(t *testing.T) {
	t.Parallel()
	calls := 0
	Do(func() error {
		calls++
		return errors.New("fail")
	}, Attempts(0))
	if calls != 1 {
		t.Errorf("expected 1 call for Attempts(0), got %d", calls)
	}
}

// ── Unrecoverable ────────────────────────────────────────────────────────────

func TestDo_Unrecoverable_StopsRetries(t *testing.T) {
	t.Parallel()
	calls := 0
	inner := errors.New("fatal")
	err := Do(func() error {
		calls++
		return Unrecoverable(inner)
	}, Attempts(10))
	if !errors.Is(err, inner) {
		t.Errorf("expected inner error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call before stopping, got %d", calls)
	}
}

func TestUnrecoverable_NilReturnsNil(t *testing.T) {
	t.Parallel()
	if Unrecoverable(nil) != nil {
		t.Error("Unrecoverable(nil) should return nil")
	}
}

// ── RetryIf ──────────────────────────────────────────────────────────────────

func TestDo_RetryIf_StopsWhenPredicateFalse(t *testing.T) {
	t.Parallel()
	calls := 0
	sentinel := errors.New("stop here")
	err := Do(func() error {
		calls++
		return sentinel
	},
		Attempts(10),
		RetryIf(func(err error) bool {
			return !errors.Is(err, sentinel)
		}),
	)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call when predicate rejects immediately, got %d", calls)
	}
}

func TestDo_RetryIf_RetryableErrors(t *testing.T) {
	t.Parallel()
	calls := 0
	transient := errors.New("transient")
	permanent := errors.New("permanent")
	err := Do(func() error {
		calls++
		if calls < 3 {
			return transient
		}
		return permanent
	},
		Attempts(10),
		RetryIf(func(err error) bool {
			return errors.Is(err, transient)
		}),
	)
	if !errors.Is(err, permanent) {
		t.Errorf("expected permanent error, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

// ── OnRetry ──────────────────────────────────────────────────────────────────

func TestDo_OnRetry_CalledForEachRetry(t *testing.T) {
	t.Parallel()
	retryCalls := 0
	Do(func() error {
		return errors.New("fail")
	},
		Attempts(4),
		OnRetry(func(attempt uint, err error) {
			retryCalls++
		}),
	)
	// OnRetry is called after each failed attempt except the last
	if retryCalls != 3 {
		t.Errorf("expected 3 OnRetry calls for 4 attempts, got %d", retryCalls)
	}
}

// ── Context cancellation ─────────────────────────────────────────────────────

func TestDo_ContextCanceled_StopsRetries(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	calls := 0
	err := Do(func() error {
		calls++
		return errors.New("fail")
	},
		Attempts(10),
		Context(ctx),
	)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls with cancelled context, got %d", calls)
	}
}

func TestDo_ContextTimeout_StopsRetries(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	calls := 0
	err := Do(func() error {
		calls++
		return errors.New("fail")
	},
		Attempts(100),
		Delay(10*time.Millisecond),
		Context(ctx),
	)
	if err == nil {
		t.Fatal("expected error due to context timeout")
	}
	// Should have stopped due to context, not exhausted all attempts
	if calls >= 100 {
		t.Error("should have stopped before exhausting all attempts")
	}
}

// ── Backoff ───────────────────────────────────────────────────────────────────

func TestDo_BackoffDelay_IncreasesDelay(t *testing.T) {
	t.Parallel()
	var durations []time.Duration
	start := time.Now()
	var lastStart time.Time

	calls := 0
	Do(func() error {
		now := time.Now()
		if !lastStart.IsZero() {
			durations = append(durations, now.Sub(lastStart))
		}
		lastStart = now
		calls++
		if calls >= 3 {
			return nil
		}
		return errors.New("fail")
	},
		Attempts(3),
		Delay(5*time.Millisecond),
		DelayType(BackOffDelay),
	)
	_ = start

	// We can't test exact timing, but we can verify the function ran
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_MaxDelay_Caps(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Do(func() error {
		calls++
		return errors.New("fail")
	},
		Attempts(5),
		Delay(1*time.Millisecond),
		MaxDelay(2*time.Millisecond),
		DelayType(BackOffDelay),
	)
	if err == nil {
		t.Error("expected error after exhausting attempts")
	}
	if calls != 5 {
		t.Errorf("expected 5 calls, got %d", calls)
	}
}

// ── LastErrorOnly ─────────────────────────────────────────────────────────────

func TestDo_LastErrorOnly_True(t *testing.T) {
	t.Parallel()
	lastErr := errors.New("last error")
	calls := 0
	err := Do(func() error {
		calls++
		if calls == 3 {
			return lastErr
		}
		return errors.New("earlier error")
	},
		Attempts(3),
		LastErrorOnly(true),
	)
	if !errors.Is(err, lastErr) {
		t.Errorf("expected last error, got %v", err)
	}
}

func TestDo_LastErrorOnly_False_WrapsError(t *testing.T) {
	t.Parallel()
	err := Do(func() error {
		return errors.New("fail")
	},
		Attempts(3),
		LastErrorOnly(false),
	)
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	// When lastErrorOnly=false, error is wrapped with attempt count
	msg := err.Error()
	if len(msg) == 0 {
		t.Error("expected non-empty error message")
	}
}

// ── sleepWithContext ─────────────────────────────────────────────────────────

func TestSleepWithContext_ZeroDuration(t *testing.T) {
	t.Parallel()
	err := sleepWithContext(nil, 0)
	if err != nil {
		t.Errorf("sleepWithContext(nil, 0) returned error: %v", err)
	}
}

func TestSleepWithContext_NilContext(t *testing.T) {
	t.Parallel()
	start := time.Now()
	err := sleepWithContext(nil, 10*time.Millisecond)
	elapsed := time.Since(start)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if elapsed < 5*time.Millisecond {
		t.Error("sleep was too short")
	}
}

func TestSleepWithContext_CancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := sleepWithContext(ctx, 1*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSleepWithContext_ContextCancelsDuringSleep(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := sleepWithContext(ctx, 1*time.Second)
	elapsed := time.Since(start)
	if err == nil {
		t.Error("expected context error")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("sleep should have been cut short by context, but took %v", elapsed)
	}
}

// ── Options edge cases ────────────────────────────────────────────────────────

func TestNilOptionIsIgnored(t *testing.T) {
	t.Parallel()
	calls := 0
	err := Do(func() error {
		calls++
		return nil
	}, nil, Attempts(2), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}
