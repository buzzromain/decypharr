package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidUsenetPartialAppliesDefaultsAndValidates(t *testing.T) {
	base := t.TempDir()
	SetConfigPath(base)

	json := `{
		"download_folder":"/downloads",
		"usenet":{
			"providers":[{"host":"news.example.com","username":"u","password":"p"}],
			"availability_sample_percent":150
		}
	}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(json), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c := &Config{}
	if err := c.loadConfig(); err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if c.Usenet.MaxConnections != 15 {
		t.Fatalf("Usenet.MaxConnections = %d, want 15 default", c.Usenet.MaxConnections)
	}
	if c.Usenet.ReadAhead != "16MB" {
		t.Fatalf("Usenet.ReadAhead = %q, want 16MB default", c.Usenet.ReadAhead)
	}
	if c.Usenet.ProcessingTimeout != "10m" {
		t.Fatalf("Usenet.ProcessingTimeout = %q, want 10m default", c.Usenet.ProcessingTimeout)
	}
	if c.Usenet.AvailabilitySamplePercent != 100 {
		t.Fatalf("Usenet.AvailabilitySamplePercent = %d, want clamped 100", c.Usenet.AvailabilitySamplePercent)
	}

	if len(c.Usenet.Providers) != 1 {
		t.Fatalf("len(Usenet.Providers) = %d, want 1", len(c.Usenet.Providers))
	}
	p := c.Usenet.Providers[0]
	if p.Port != 119 {
		t.Fatalf("provider port = %d, want default 119", p.Port)
	}
	if p.MaxConnections != 20 {
		t.Fatalf("provider max_connections = %d, want default 20", p.MaxConnections)
	}
	if p.Priority != 1 {
		t.Fatalf("provider priority = %d, want default 1", p.Priority)
	}

	wantDiskPath := filepath.Join(base, "usenet", "streams")
	if c.Usenet.DiskBufferPath != wantDiskPath {
		t.Fatalf("Usenet.DiskBufferPath = %q, want %q", c.Usenet.DiskBufferPath, wantDiskPath)
	}
}

func TestLoadConfig_InvalidJSONReturnsError(t *testing.T) {
	base := t.TempDir()
	SetConfigPath(base)

	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(`{"download_folder":`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c := &Config{}
	if err := c.loadConfig(); err == nil {
		t.Fatal("loadConfig() expected error for invalid JSON, got nil")
	}
}

func TestLoadConfig_PartialUsenetCredentialsFromEnvFailsValidation(t *testing.T) {
	base := t.TempDir()
	SetConfigPath(base)

	json := `{"download_folder":"/downloads"}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(json), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("DECYPHARR_USENET__PROVIDERS__0__HOST", "news.example.com")

	c := &Config{}
	if err := c.loadConfig(); err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() expected missing usenet credentials error, got nil")
	}
}
