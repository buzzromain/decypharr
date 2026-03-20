package config

import (
	"strings"
	"testing"
)

// ── IsZero helpers ────────────────────────────────────────────────────────────

func TestQBitTorrentIsZero(t *testing.T) {
	if !(QBitTorrent{}).IsZero() {
		t.Error("zero value should be IsZero")
	}
	if (QBitTorrent{DownloadFolder: "/dl"}).IsZero() {
		t.Error("non-zero value should not be IsZero")
	}
}

func TestArrIsZero(t *testing.T) {
	if !(Arr{}).IsZero() {
		t.Error("zero value should be IsZero")
	}
	if (Arr{Name: "sonarr"}).IsZero() {
		t.Error("non-zero value should not be IsZero")
	}
}

func TestUsenetIsZero(t *testing.T) {
	if !(Usenet{}).IsZero() {
		t.Error("zero value should be IsZero")
	}
	if (Usenet{Providers: []UsenetProvider{{Host: "news.example.com"}}}).IsZero() {
		t.Error("non-zero value should not be IsZero")
	}
	if (Usenet{MaxConnections: 5}).IsZero() {
		t.Error("non-zero MaxConnections should not be IsZero")
	}
}

// ── validateDebrids ───────────────────────────────────────────────────────────

func TestValidateDebrids_Empty(t *testing.T) {
	if err := validateDebrids(nil); err != nil {
		t.Errorf("empty debrids should pass validation, got: %v", err)
	}
}

func TestValidateDebrids_MissingAPIKey(t *testing.T) {
	debrids := []Debrid{{Name: "rd", Provider: "realdebrid"}}
	if err := validateDebrids(debrids); err == nil {
		t.Error("missing api_key should fail validation")
	}
}

func TestValidateDebrids_ValidDebrid(t *testing.T) {
	debrids := []Debrid{{Name: "rd", Provider: "realdebrid", APIKey: "secret"}}
	if err := validateDebrids(debrids); err != nil {
		t.Errorf("valid debrid should pass, got: %v", err)
	}
}

func TestValidateDebrids_SlotStrategyWrongProvider(t *testing.T) {
	debrids := []Debrid{{APIKey: "key", Provider: "realdebrid", SlotStrategy: "remove_after_add"}}
	if err := validateDebrids(debrids); err == nil {
		t.Error("slot_strategy on non-alldebrid provider should fail")
	}
}

func TestValidateDebrids_InvalidSlotStrategy(t *testing.T) {
	debrids := []Debrid{{APIKey: "key", Provider: "alldebrid", SlotStrategy: "invalid"}}
	if err := validateDebrids(debrids); err == nil {
		t.Error("invalid slot_strategy should fail")
	}
}

func TestValidateDebrids_ValidSlotStrategy(t *testing.T) {
	for _, strategy := range []string{"remove_after_add", "remove_oldest"} {
		debrids := []Debrid{{APIKey: "key", Provider: "alldebrid", SlotStrategy: strategy}}
		if err := validateDebrids(debrids); err != nil {
			t.Errorf("valid slot_strategy %q should pass, got: %v", strategy, err)
		}
	}
}

// ── validateUsenet ────────────────────────────────────────────────────────────

func TestValidateUsenet_Empty(t *testing.T) {
	if err := validateUsenet(nil); err != nil {
		t.Errorf("empty usenet should pass, got: %v", err)
	}
}

func TestValidateUsenet_MissingHost(t *testing.T) {
	providers := []UsenetProvider{{Username: "u", Password: "p"}}
	if err := validateUsenet(providers); err == nil {
		t.Error("missing host should fail")
	}
}

func TestValidateUsenet_MissingUsername(t *testing.T) {
	providers := []UsenetProvider{{Host: "news.example.com", Password: "p"}}
	if err := validateUsenet(providers); err == nil {
		t.Error("missing username should fail")
	}
}

func TestValidateUsenet_MissingPassword(t *testing.T) {
	providers := []UsenetProvider{{Host: "news.example.com", Username: "u"}}
	if err := validateUsenet(providers); err == nil {
		t.Error("missing password should fail")
	}
}

func TestValidateUsenet_Valid(t *testing.T) {
	providers := []UsenetProvider{{Host: "news.example.com", Username: "u", Password: "p"}}
	if err := validateUsenet(providers); err != nil {
		t.Errorf("valid usenet provider should pass, got: %v", err)
	}
}

// ── Validate ──────────────────────────────────────────────────────────────────

func TestValidate_NoDownloadFolder(t *testing.T) {
	c := &Config{
		Debrids: []Debrid{{APIKey: "k", Provider: "realdebrid"}},
	}
	if err := c.Validate(); err == nil {
		t.Error("missing download folder should fail")
	}
}

func TestValidate_NoProviders(t *testing.T) {
	c := &Config{DownloadFolder: "/downloads"}
	if err := c.Validate(); err == nil {
		t.Error("no debrid or usenet providers should fail")
	}
}

func TestValidate_WithDebrid(t *testing.T) {
	c := &Config{
		DownloadFolder: "/downloads",
		Debrids:        []Debrid{{APIKey: "k", Provider: "realdebrid"}},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("valid config with debrid should pass, got: %v", err)
	}
}

func TestValidate_WithUsenet(t *testing.T) {
	c := &Config{
		DownloadFolder: "/downloads",
		Usenet: Usenet{
			Providers: []UsenetProvider{{Host: "news.example.com", Username: "u", Password: "p"}},
		},
	}
	if err := c.Validate(); err != nil {
		t.Errorf("valid config with usenet should pass, got: %v", err)
	}
}

// ── SetupComplete / SetupError ────────────────────────────────────────────────

func TestSetupComplete_Valid(t *testing.T) {
	c := &Config{
		DownloadFolder: "/downloads",
		Debrids:        []Debrid{{APIKey: "k", Provider: "rd"}},
	}
	if err := c.SetupComplete(); err != nil {
		t.Errorf("SetupComplete on valid config should return nil, got: %v", err)
	}
}

func TestSetupComplete_Invalid(t *testing.T) {
	c := &Config{}
	if err := c.SetupComplete(); err == nil {
		t.Error("SetupComplete on invalid config should return error")
	}
}

func TestSetupError_Valid(t *testing.T) {
	c := &Config{
		DownloadFolder: "/downloads",
		Debrids:        []Debrid{{APIKey: "k", Provider: "rd"}},
	}
	if msg := c.SetupError(); msg != "" {
		t.Errorf("SetupError on valid config should return empty string, got: %q", msg)
	}
}

func TestSetupError_Invalid(t *testing.T) {
	c := &Config{}
	if msg := c.SetupError(); msg == "" {
		t.Error("SetupError on invalid config should return non-empty string")
	}
}

// ── NeedsAuth ─────────────────────────────────────────────────────────────────

func TestNeedsAuth(t *testing.T) {
	tests := []struct {
		name    string
		c       Config
		want    bool
	}{
		{"use_auth=false", Config{UseAuth: false}, false},
		{"use_auth=true_nil_auth", Config{UseAuth: true, Auth: nil}, true},
		{"use_auth=true_empty_user", Config{UseAuth: true, Auth: &Auth{Username: "", Password: "p"}}, true},
		{"use_auth=true_empty_pass", Config{UseAuth: true, Auth: &Auth{Username: "u", Password: ""}}, true},
		{"use_auth=true_full_auth", Config{UseAuth: true, Auth: &Auth{Username: "u", Password: "p"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.c.NeedsAuth()
			if got != tt.want {
				t.Errorf("NeedsAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ── SecretKey ─────────────────────────────────────────────────────────────────

func TestSecretKey_Default(t *testing.T) {
	c := &Config{}
	key := c.SecretKey()
	if key == "" {
		t.Error("SecretKey should return a non-empty default")
	}
}

func TestSecretKey_FromEnv(t *testing.T) {
	t.Setenv("DECYPHARR_SECRET_KEY", "my-custom-secret")
	c := &Config{}
	if got := c.SecretKey(); got != "my-custom-secret" {
		t.Errorf("SecretKey() = %q, want %q", got, "my-custom-secret")
	}
}

// ── ShouldUseTorrentFile ──────────────────────────────────────────────────────

func TestShouldUseTorrentFile(t *testing.T) {
	trueVal := true
	falseVal := false
	tests := []struct {
		name string
		d    Debrid
		want bool
	}{
		{"nil_pointer_defaults_true", Debrid{UseTorrentFile: nil}, true},
		{"explicit_true", Debrid{UseTorrentFile: &trueVal}, true},
		{"explicit_false", Debrid{UseTorrentFile: &falseVal}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.ShouldUseTorrentFile(); got != tt.want {
				t.Errorf("ShouldUseTorrentFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ── IsEventEnabled ────────────────────────────────────────────────────────────

func TestIsEventEnabled_NotEnabled(t *testing.T) {
	n := &Notifications{Enabled: false}
	if n.IsEventEnabled(EventDownloadComplete) {
		t.Error("disabled notifications should return false for any event")
	}
}

func TestIsEventEnabled_AllEvents(t *testing.T) {
	n := &Notifications{Enabled: true, Events: nil}
	if !n.IsEventEnabled(EventDownloadComplete) {
		t.Error("empty Events list should allow all events")
	}
}

func TestIsEventEnabled_SpecificEvent(t *testing.T) {
	n := &Notifications{Enabled: true, Events: []NotificationEvent{EventRepairComplete}}
	if !n.IsEventEnabled(EventRepairComplete) {
		t.Error("listed event should be enabled")
	}
	if n.IsEventEnabled(EventDownloadComplete) {
		t.Error("unlisted event should not be enabled")
	}
}

// ── generateAPIToken ──────────────────────────────────────────────────────────

func TestGenerateAPIToken(t *testing.T) {
	token, err := generateAPIToken()
	if err != nil {
		t.Fatalf("generateAPIToken() returned error: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length = %d, want 64 (32 bytes hex-encoded)", len(token))
	}

	// Two calls should produce different tokens
	token2, _ := generateAPIToken()
	if token == token2 {
		t.Error("expected unique tokens across calls")
	}
}

// ── getDefaultExtensions ──────────────────────────────────────────────────────

func TestGetDefaultExtensions_ContainsExpected(t *testing.T) {
	exts := getDefaultExtensions()
	if len(exts) == 0 {
		t.Fatal("getDefaultExtensions() returned empty slice")
	}

	extSet := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		extSet[e] = struct{}{}
	}

	for _, expected := range []string{"mkv", "mp4", "avi", "flac", "mp3"} {
		if _, ok := extSet[expected]; !ok {
			t.Errorf("expected extension %q not found in defaults", expected)
		}
	}
}

func TestGetDefaultExtensions_NoDuplicates(t *testing.T) {
	exts := getDefaultExtensions()
	seen := make(map[string]struct{})
	for _, e := range exts {
		if _, dup := seen[e]; dup {
			t.Errorf("duplicate extension %q in default list", e)
		}
		seen[e] = struct{}{}
	}
}

func TestGetDefaultExtensions_AllLowercase(t *testing.T) {
	for _, e := range getDefaultExtensions() {
		if e != strings.ToLower(e) {
			t.Errorf("extension %q is not lowercase", e)
		}
	}
}

// ── updateDebrid ──────────────────────────────────────────────────────────────

func TestUpdateDebrid_FillsDefaults(t *testing.T) {
	c := &Config{Debrids: []Debrid{{Name: "rd", APIKey: "key"}}}
	updated := c.updateDebrid(c.Debrids[0])

	if updated.Provider != "rd" {
		t.Errorf("Provider: got %q, want %q (copied from Name)", updated.Provider, "rd")
	}
	if updated.TorrentsRefreshInterval != DefaultTorrentsRefreshInterval {
		t.Errorf("TorrentsRefreshInterval: got %q, want default", updated.TorrentsRefreshInterval)
	}
	if updated.DownloadLinksRefreshInterval != DefaultDownloadsRefreshInterval {
		t.Errorf("DownloadLinksRefreshInterval: got %q, want default", updated.DownloadLinksRefreshInterval)
	}
	if updated.AutoExpireLinksAfter != DefaultAutoExpireLinksAfter {
		t.Errorf("AutoExpireLinksAfter: got %q, want default", updated.AutoExpireLinksAfter)
	}
	if updated.Workers == 0 {
		t.Error("Workers should be set to a positive default")
	}
	if updated.UseTorrentFile == nil || !*updated.UseTorrentFile {
		t.Error("UseTorrentFile should default to true")
	}
}

func TestUpdateDebrid_DownloadKeysFromAPIKey(t *testing.T) {
	c := &Config{Debrids: []Debrid{{APIKey: "mykey"}}}
	updated := c.updateDebrid(c.Debrids[0])

	if len(updated.DownloadAPIKeys) != 1 || updated.DownloadAPIKeys[0] != "mykey" {
		t.Errorf("DownloadAPIKeys should be [mykey] when no explicit keys, got %v", updated.DownloadAPIKeys)
	}
}

func TestUpdateDebrid_ExistingDownloadKeys(t *testing.T) {
	c := &Config{Debrids: []Debrid{{APIKey: "key", DownloadAPIKeys: []string{"k1", "k2"}}}}
	updated := c.updateDebrid(c.Debrids[0])

	if len(updated.DownloadAPIKeys) != 2 {
		t.Errorf("DownloadAPIKeys should preserve existing keys, got %v", updated.DownloadAPIKeys)
	}
}

// ── Path helpers ──────────────────────────────────────────────────────────────

func TestPathHelpers(t *testing.T) {
	SetConfigPath("/test/config")
	c := &Config{}

	if got := c.JsonFile(); got != "/test/config/config.json" {
		t.Errorf("JsonFile() = %q, want /test/config/config.json", got)
	}
	if got := c.AuthFile(); got != "/test/config/auth.json" {
		t.Errorf("AuthFile() = %q, want /test/config/auth.json", got)
	}
	if got := c.TorrentsFile(); got != "/test/config/torrents.json" {
		t.Errorf("TorrentsFile() = %q, want /test/config/torrents.json", got)
	}
}

// ── applyDebridEnvVars ────────────────────────────────────────────────────────

func TestApplyDebridEnvVars_BasicFields(t *testing.T) {
	c := &Config{}
	t.Setenv("DECYPHARR_DEBRIDS__0__NAME", "realdebrid")
	t.Setenv("DECYPHARR_DEBRIDS__0__API_KEY", "my-api-key")
	t.Setenv("DECYPHARR_DEBRIDS__0__PROVIDER", "realdebrid")
	t.Setenv("DECYPHARR_DEBRIDS__0__PROXY", "http://proxy:8080")
	c.applyDebridEnvVars()

	if len(c.Debrids) == 0 {
		t.Fatal("Debrids should not be empty after env vars")
	}
	if c.Debrids[0].Name != "realdebrid" {
		t.Errorf("Name = %q, want realdebrid", c.Debrids[0].Name)
	}
	if c.Debrids[0].APIKey != "my-api-key" {
		t.Errorf("APIKey = %q, want my-api-key", c.Debrids[0].APIKey)
	}
	if c.Debrids[0].Provider != "realdebrid" {
		t.Errorf("Provider = %q, want realdebrid", c.Debrids[0].Provider)
	}
	if c.Debrids[0].Proxy != "http://proxy:8080" {
		t.Errorf("Proxy = %q, want http://proxy:8080", c.Debrids[0].Proxy)
	}
}

// ── applyUsenetEnvVars ────────────────────────────────────────────────────────

func TestApplyUsenetEnvVars_BasicFields(t *testing.T) {
	c := &Config{}
	t.Setenv("DECYPHARR_USENET__MAX_CONNECTIONS", "20")
	t.Setenv("DECYPHARR_USENET__READ_AHEAD", "32MB")
	t.Setenv("DECYPHARR_USENET__PROCESSING_TIMEOUT", "15m")
	t.Setenv("DECYPHARR_USENET__AVAILABILITY_SAMPLE_PERCENT", "50")
	t.Setenv("DECYPHARR_USENET__MAX_CONCURRENT_NZB", "5")
	t.Setenv("DECYPHARR_USENET__SKIP_REPAIR", "1")
	c.applyUsenetEnvVars()

	if c.Usenet.MaxConnections != 20 {
		t.Errorf("MaxConnections = %d, want 20", c.Usenet.MaxConnections)
	}
	if c.Usenet.ReadAhead != "32MB" {
		t.Errorf("ReadAhead = %q, want 32MB", c.Usenet.ReadAhead)
	}
	if c.Usenet.ProcessingTimeout != "15m" {
		t.Errorf("ProcessingTimeout = %q, want 15m", c.Usenet.ProcessingTimeout)
	}
	if c.Usenet.AvailabilitySamplePercent != 50 {
		t.Errorf("AvailabilitySamplePercent = %d, want 50", c.Usenet.AvailabilitySamplePercent)
	}
	if c.Usenet.MaxConcurrentNZB != 5 {
		t.Errorf("MaxConcurrentNZB = %d, want 5", c.Usenet.MaxConcurrentNZB)
	}
	if !c.Usenet.SkipRepair {
		t.Error("SkipRepair should be true")
	}
}

func TestApplyUsenetEnvVars_ProviderFields(t *testing.T) {
	c := &Config{}
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__HOST", "news.example.com")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PORT", "563")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__USERNAME", "user")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PASSWORD", "pass")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__MAX_CONNECTIONS", "30")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__SSL", "true")
	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__PRIORITY", "2")
	c.applyUsenetEnvVars()

	if len(c.Usenet.Providers) == 0 {
		t.Fatal("Providers should not be empty after env vars")
	}
	p := c.Usenet.Providers[0]
	if p.Host != "news.example.com" {
		t.Errorf("Host = %q, want news.example.com", p.Host)
	}
	if p.Port != 563 {
		t.Errorf("Port = %d, want 563", p.Port)
	}
	if p.Username != "user" {
		t.Errorf("Username = %q, want user", p.Username)
	}
	if p.Password != "pass" {
		t.Errorf("Password = %q, want pass", p.Password)
	}
	if p.MaxConnections != 30 {
		t.Errorf("MaxConnections = %d, want 30", p.MaxConnections)
	}
	if !p.SSL {
		t.Error("SSL should be true")
	}
	if p.Priority != 2 {
		t.Errorf("Priority = %d, want 2", p.Priority)
	}
}

// ── updateUsenetProvider ──────────────────────────────────────────────────────

func TestUpdateUsenetProvider_Defaults(t *testing.T) {
	c := &Config{}
	p := c.updateUsenetProvider(0, UsenetProvider{})

	if p.Port != 119 {
		t.Errorf("Port = %d, want 119 (default NNTP port)", p.Port)
	}
	if p.MaxConnections != 20 {
		t.Errorf("MaxConnections = %d, want 20 (default)", p.MaxConnections)
	}
	if p.Priority != 1 {
		t.Errorf("Priority = %d, want 1 (index+1 for index 0)", p.Priority)
	}
}

func TestUpdateUsenetProvider_PreservesExisting(t *testing.T) {
	c := &Config{}
	p := c.updateUsenetProvider(2, UsenetProvider{Port: 563, MaxConnections: 10, Priority: 5})

	if p.Port != 563 {
		t.Errorf("Port should not be overwritten, got %d", p.Port)
	}
	if p.MaxConnections != 10 {
		t.Errorf("MaxConnections should not be overwritten, got %d", p.MaxConnections)
	}
	if p.Priority != 5 {
		t.Errorf("Priority should not be overwritten, got %d", p.Priority)
	}
}
