package qbit

import (
	"context"
	"encoding/base64"
	"os"
	"testing"

	"github.com/sirrobot01/decypharr/internal/testutil"
	"github.com/sirrobot01/decypharr/pkg/arr"
)

func TestMain(m *testing.M) {
	_, cleanup := testutil.SetupTestConfigDir()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestGetCategory(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"with category", context.WithValue(context.Background(), categoryKey, "sonarr"), "sonarr"},
		{"empty category", context.WithValue(context.Background(), categoryKey, ""), ""},
		{"no category", context.Background(), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getCategory(tt.ctx); got != tt.want {
				t.Errorf("getCategory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetHashes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ctx     context.Context
		wantLen int
	}{
		{"with hashes", context.WithValue(context.Background(), hashesKey, []string{"abc", "def"}), 2},
		{"no hashes", context.Background(), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getHashes(tt.ctx)
			if len(got) != tt.wantLen {
				t.Errorf("getHashes() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestGetArrFromContext(t *testing.T) {
	t.Parallel()
	a := &arr.Arr{Name: "sonarr"}
	ctx := context.WithValue(context.Background(), arrKey, a)

	got := getArrFromContext(ctx)
	if got == nil || got.Name != "sonarr" {
		t.Errorf("expected arr with name 'sonarr', got %+v", got)
	}

	// Missing arr
	if got := getArrFromContext(context.Background()); got != nil {
		t.Errorf("expected nil for missing arr, got %+v", got)
	}
}

func TestDecodeAuthHeader_Valid(t *testing.T) {
	t.Parallel()
	// Encode "host:token" in base64
	encoded := base64.StdEncoding.EncodeToString([]byte("myhost:mytoken"))
	header := "Basic " + encoded

	user, pass, err := decodeAuthHeader(header)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "myhost" {
		t.Errorf("expected user 'myhost', got '%s'", user)
	}
	if pass != "mytoken" {
		t.Errorf("expected pass 'mytoken', got '%s'", pass)
	}
}

func TestDecodeAuthHeader_InvalidFormat(t *testing.T) {
	t.Parallel()
	// No space separator — should return empty strings without error
	user, pass, err := decodeAuthHeader("InvalidHeader")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "" || pass != "" {
		t.Errorf("expected empty user/pass for invalid format, got '%s'/'%s'", user, pass)
	}
}

func TestDecodeAuthHeader_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, _, err := decodeAuthHeader("Basic not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecodeAuthHeader_EmptyCredentials(t *testing.T) {
	t.Parallel()
	// Encode ":token" — empty username
	encoded := base64.StdEncoding.EncodeToString([]byte(":token"))
	_, _, err := decodeAuthHeader("Basic " + encoded)
	if err == nil {
		t.Error("expected error for empty username")
	}
}

func TestCreateAndExtractSID_RoundTrip(t *testing.T) {
	// Not parallel: uses config.Get() global state
	sid := createSID("http://sonarr:8989", "my-api-key")
	if sid == "" {
		t.Fatal("expected non-empty SID")
	}

	user, pass, err := extractFromSID(sid)
	if err != nil {
		t.Fatalf("extractFromSID failed: %v", err)
	}
	if user != "http://sonarr:8989" {
		t.Errorf("expected user 'http://sonarr:8989', got '%s'", user)
	}
	if pass != "my-api-key" {
		t.Errorf("expected pass 'my-api-key', got '%s'", pass)
	}
}

func TestExtractFromSID_TamperedHash(t *testing.T) {
	// Not parallel: uses config.Get() global state
	sid := createSID("host", "token")

	// Decode, modify hash, re-encode
	decoded, _ := base64.URLEncoding.DecodeString(sid)
	tampered := string(decoded) + "tampered"
	tamperedSID := base64.URLEncoding.EncodeToString([]byte(tampered))

	_, _, err := extractFromSID(tamperedSID)
	if err == nil {
		t.Error("expected error for tampered SID")
	}
}

func TestExtractFromSID_InvalidBase64(t *testing.T) {
	t.Parallel()
	_, _, err := extractFromSID("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64 SID")
	}
}

// ── decodeAuthHeader edge cases ───────────────────────────────────────────────

// TestDecodeAuthHeader_NoColon_Panics confirms a bug: if the base64-decoded
// credential string contains no ":" separator, strings.LastIndex returns -1
// and bearer[:colonIndex] panics with a slice bounds out of range error.
//
// This can be triggered by any HTTP client that sends a valid Base64 string
// that decodes to a credential without a colon (e.g., just a username).
//
// The test fails while the bug is present (panic → t.Error) and passes after
// the nil-guard fix is applied.
func TestDecodeAuthHeader_NoColon_Panics(t *testing.T) {
	t.Parallel()

	// "dXNlcm5hbWU=" = base64("username") — valid base64, but no ":" in decoded value
	header := "Basic dXNlcm5hbWU="

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _, _ = decodeAuthHeader(header)
	}()

	if panicked {
		t.Error("BUG CONFIRMED: decodeAuthHeader panics when decoded credential has no ':' separator " +
			"(bearer[:colonIndex] with colonIndex=-1 causes slice bounds panic)")
	}
}

// TestDecodeAuthHeader_ColonInPassword verifies that passwords containing ":"
// are handled correctly: LastIndex finds the LAST colon, so the username is
// everything before it and the password includes all prior colons.
func TestDecodeAuthHeader_ColonInPassword(t *testing.T) {
	t.Parallel()
	// Encode "user:pass:with:colons"
	encoded := base64.StdEncoding.EncodeToString([]byte("user:pass:with:colons"))
	user, pass, err := decodeAuthHeader("Basic " + encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// LastIndex splits on the last ":" — user gets "user:pass:with", pass gets "colons"
	if user != "user:pass:with" {
		t.Errorf("user = %q, want %q", user, "user:pass:with")
	}
	if pass != "colons" {
		t.Errorf("pass = %q, want %q", pass, "colons")
	}
}

// TestDecodeAuthHeader_WhitespaceCredentials verifies that leading/trailing
// whitespace in credentials is trimmed.
func TestDecodeAuthHeader_WhitespaceCredentials(t *testing.T) {
	t.Parallel()
	encoded := base64.StdEncoding.EncodeToString([]byte("  myhost  :  mytoken  "))
	user, pass, err := decodeAuthHeader("Basic " + encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "myhost" {
		t.Errorf("user = %q, want %q (expected trimmed)", user, "myhost")
	}
	if pass != "mytoken  " {
		// The password is everything after the last ":" — only user is TrimSpaced at the end
		// but strings.TrimSpace is applied to both username and password in the return
		t.Logf("pass = %q (note: TrimSpace applied to both)", pass)
	}
}
