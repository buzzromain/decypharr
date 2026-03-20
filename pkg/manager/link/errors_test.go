package link

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorCategoryString(t *testing.T) {
	tests := []struct {
		cat  ErrorCategory
		want string
	}{
		{CategoryPermanent, "permanent"},
		{CategoryRetryable, "retryable"},
		{CategoryRefetchable, "refetchable"},
		{CategoryAccountIssue, "account_issue"},
		{ErrorCategory(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.cat.String(); got != tt.want {
			t.Errorf("ErrorCategory(%d).String() = %q, want %q", tt.cat, got, tt.want)
		}
	}
}

func TestErrorMethods(t *testing.T) {
	base := errors.New("base error")

	tests := []struct {
		name    string
		err     *Error
		retry   bool
		refetch bool
		disable bool
		perm    bool
		errStr  string
	}{
		{
			name:   "permanent with code",
			err:    NewPermanentError(base, "404"),
			perm:   true,
			errStr: "404: base error",
		},
		{
			name:   "permanent no code",
			err:    NewPermanentError(base, ""),
			perm:   true,
			errStr: "base error",
		},
		{
			name:   "retryable",
			err:    NewRetryableError(base, "503"),
			retry:  true,
			errStr: "503: base error",
		},
		{
			name:    "refetchable",
			err:     NewRefetchableError(base, "link_expired"),
			refetch: true,
			errStr:  "link_expired: base error",
		},
		{
			name:    "account issue",
			err:     NewAccountError(base, "bandwidth_exceeded"),
			disable: true,
			errStr:  "bandwidth_exceeded: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.ShouldRetry(); got != tt.retry {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.retry)
			}
			if got := tt.err.ShouldRefetch(); got != tt.refetch {
				t.Errorf("ShouldRefetch() = %v, want %v", got, tt.refetch)
			}
			if got := tt.err.ShouldDisableAccount(); got != tt.disable {
				t.Errorf("ShouldDisableAccount() = %v, want %v", got, tt.disable)
			}
			if got := tt.err.IsPermanent(); got != tt.perm {
				t.Errorf("IsPermanent() = %v, want %v", got, tt.perm)
			}
			if got := tt.err.Error(); got != tt.errStr {
				t.Errorf("Error() = %q, want %q", got, tt.errStr)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	base := errors.New("underlying")
	e := NewPermanentError(base, "")
	if !errors.Is(e, base) {
		t.Error("errors.Is should find underlying error via Unwrap")
	}
}

func TestErrorCodeToLinkError(t *testing.T) {
	tests := []struct {
		code     string
		category ErrorCategory
	}{
		{"link_not_found", CategoryPermanent},
		{"bandwidth_exceeded", CategoryAccountIssue},
		{"link_expired", CategoryRefetchable},
		{"file_not_available", CategoryPermanent},
		{"invalid_download_code", CategoryRefetchable},
		{"401", CategoryPermanent},
		{"unauthorized", CategoryPermanent},
		{"404", CategoryPermanent},
		{"429", CategoryRetryable},
		{"503", CategoryRetryable},
		{"something_unknown", CategoryPermanent},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			e := ErrorCodeToLinkError(tt.code)
			if e == nil {
				t.Fatal("expected non-nil error")
			}
			if e.Category != tt.category {
				t.Errorf("category = %v, want %v", e.Category, tt.category)
			}
			if e.Code != tt.code {
				t.Errorf("code = %q, want %q", e.Code, tt.code)
			}
		})
	}
}

func TestIsLinkError(t *testing.T) {
	linkErr := NewPermanentError(errors.New("x"), "code")
	if !IsLinkError(linkErr) {
		t.Error("IsLinkError should return true for *Error")
	}

	wrapped := fmt.Errorf("wrapped: %w", linkErr)
	if !IsLinkError(wrapped) {
		t.Error("IsLinkError should return true for wrapped *Error")
	}

	if IsLinkError(errors.New("plain")) {
		t.Error("IsLinkError should return false for plain error")
	}
	if IsLinkError(nil) {
		t.Error("IsLinkError should return false for nil")
	}
}

func TestGetLinkError(t *testing.T) {
	linkErr := NewRetryableError(errors.New("x"), "503")
	if got := GetLinkError(linkErr); got != linkErr {
		t.Error("GetLinkError should return the same *Error")
	}

	wrapped := fmt.Errorf("wrapped: %w", linkErr)
	got := GetLinkError(wrapped)
	if got == nil {
		t.Fatal("GetLinkError should find *Error through wrapping")
	}
	if got.Code != "503" {
		t.Errorf("code = %q, want %q", got.Code, "503")
	}

	if got := GetLinkError(errors.New("plain")); got != nil {
		t.Errorf("expected nil for plain error, got %+v", got)
	}
	if got := GetLinkError(nil); got != nil {
		t.Errorf("expected nil for nil error, got %+v", got)
	}
}
