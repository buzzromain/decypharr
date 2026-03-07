package config

import (
	"testing"
)

func TestParseSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"100MB", 100 * 1024 * 1024, false},
		{"1GB", 1 * 1024 * 1024 * 1024, false},
		{"500KB", 500 * 1024, false},
		{"1.5GB", int64(1.5 * 1024 * 1024 * 1024), false},
		{"2048MB", 2048 * 1024 * 1024, false},
		{"100mb", 100 * 1024 * 1024, false}, // lowercase
		{"", 0, true},
		{"invalid", 0, true},
		{"100XB", 0, true}, // unknown unit treated as number
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetMinFileSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		minFileSize string
		want        int64
	}{
		{"", 0},
		{"100MB", 100 * 1024 * 1024},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.minFileSize, func(t *testing.T) {
			t.Parallel()
			c := &Config{MinFileSize: tt.minFileSize}
			got := c.GetMinFileSize()
			if got != tt.want {
				t.Errorf("GetMinFileSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetMaxFileSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		maxFileSize string
		want        int64
	}{
		{"", 0},
		{"4GB", 4 * 1024 * 1024 * 1024},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.maxFileSize, func(t *testing.T) {
			t.Parallel()
			c := &Config{MaxFileSize: tt.maxFileSize}
			got := c.GetMaxFileSize()
			if got != tt.want {
				t.Errorf("GetMaxFileSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsSample(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		// HasSuffix("sample.mkv") special case
		{"sample.mkv", true},
		{"Movie.sample.mkv", true},
		// Normal files
		{"Movie.mkv", false},
		{"Movie (2023).mkv", false},
		// Pattern 1: keyword at start followed by [-/]
		{"trailer-x.mkv", true},
		{"special-edition.mkv", true},
		{"extras-folder.mkv", true},
		// Pattern 1 requires [-/] after keyword — bare keyword without it doesn't match
		{"trailer.mkv", false},
		{"thumb.jpg", false},
		// Pattern 2: keyword in parentheses
		{"(sample).mkv", true},
		{"(trailer).mkv", true},
		// Pattern 3: keyword preceded by hyphen
		{"movie-sample.mkv", true},
		{"movie-trailer.mkv", true},
		// filepath.Base strips directory — directory "sample" doesn't make the file a sample
		{"/path/to/sample/video.mkv", false},
		{"/path/to/extras/video.mkv", false},
		// "extras" in filename body without [-/] after doesn't match pattern 1
		{"movie.extras.mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := isSample(tt.path)
			if got != tt.want {
				t.Errorf("isSample(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsFileAllowed_Extension(t *testing.T) {
	t.Parallel()
	c := &Config{
		AllowedExt:  []string{"mkv", "mp4", "avi"},
		AllowSamples: true,
	}

	if err := c.IsFileAllowed("movie.mkv", 100); err != nil {
		t.Errorf("expected mkv to be allowed, got: %v", err)
	}
	if err := c.IsFileAllowed("movie.mp4", 100); err != nil {
		t.Errorf("expected mp4 to be allowed, got: %v", err)
	}
	if err := c.IsFileAllowed("movie.txt", 100); err == nil {
		t.Error("expected txt to be disallowed")
	}
	if err := c.IsFileAllowed("movie", 100); err == nil {
		t.Error("expected file without extension to be disallowed")
	}
}

func TestIsFileAllowed_SampleFilter(t *testing.T) {
	t.Parallel()
	c := &Config{
		AllowedExt:  []string{"mkv"},
		AllowSamples: false,
	}

	if err := c.IsFileAllowed("sample.mkv", 100); err == nil {
		t.Error("expected sample to be disallowed when AllowSamples=false")
	}

	c.AllowSamples = true
	if err := c.IsFileAllowed("sample.mkv", 100); err != nil {
		t.Errorf("expected sample to be allowed when AllowSamples=true, got: %v", err)
	}
}

func TestIsFileAllowed_SizeFilter(t *testing.T) {
	t.Parallel()
	c := &Config{
		AllowedExt:  []string{"mkv"},
		AllowSamples: true,
		MinFileSize:  "100MB",
		MaxFileSize:  "10GB",
	}

	const MB = 1024 * 1024
	const GB = 1024 * MB

	// Below min size
	if err := c.IsFileAllowed("movie.mkv", 50*MB); err == nil {
		t.Error("expected file below min size to be disallowed")
	}

	// Within range
	if err := c.IsFileAllowed("movie.mkv", 500*MB); err != nil {
		t.Errorf("expected 500MB file to be allowed, got: %v", err)
	}

	// Above max size
	if err := c.IsFileAllowed("movie.mkv", 20*GB); err == nil {
		t.Error("expected file above max size to be disallowed")
	}
}

func TestIsNameAllowed(t *testing.T) {
	t.Parallel()
	c := &Config{AllowedExt: []string{"mkv", "mp4", "avi"}}

	tests := []struct {
		filename string
		want     bool
	}{
		{"movie.mkv", true},
		{"MOVIE.MKV", true}, // case insensitive
		{"movie.mp4", true},
		{"movie.txt", false},
		{"movie", false},    // no extension
		{".mkv", true},     // just extension (valid)
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()
			got := c.isNameAllowed(tt.filename)
			if got != tt.want {
				t.Errorf("isNameAllowed(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestMigrateQBitTorrentToManager(t *testing.T) {
	t.Parallel()
	c := &Config{
		QBitTorrent: QBitTorrent{
			DownloadFolder:      "/downloads",
			Categories:          []string{"sonarr", "radarr"},
			RefreshInterval:     60,
			SkipPreCache:        true,
			MaxDownloads:        5,
			AlwaysRmTrackerUrls: true,
		},
	}

	c.migrateQBitTorrentToManager()

	if c.DownloadFolder != "/downloads" {
		t.Errorf("expected DownloadFolder '/downloads', got '%s'", c.DownloadFolder)
	}
	if len(c.Categories) != 2 || c.Categories[0] != "sonarr" {
		t.Errorf("expected categories [sonarr radarr], got %v", c.Categories)
	}
	if c.RefreshInterval != "60s" {
		t.Errorf("expected RefreshInterval '60s', got '%s'", c.RefreshInterval)
	}
	if !c.SkipPreCache {
		t.Error("expected SkipPreCache=true")
	}
	if c.MaxDownloads != 5 {
		t.Errorf("expected MaxDownloads=5, got %d", c.MaxDownloads)
	}
	if !c.AlwaysRmTrackerUrls {
		t.Error("expected AlwaysRmTrackerUrls=true")
	}
}

func TestMigrateQBitTorrentToManager_DoesNotOverwrite(t *testing.T) {
	t.Parallel()
	// Manager fields already set — should not be overwritten
	c := &Config{
		DownloadFolder: "/existing-downloads",
		Categories:     []string{"lidarr"},
		QBitTorrent: QBitTorrent{
			DownloadFolder: "/old-downloads",
			Categories:     []string{"sonarr"},
		},
	}

	c.migrateQBitTorrentToManager()

	if c.DownloadFolder != "/existing-downloads" {
		t.Errorf("expected existing DownloadFolder to not be overwritten, got '%s'", c.DownloadFolder)
	}
	if len(c.Categories) != 1 || c.Categories[0] != "lidarr" {
		t.Errorf("expected existing categories to not be overwritten, got %v", c.Categories)
	}
}

func TestMigrateNotifications(t *testing.T) {
	t.Parallel()
	c := &Config{
		DiscordWebhook: "https://discord.com/webhook/xxx",
		CallbackURL:    "https://callback.example.com",
	}

	c.migrateNotifications()

	if c.Notifications.WebhookURL != "https://discord.com/webhook/xxx" {
		t.Errorf("expected WebhookURL to be migrated, got '%s'", c.Notifications.WebhookURL)
	}
	if c.Notifications.CallbackURL != "https://callback.example.com" {
		t.Errorf("expected CallbackURL to be migrated, got '%s'", c.Notifications.CallbackURL)
	}
	if !c.Notifications.Enabled {
		t.Error("expected Notifications.Enabled=true after migration")
	}
}

func TestMigrateNotifications_DoesNotOverwrite(t *testing.T) {
	t.Parallel()
	c := &Config{
		DiscordWebhook: "https://old-webhook.com",
		Notifications: Notifications{
			WebhookURL: "https://existing-webhook.com",
		},
	}

	c.migrateNotifications()

	if c.Notifications.WebhookURL != "https://existing-webhook.com" {
		t.Errorf("expected existing WebhookURL to not be overwritten, got '%s'", c.Notifications.WebhookURL)
	}
}
