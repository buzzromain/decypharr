package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_BootstrapCreatesPredictableDefaults(t *testing.T) {
	base := t.TempDir()
	SetConfigPath(base)
	c := &Config{}

	if err := c.loadConfig(); err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(base, "config.json")); err != nil {
		t.Fatalf("config.json not created: %v", err)
	}
	if c.Port != DefaultPort {
		t.Fatalf("port = %q, want %q", c.Port, DefaultPort)
	}
	if c.URLBase != "/" {
		t.Fatalf("url_base = %q, want /", c.URLBase)
	}
	if !c.UseAuth {
		t.Fatal("use_auth = false, want true on first bootstrap")
	}
}

func TestLoadConfig_PartialAndEnvOverride(t *testing.T) {
	base := t.TempDir()
	SetConfigPath(base)

	json := `{"url_base":"decy","download_folder":"/d","debrids":[{"name":"realdebrid","provider":"realdebrid","api_key":"k"}]}`
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(json), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("DECYPHARR_PORT", "9191")

	c := &Config{}
	if err := c.loadConfig(); err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if c.URLBase != "/decy/" {
		t.Fatalf("normalized URLBase = %q, want /decy/", c.URLBase)
	}
	if c.Port != "9191" {
		t.Fatalf("port override = %q, want 9191", c.Port)
	}
	if c.DefaultDownloadAction == "" {
		t.Fatal("default download action not set")
	}
	if len(c.AllowedExt) == 0 {
		t.Fatal("allowed extensions defaults not applied")
	}
}

func TestGetAuth_InvalidSecretFileKeepsNeedsAuthTrue(t *testing.T) {
	base := t.TempDir()
	SetConfigPath(base)
	c := &Config{UseAuth: true}

	if err := os.WriteFile(filepath.Join(base, "auth.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	a := c.GetAuth()
	if a == nil {
		t.Fatal("GetAuth() returned nil while auth enabled")
	}
	if !c.NeedsAuth() {
		t.Fatal("NeedsAuth() = false, want true for invalid/empty auth")
	}
}

func TestValidate_ReturnsActionableErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing download folder",
			cfg:     Config{Debrids: []Debrid{{Name: "realdebrid", Provider: "realdebrid", APIKey: "k"}}},
			wantErr: "download folder is required",
		},
		{
			name:    "no providers configured",
			cfg:     Config{DownloadFolder: "/downloads"},
			wantErr: "at least one debrid provider or usenet provider must be configured",
		},
		{
			name:    "debrid secret missing",
			cfg:     Config{DownloadFolder: "/downloads", Debrids: []Debrid{{Name: "realdebrid", Provider: "realdebrid"}}},
			wantErr: "debrid api key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}
