package decypharr

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"

	"github.com/sirrobot01/decypharr/internal/testutil"
)

func TestMain(m *testing.M) {
	_, cleanup := testutil.SetupTestConfigDir()
	defer cleanup()
	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Test 1: safeGo panic recovery
//
// Exercises the real startServices function with nil manager and nil server.
// Both safeGo goroutines will hit a nil-pointer dereference, and the
// defer/recover inside safeGo must catch each panic, route the error through
// errChan, and cancel the service context so startServices returns normally.
// ---------------------------------------------------------------------------

func TestStartServices_PanicRecovery(t *testing.T) {
	// Arrange: a parent context we control and a cancel func that
	// startServices' error handler should invoke on first panic.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act: both srv.Start and manager.Start will panic on nil receiver.
	// startServices must recover both panics and return without crashing.
	done := make(chan error, 1)
	go func() {
		done <- startServices(ctx, nil, cancel, nil)
	}()

	select {
	case err := <-done:
		// startServices always returns nil (errors flow through errChan).
		if err != nil {
			t.Fatalf("startServices returned unexpected error: %v", err)
		}
	case <-ctx.Done():
		// Context was cancelled by the panic handler — now wait for return.
		if err := <-done; err != nil {
			t.Fatalf("startServices returned unexpected error after cancel: %v", err)
		}
	}

	// Assert: if we got here the process didn't crash, panics were recovered.
}

func TestStartServices_PanicDoesNotLeakGoroutines(t *testing.T) {
	// Capture baseline goroutine count, allowing a small margin for GC / runtime.
	runtime.GC()
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = startServices(ctx, nil, cancel, nil)
		close(done)
	}()
	<-done

	// Give background goroutines a moment to exit after errChan is closed.
	runtime.Gosched()
	runtime.GC()
	after := runtime.NumGoroutine()

	// We tolerate up to 2 extra goroutines (runtime background work).
	if delta := after - before; delta > 2 {
		t.Errorf("goroutine leak: before=%d after=%d delta=%d", before, after, delta)
	}
}

// ---------------------------------------------------------------------------
// Test 3: graceful shutdown
//
// Verifies that cancelling the parent context causes startServices to:
//   - unblock and return
//   - allow all internal goroutines to exit
// ---------------------------------------------------------------------------

func TestStartServices_GracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		// Using nil manager/server: both goroutines will panic and recover.
		// The panic handler calls cancel, ctx.Done() fires, startServices returns.
		done <- startServices(ctx, nil, cancel, nil)
	}()

	// Wait for startServices to return (panics trigger cancel).
	err := <-done
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartServices_ShutdownAfterCleanRun(t *testing.T) {
	// Test shutdown when services exit normally (no panic, no error).
	// We replicate the safeGo + errChan + wg pattern exactly from production
	// but use fake services that respect context cancellation.
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	errChan := make(chan error)

	safeGo := func(f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					_ = stack
					errChan <- fmt.Errorf("panic: %v", r)
				}
			}()
			if err := f(); err != nil {
				errChan <- err
			}
		}()
	}

	// Two fake services that block until context cancellation.
	safeGo(func() error {
		<-ctx.Done()
		return nil
	})
	safeGo(func() error {
		<-ctx.Done()
		return nil
	})

	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Error consumer (mirrors production).
	errsDone := make(chan struct{})
	go func() {
		for err := range errChan {
			if err != nil && ctx.Err() == nil {
				cancel()
			}
		}
		close(errsDone)
	}()

	// Cancel — services should exit, wg completes, errChan closes.
	cancel()

	// Wait for the error consumer to drain.
	<-errsDone
}

func TestGracefulShutdown_NoGoroutineLeaks(t *testing.T) {
	runtime.GC()
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = startServices(ctx, nil, cancel, nil)
		close(done)
	}()
	<-done

	// Allow internal goroutines to wind down.
	runtime.Gosched()
	runtime.GC()
	after := runtime.NumGoroutine()

	if delta := after - before; delta > 2 {
		t.Errorf("goroutine leak after shutdown: before=%d after=%d delta=%d",
			before, after, delta)
	}
}
