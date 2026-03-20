package decypharr

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// errChan consumer goroutine lifecycle tests
//
// startServices spawns four goroutines:
//
//   G1, G2: safeGo workers (srv.Start, manager.Start) — tracked by wg
//   G3:     wg-closer: wg.Wait() then close(errChan)
//   G4:     error consumer: for err := range errChan { ... }
//
// The critical property: G4 must exit when errChan is closed (by G3).
// startServices itself returns on <-ctx.Done() without joining G4,
// so G4 must self-terminate via the range loop ending.
//
// These tests replicate the exact production goroutine topology and verify
// that G4 exits under various scenarios.
// ---------------------------------------------------------------------------

// replicateStartServicesGoroutines builds the exact goroutine topology from
// startServices (lines 142-202 of main.go) with injectable worker functions.
// It returns channels that signal when each phase completes:
//   - consumerDone: closed when the error-consumer goroutine (G4) exits
//   - closerDone:   closed when the wg-closer goroutine (G3) exits
//
// The caller controls worker lifetime via workerFunc(ctx).
func replicateStartServicesGoroutines(
	ctx context.Context,
	cancelSvc context.CancelFunc,
	workerFunc func(ctx context.Context) error,
	numWorkers int,
) (consumerDone, closerDone chan struct{}) {
	var wg sync.WaitGroup
	errChan := make(chan error)

	consumerDone = make(chan struct{})
	closerDone = make(chan struct{})

	// safeGo — exact replica of production (lines 148-169)
	safeGo := func(f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					_ = debug.Stack()
					errChan <- fmt.Errorf("panic: %v", r)
				}
			}()
			if err := f(); err != nil {
				errChan <- err
			}
		}()
	}

	for i := 0; i < numWorkers; i++ {
		safeGo(func() error {
			return workerFunc(ctx)
		})
	}

	// G3: wg-closer (line 180-183)
	go func() {
		wg.Wait()
		close(errChan)
		close(closerDone)
	}()

	// G4: error consumer (lines 185-196)
	go func() {
		for err := range errChan {
			if err != nil {
				if ctx.Err() == nil {
					cancelSvc()
				}
			}
		}
		close(consumerDone)
	}()

	return consumerDone, closerDone
}

// ---------------------------------------------------------------------------
// Test: consumer exits when workers complete cleanly (no errors)
// ---------------------------------------------------------------------------

func TestErrChanConsumer_ExitsOnCleanWorkerCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	consumerDone, closerDone := replicateStartServicesGoroutines(
		ctx, cancel,
		func(ctx context.Context) error {
			<-ctx.Done()
			return nil // no error
		},
		2,
	)

	// Cancel context to let workers exit cleanly.
	cancel()

	// G3 (wg-closer) must finish, which closes errChan.
	<-closerDone

	// G4 (consumer) must exit because errChan is closed.
	<-consumerDone
}

// ---------------------------------------------------------------------------
// Test: consumer exits when workers return errors
// ---------------------------------------------------------------------------

func TestErrChanConsumer_ExitsAfterProcessingErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumerDone, closerDone := replicateStartServicesGoroutines(
		ctx, cancel,
		func(ctx context.Context) error {
			return fmt.Errorf("simulated service error")
		},
		2,
	)

	// Workers return errors immediately. G3 closes errChan after wg.Wait().
	<-closerDone

	// G4 must drain errors and exit.
	<-consumerDone
}

// ---------------------------------------------------------------------------
// Test: consumer exits when workers panic
// ---------------------------------------------------------------------------

func TestErrChanConsumer_ExitsAfterPanics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumerDone, closerDone := replicateStartServicesGoroutines(
		ctx, cancel,
		func(ctx context.Context) error {
			panic("test panic")
		},
		2,
	)

	<-closerDone
	<-consumerDone
}

// ---------------------------------------------------------------------------
// Test: consumer exits when mix of error and clean workers
// ---------------------------------------------------------------------------

func TestErrChanConsumer_ExitsWithMixedWorkerOutcomes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	errChan := make(chan error)
	consumerDone := make(chan struct{})

	safeGo := func(f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					_ = debug.Stack()
					errChan <- fmt.Errorf("panic: %v", r)
				}
			}()
			if err := f(); err != nil {
				errChan <- err
			}
		}()
	}

	// Worker 1: returns error immediately.
	safeGo(func() error {
		return fmt.Errorf("worker 1 failed")
	})

	// Worker 2: blocks until context cancelled, returns clean.
	safeGo(func() error {
		<-ctx.Done()
		return nil
	})

	// Worker 3: panics.
	safeGo(func() error {
		panic("worker 3 exploded")
	})

	go func() {
		wg.Wait()
		close(errChan)
	}()

	go func() {
		for err := range errChan {
			if err != nil && ctx.Err() == nil {
				cancel()
			}
		}
		close(consumerDone)
	}()

	// Error from worker 1 or panic from worker 3 triggers cancel,
	// which unblocks worker 2. All finish, errChan closes, consumer exits.
	<-consumerDone
}

// ---------------------------------------------------------------------------
// Test: no goroutine leak after full startServices cycle
//
// Uses the real startServices function with nil args (triggering panics).
// Verifies that ALL internal goroutines (G1-G4) exit after the function
// returns, leaving no leaked goroutines.
// ---------------------------------------------------------------------------

func TestErrChanConsumer_NoLeakAfterStartServices(t *testing.T) {
	// Stabilize goroutine count — run a dummy cycle first to warm up
	// any lazy runtime goroutines.
	warmCtx, warmCancel := context.WithCancel(context.Background())
	warmDone := make(chan struct{})
	go func() {
		_ = startServices(warmCtx, nil, warmCancel, nil)
		close(warmDone)
	}()
	<-warmDone
	runtime.Gosched()
	runtime.GC()

	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = startServices(ctx, nil, cancel, nil)
		close(done)
	}()
	<-done

	// startServices has returned, but G4 (error consumer) might still be
	// draining. Allow scheduler to run pending goroutines.
	runtime.Gosched()
	runtime.GC()

	after := runtime.NumGoroutine()
	if delta := after - before; delta > 0 {
		t.Errorf("goroutine leak: before=%d after=%d delta=%d — "+
			"error consumer goroutine may not have exited", before, after, delta)
	}
}

// ---------------------------------------------------------------------------
// Test: consumer processes all errors before exiting
//
// Verifies that closing errChan doesn't cause the consumer to drop pending
// errors — it must drain the channel fully before exiting.
// ---------------------------------------------------------------------------

func TestErrChanConsumer_DrainsAllErrorsBeforeExit(t *testing.T) {
	var errorsSeen atomic.Int32
	errChan := make(chan error, 10) // buffered to preload errors
	consumerDone := make(chan struct{})

	// Preload 5 errors into the buffered channel.
	for i := 0; i < 5; i++ {
		errChan <- fmt.Errorf("error %d", i)
	}
	close(errChan)

	// Replicate the consumer goroutine.
	go func() {
		for err := range errChan {
			if err != nil {
				errorsSeen.Add(1)
			}
		}
		close(consumerDone)
	}()

	<-consumerDone

	if got := errorsSeen.Load(); got != 5 {
		t.Errorf("consumer processed %d errors, want 5", got)
	}
}

// ---------------------------------------------------------------------------
// Test: ordering — G3 closes errChan AFTER all workers finish,
// G4 exits AFTER errChan is closed
// ---------------------------------------------------------------------------

func TestErrChanConsumer_OrderingGuarantees(t *testing.T) {
	var sequence atomic.Int64

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	errChan := make(chan error)

	// Track the highest sequence number assigned to any worker exit.
	var lastWorkerSeq atomic.Int64
	var closerOrder, consumerOrder int64

	// Two workers that record exit ordering.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ctx.Done()
			seq := sequence.Add(1)
			lastWorkerSeq.Store(seq) // safe: Store is atomic
		}()
	}

	closerDone := make(chan struct{})
	go func() {
		wg.Wait()
		closerOrder = sequence.Add(1) // must be after workers
		close(errChan)
		close(closerDone)
	}()

	consumerDone := make(chan struct{})
	go func() {
		for range errChan {
		}
		consumerOrder = sequence.Add(1) // must be after closer
		close(consumerDone)
	}()

	cancel()
	<-closerDone
	<-consumerDone

	// Workers exit first, then closer, then consumer.
	workerSeq := lastWorkerSeq.Load()
	if workerSeq <= 0 {
		t.Fatal("no worker recorded its exit sequence")
	}
	if closerOrder <= workerSeq {
		t.Errorf("closer (seq=%d) ran before last worker exit (seq=%d)",
			closerOrder, workerSeq)
	}
	if consumerOrder <= closerOrder {
		t.Errorf("consumer exited (seq=%d) before or at closer (seq=%d)",
			consumerOrder, closerOrder)
	}
}
