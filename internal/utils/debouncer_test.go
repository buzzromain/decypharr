package utils

import (
	"testing"
	"time"
)

func TestDebouncerCallsEventually(t *testing.T) {
	called := make(chan struct{}, 1)
	d := NewDebouncer[int](20*time.Millisecond, func(_ int) {
		called <- struct{}{}
	})

	d.Call(1)

	select {
	case <-called:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("debounced call did not fire")
	}
	select {
	case <-called:
		t.Fatal("debounced call fired more than once")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDebouncerCoalescesRapidCalls(t *testing.T) {
	called := make(chan int, 2)
	d := NewDebouncer[int](30*time.Millisecond, func(_ int) {
		called <- 1
	})

	// Fire many calls rapidly — only the last one should trigger
	for i := 0; i < 10; i++ {
		d.Call(i)
	}

	select {
	case <-called:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("debounced call did not fire")
	}
	select {
	case <-called:
		t.Fatal("rapid calls should coalesce to one callback")
	case <-time.After(80 * time.Millisecond):
	}
}

func TestDebouncerStopPreventsCall(t *testing.T) {
	called := make(chan struct{}, 1)
	d := NewDebouncer[int](50*time.Millisecond, func(_ int) {
		called <- struct{}{}
	})

	d.Call(1)
	d.Stop()

	select {
	case <-called:
		t.Fatal("callback should not fire after Stop")
	case <-time.After(120 * time.Millisecond):
	}
}

func TestDebouncerPassesArgument(t *testing.T) {
	ch := make(chan int, 1)
	d := NewDebouncer[int](10*time.Millisecond, func(arg int) {
		ch <- arg
	})

	d.Call(42)

	select {
	case got := <-ch:
		if got != 42 {
			t.Errorf("arg = %d, want 42", got)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("debouncer never fired")
	}
}
