package config

import (
	"testing"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"", false},
		{"True", false},
		{"TRUE", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseBool(tt.input)
			if got != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestApplyEnvOverrides_RootFields(t *testing.T) {
	t.Run("PORT sets c.Port", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_PORT", "9090")
		c.applyEnvOverrides()
		if c.Port != "9090" {
			t.Errorf("Port = %q, want %q", c.Port, "9090")
		}
	})

	t.Run("BIND_ADDRESS sets c.BindAddress", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_BIND_ADDRESS", "0.0.0.0")
		c.applyEnvOverrides()
		if c.BindAddress != "0.0.0.0" {
			t.Errorf("BindAddress = %q, want %q", c.BindAddress, "0.0.0.0")
		}
	})

	t.Run("URL_BASE sets c.URLBase", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_URL_BASE", "/decypharr")
		c.applyEnvOverrides()
		if c.URLBase != "/decypharr" {
			t.Errorf("URLBase = %q, want %q", c.URLBase, "/decypharr")
		}
	})

	t.Run("LOG_LEVEL sets c.LogLevel", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_LOG_LEVEL", "debug")
		c.applyEnvOverrides()
		if c.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want %q", c.LogLevel, "debug")
		}
	})

	t.Run("USE_AUTH=true sets c.UseAuth=true", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_USE_AUTH", "true")
		c.applyEnvOverrides()
		if !c.UseAuth {
			t.Errorf("UseAuth = false, want true")
		}
	})

	t.Run("USE_AUTH=false sets c.UseAuth=false", func(t *testing.T) {
		c := &Config{UseAuth: true}
		t.Setenv("DECYPHARR_USE_AUTH", "false")
		c.applyEnvOverrides()
		if c.UseAuth {
			t.Errorf("UseAuth = true, want false")
		}
	})

	t.Run("DOWNLOAD_FOLDER sets c.DownloadFolder", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_DOWNLOAD_FOLDER", "/data/downloads")
		c.applyEnvOverrides()
		if c.DownloadFolder != "/data/downloads" {
			t.Errorf("DownloadFolder = %q, want %q", c.DownloadFolder, "/data/downloads")
		}
	})

	t.Run("MAX_DOWNLOADS=5 sets c.MaxDownloads=5", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_MAX_DOWNLOADS", "5")
		c.applyEnvOverrides()
		if c.MaxDownloads != 5 {
			t.Errorf("MaxDownloads = %d, want %d", c.MaxDownloads, 5)
		}
	})

	t.Run("MAX_DOWNLOADS=notanumber no change", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_MAX_DOWNLOADS", "notanumber")
		c.applyEnvOverrides()
		if c.MaxDownloads != 0 {
			t.Errorf("MaxDownloads = %d, want %d", c.MaxDownloads, 0)
		}
	})

	t.Run("SKIP_PRE_CACHE=1 sets c.SkipPreCache=true", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_SKIP_PRE_CACHE", "1")
		c.applyEnvOverrides()
		if !c.SkipPreCache {
			t.Errorf("SkipPreCache = false, want true")
		}
	})

	t.Run("ALWAYS_RM_TRACKER_URLS=yes sets c.AlwaysRmTrackerUrls=true", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_ALWAYS_RM_TRACKER_URLS", "yes")
		c.applyEnvOverrides()
		if !c.AlwaysRmTrackerUrls {
			t.Errorf("AlwaysRmTrackerUrls = false, want true")
		}
	})

	t.Run("RETRIES=3 sets c.Retries=3", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_RETRIES", "3")
		c.applyEnvOverrides()
		if c.Retries != 3 {
			t.Errorf("Retries = %d, want %d", c.Retries, 3)
		}
	})

	t.Run("MANAGED_ONLY=true sets c.ManagedOnly=true", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_MANAGED_ONLY", "true")
		c.applyEnvOverrides()
		if !c.ManagedOnly {
			t.Errorf("ManagedOnly = false, want true")
		}
	})
}

func TestApplyEnvOverrides_NoEnvVars(t *testing.T) {
	c := &Config{}
	c.applyEnvOverrides()

	if c.Port != "" {
		t.Errorf("Port = %q, want empty", c.Port)
	}
	if c.BindAddress != "" {
		t.Errorf("BindAddress = %q, want empty", c.BindAddress)
	}
	if c.URLBase != "" {
		t.Errorf("URLBase = %q, want empty", c.URLBase)
	}
	if c.LogLevel != "" {
		t.Errorf("LogLevel = %q, want empty", c.LogLevel)
	}
	if c.UseAuth {
		t.Errorf("UseAuth = true, want false")
	}
	if c.DownloadFolder != "" {
		t.Errorf("DownloadFolder = %q, want empty", c.DownloadFolder)
	}
	if c.MaxDownloads != 0 {
		t.Errorf("MaxDownloads = %d, want 0", c.MaxDownloads)
	}
	if c.SkipPreCache {
		t.Errorf("SkipPreCache = true, want false")
	}
	if c.AlwaysRmTrackerUrls {
		t.Errorf("AlwaysRmTrackerUrls = true, want false")
	}
	if c.Retries != 0 {
		t.Errorf("Retries = %d, want 0", c.Retries)
	}
	if c.ManagedOnly {
		t.Errorf("ManagedOnly = true, want false")
	}
}

func TestApplyEnvOverrides_CategoriesArray(t *testing.T) {
	t.Run("two categories set", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_CATEGORIES__0", "sonarr")
		t.Setenv("DECYPHARR_CATEGORIES__1", "radarr")
		c.applyEnvOverrides()

		if len(c.Categories) != 2 {
			t.Fatalf("len(Categories) = %d, want 2", len(c.Categories))
		}
		if c.Categories[0] != "sonarr" {
			t.Errorf("Categories[0] = %q, want %q", c.Categories[0], "sonarr")
		}
		if c.Categories[1] != "radarr" {
			t.Errorf("Categories[1] = %q, want %q", c.Categories[1], "radarr")
		}
	})

	t.Run("gap in index stops at first missing", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_CATEGORIES__0", "sonarr")
		// CATEGORIES__1 intentionally not set
		c.applyEnvOverrides()

		if len(c.Categories) != 1 {
			t.Fatalf("len(Categories) = %d, want 1", len(c.Categories))
		}
		if c.Categories[0] != "sonarr" {
			t.Errorf("Categories[0] = %q, want %q", c.Categories[0], "sonarr")
		}
	})
}

func TestApplyEnvOverrides_ArrsArray(t *testing.T) {
	t.Run("arr fields set correctly", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_ARRS__0__NAME", "sonarr")
		t.Setenv("DECYPHARR_ARRS__0__HOST", "http://sonarr:8989")
		t.Setenv("DECYPHARR_ARRS__0__TOKEN", "abc")
		c.applyEnvOverrides()

		if len(c.Arrs) == 0 {
			t.Fatal("Arrs is empty, want at least 1 element")
		}
		if c.Arrs[0].Name != "sonarr" {
			t.Errorf("Arrs[0].Name = %q, want %q", c.Arrs[0].Name, "sonarr")
		}
		if c.Arrs[0].Host != "http://sonarr:8989" {
			t.Errorf("Arrs[0].Host = %q, want %q", c.Arrs[0].Host, "http://sonarr:8989")
		}
		if c.Arrs[0].Token != "abc" {
			t.Errorf("Arrs[0].Token = %q, want %q", c.Arrs[0].Token, "abc")
		}
	})

	t.Run("CLEANUP=true sets Arrs[0].Cleanup=true", func(t *testing.T) {
		c := &Config{}
		t.Setenv("DECYPHARR_ARRS__0__NAME", "radarr")
		t.Setenv("DECYPHARR_ARRS__0__CLEANUP", "true")
		c.applyEnvOverrides()

		if len(c.Arrs) == 0 {
			t.Fatal("Arrs is empty, want at least 1 element")
		}
		if !c.Arrs[0].Cleanup {
			t.Errorf("Arrs[0].Cleanup = false, want true")
		}
	})
}
