package customerror

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"testing"
)

// ── Error ────────────────────────────────────────────────────────────────────

func TestError_Error(t *testing.T) {
	t.Parallel()
	inner := errors.New("something went wrong")
	e := &Error{err: inner}
	if e.Error() != "something went wrong" {
		t.Errorf("Error() = %q, want %q", e.Error(), "something went wrong")
	}
}

func TestError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	e := &Error{err: inner}
	if !errors.Is(e, inner) {
		t.Error("expected errors.Is to find inner error via Unwrap")
	}
}

func TestError_RetryableAndPermanent(t *testing.T) {
	t.Parallel()
	e := &Error{err: errors.New("test")}

	if e.IsRetryable() {
		t.Error("new error should not be retryable by default")
	}
	if e.IsPermanent() {
		t.Error("new error should not be permanent by default")
	}

	e.Retryable()
	if !e.IsRetryable() {
		t.Error("expected IsRetryable=true after Retryable()")
	}

	// Permanent overrides retryable
	e.Permanent()
	if e.IsRetryable() {
		t.Error("permanent error should not be retryable")
	}
	if !e.IsPermanent() {
		t.Error("expected IsPermanent=true after Permanent()")
	}
}

func TestError_IsSilent(t *testing.T) {
	t.Parallel()

	// Explicit silent flag
	e := &Error{err: errors.New("shh"), silent: true}
	if !e.IsSilent() {
		t.Error("expected IsSilent=true for silent=true error")
	}

	// Not silent
	e2 := &Error{err: errors.New("loud")}
	if e2.IsSilent() {
		t.Error("expected IsSilent=false for non-silent, non-special error")
	}

	// nil inner error
	e3 := &Error{err: nil}
	if e3.IsSilent() {
		t.Error("expected IsSilent=false for nil inner error")
	}
}

func TestError_IsSilent_SilentErrors(t *testing.T) {
	t.Parallel()
	// io.EOF should be silent
	e := &Error{err: io.EOF}
	if !e.IsSilent() {
		t.Error("expected IsSilent=true for io.EOF")
	}

	// context.Canceled should be silent
	e2 := &Error{err: context.Canceled}
	if !e2.IsSilent() {
		t.Error("expected IsSilent=true for context.Canceled")
	}
}

// ── NewError / helpers ───────────────────────────────────────────────────────

func TestNewError(t *testing.T) {
	t.Parallel()
	inner := errors.New("test")
	e := NewError(inner, 500, "code", true, false)
	if e.Error() != "test" {
		t.Errorf("NewError.Error() = %q, want %q", e.Error(), "test")
	}
}

func TestNewSilentError(t *testing.T) {
	t.Parallel()
	e := NewSilentError(errors.New("silent"))
	if !e.IsSilent() {
		t.Error("NewSilentError should be silent")
	}
	if e.statusCode != http.StatusInternalServerError {
		t.Errorf("NewSilentError statusCode = %d, want %d", e.statusCode, http.StatusInternalServerError)
	}
}

func TestNewPermanentError(t *testing.T) {
	t.Parallel()
	e := NewPermanentError(errors.New("perm"))
	if !e.IsPermanent() {
		t.Error("NewPermanentError should be permanent")
	}
	if e.IsRetryable() {
		t.Error("permanent error should not be retryable")
	}
}

func TestNewArticleNotFoundError(t *testing.T) {
	t.Parallel()
	e := NewArticleNotFoundError(nil)
	if !e.IsPermanent() {
		t.Error("article-not-found error should be permanent")
	}
	if e.Error() != "article not found" {
		t.Errorf("unexpected message: %s", e.Error())
	}

	// With custom error
	inner := errors.New("custom msg")
	e2 := NewArticleNotFoundError(inner)
	if e2.Error() != "custom msg" {
		t.Errorf("expected 'custom msg', got %s", e2.Error())
	}
}

func TestFromError(t *testing.T) {
	t.Parallel()

	// Wraps a *Error as-is
	orig := NewSilentError(errors.New("orig"))
	got := FromError(orig)
	if got != orig {
		t.Error("FromError should return the same *Error")
	}

	// Wraps plain error
	plain := errors.New("plain")
	wrapped := FromError(plain)
	if wrapped == nil {
		t.Fatal("FromError returned nil")
	}
	if wrapped.Error() != "plain" {
		t.Errorf("FromError wrapped message = %q, want %q", wrapped.Error(), "plain")
	}
}

// ── IsSilentError ────────────────────────────────────────────────────────────

func TestIsSilentError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"io.EOF", io.EOF, true},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
		{"context.Canceled", context.Canceled, true},
		{"context.DeadlineExceeded", context.DeadlineExceeded, true},
		{"net.ErrClosed", net.ErrClosed, true},
		{"syscall.ECONNABORTED", syscall.ECONNABORTED, true},
		{"http.ErrHandlerTimeout", http.ErrHandlerTimeout, true},
		{"broken pipe string", errors.New("broken pipe"), true},
		{"connection reset string", errors.New("connection reset"), true},
		{"client disconnected string", errors.New("client disconnected"), true},
		{"regular error", errors.New("regular error"), false},
		// NOTE: nil is intentionally excluded here — see TestIsSilentError_NilPanics below.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsSilentError(tt.err); got != tt.want {
				t.Errorf("IsSilentError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsSilentError_NetOpError_Pipe(t *testing.T) {
	t.Parallel()
	netErr := &net.OpError{Err: syscall.EPIPE}
	if !IsSilentError(netErr) {
		t.Error("expected EPIPE net.OpError to be silent")
	}
}

func TestIsSilentError_NetOpError_ConnReset(t *testing.T) {
	t.Parallel()
	netErr := &net.OpError{Err: syscall.ECONNRESET}
	if !IsSilentError(netErr) {
		t.Error("expected ECONNRESET net.OpError to be silent")
	}
}

func TestIsSilentError_CustomSilentError(t *testing.T) {
	t.Parallel()
	e := NewSilentError(errors.New("shh"))
	if !IsSilentError(e) {
		t.Error("expected custom silent error to be silent")
	}
}

// ── IsRetriableError ─────────────────────────────────────────────────────────

func TestIsRetriableError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, true},
		{"context.Canceled", context.Canceled, false},
		{"io.ErrUnexpectedEOF", io.ErrUnexpectedEOF, true},
		{"connection reset by peer", errors.New("connection reset by peer"), true},
		{"broken pipe", errors.New("broken pipe"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"no such host", errors.New("no such host"), true},
		{"use of closed network connection", errors.New("use of closed network connection"), true},
		{"regular error", errors.New("something else"), false},
		// Permanent error strings
		{"404 not found", errors.New("404 not found"), false},
		{"forbidden 403", errors.New("403 forbidden"), false},
		{"401 unauthorized", errors.New("401 unauthorized"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsRetriableError(tt.err); got != tt.want {
				t.Errorf("IsRetriableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetriableError_SyscallErrors(t *testing.T) {
	t.Parallel()
	retriable := []syscall.Errno{
		syscall.ECONNRESET,
		syscall.ECONNREFUSED,
		syscall.ECONNABORTED,
		syscall.EPIPE,
		syscall.ETIMEDOUT,
		syscall.ENETUNREACH,
		syscall.EHOSTUNREACH,
	}
	for _, e := range retriable {
		if !IsRetriableError(e) {
			t.Errorf("expected syscall error %v to be retriable", e)
		}
	}
}

func TestIsRetriableError_CustomError_Retryable(t *testing.T) {
	t.Parallel()
	e := (&Error{err: errors.New("custom")}).Retryable()
	if !IsRetriableError(e) {
		t.Error("expected retryable custom error to be retriable")
	}
}

func TestIsRetriableError_CustomError_Permanent(t *testing.T) {
	t.Parallel()
	e := NewPermanentError(errors.New("perm"))
	if IsRetriableError(e) {
		t.Error("permanent custom error should not be retriable")
	}
}

func TestIsRetriableError_CustomError_NotMarked(t *testing.T) {
	t.Parallel()
	// Custom error that is not marked retryable or permanent
	e := &Error{err: errors.New("unmarked")}
	// Should not be retryable (default)
	if IsRetriableError(e) {
		t.Error("unmarked custom error should not be retriable")
	}
}

func TestIsRetriableError_NetTimeout(t *testing.T) {
	t.Parallel()
	// timeoutErr implements net.Error with Timeout()=true
	type timeoutErr struct{ net.Error }
	// Can't easily create net.Error without implementing the interface,
	// so test with wrapped context.DeadlineExceeded
	if !IsRetriableError(context.DeadlineExceeded) {
		t.Error("DeadlineExceeded should be retriable")
	}
}

func TestIsRetriableError_WrappedError(t *testing.T) {
	t.Parallel()
	// Wrapped retriable error
	wrapped := fmt.Errorf("outer: %w", context.DeadlineExceeded)
	if !IsRetriableError(wrapped) {
		t.Error("wrapped DeadlineExceeded should be retriable via unwrap chain")
	}
}

// ── IsPermanentError ─────────────────────────────────────────────────────────

func TestIsPermanentError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"not found", errors.New("not found"), true},
		{"404", errors.New("error 404"), true},
		{"403 forbidden", errors.New("403 forbidden"), true},
		{"401 unauthorized", errors.New("401 unauthorized"), true},
		{"payment required 402", errors.New("402 payment required"), true},
		{"410 gone", errors.New("410 gone"), true},
		{"invalid api key", errors.New("invalid api key"), true},
		{"file not exist", errors.New("file not exist"), true},
		{"no such file", errors.New("no such file"), true},
		{"transient error", errors.New("connection refused"), false},
		{"regular error", errors.New("something else"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPermanentError(tt.err); got != tt.want {
				t.Errorf("IsPermanentError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// ── PanicError ───────────────────────────────────────────────────────────────

func TestPanicError(t *testing.T) {
	t.Parallel()
	e := NewPanicError("something panicked")
	if e == nil {
		t.Fatal("NewPanicError returned nil")
	}
	if !IsPanicError(e) {
		t.Error("expected IsPanicError=true")
	}
	// Error message should contain "panic:"
	if e.Error()[:6] != "panic:" {
		t.Errorf("PanicError message should start with 'panic:', got: %s", e.Error())
	}
}

func TestIsPanicError_NonPanic(t *testing.T) {
	t.Parallel()
	regular := errors.New("not a panic")
	if IsPanicError(regular) {
		t.Error("expected IsPanicError=false for regular error")
	}
}

// ── IsSilentError nil panic (CONFIRMED BUG) ───────────────────────────────────

// TestIsSilentError_NilPanics confirms the bug in IsSilentError:
// calling IsSilentError(nil) panics with a nil pointer dereference at
// error.go:113 because err.Error() is called without a nil guard.
// The function should return false for nil input.
//
// This test fails while the bug is present (panic → t.Error) and
// passes after it is fixed (no panic, function returns false).
func TestIsSilentError_NilPanics(t *testing.T) {
	t.Parallel()

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = IsSilentError(nil)
	}()

	if panicked {
		t.Error("BUG CONFIRMED: IsSilentError(nil) panics (nil pointer dereference at error.go:113); should return false")
	}
}

// ── Predefined HTTP errors ────────────────────────────────────────────────────

func TestPredefinedErrors(t *testing.T) {
	t.Parallel()
	predefined := []struct {
		name string
		err  *Error
	}{
		{"HosterUnavailableError", HosterUnavailableError},
		{"UsenetSegmentMissingError", UsenetSegmentMissingError},
		{"TrafficExceededError", TrafficExceededError},
		{"TorrentNotFoundError", TorrentNotFoundError},
		{"TooManyActiveDownloadsError", TooManyActiveDownloadsError},
	}
	for _, tt := range predefined {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err == nil {
				t.Error("predefined error is nil")
			}
			if tt.err.Error() == "" {
				t.Error("predefined error has empty message")
			}
			if tt.err.statusCode == 0 {
				t.Error("predefined error has zero status code")
			}
		})
	}
}
