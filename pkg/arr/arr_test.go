package arr

import (
	"os"
	"sync"
	"testing"

	"github.com/sirrobot01/decypharr/internal/testutil"
)

func TestMain(m *testing.M) {
	_, cleanup := testutil.SetupTestConfigDir()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestInferType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		host string
		name string
		want Type
	}{
		{"http://sonarr:8989", "", Sonarr},
		{"http://radarr:7878", "", Radarr},
		{"http://lidarr:8686", "", Lidarr},
		{"http://readarr:8787", "", Readarr},
		{"http://myserver:8989", "sonarr", Sonarr},
		{"http://myserver:7878", "radarr", Radarr},
		{"http://myserver:8686", "lidarr", Lidarr},
		{"http://myserver:8787", "readarr", Readarr},
		{"http://other:9090", "myclient", Others},
		{"", "", Others},
	}

	for _, tt := range tests {
		t.Run(tt.host+"|"+tt.name, func(t *testing.T) {
			t.Parallel()
			got := inferType(tt.host, tt.name)
			if got != tt.want {
				t.Errorf("inferType(%q, %q) = %q, want %q", tt.host, tt.name, got, tt.want)
			}
		})
	}
}

func TestNew_SetsFields(t *testing.T) {
	t.Parallel()
	dlUncached := true
	a := New("sonarr", "http://sonarr:8989", "  mytoken  ", true, false, &dlUncached, "realdebrid", "auto")

	if a.Name != "sonarr" {
		t.Errorf("expected name 'sonarr', got '%s'", a.Name)
	}
	if a.Host != "http://sonarr:8989" {
		t.Errorf("expected host 'http://sonarr:8989', got '%s'", a.Host)
	}
	// Token should be trimmed
	if a.Token != "mytoken" {
		t.Errorf("expected trimmed token 'mytoken', got '%s'", a.Token)
	}
	if a.Type != Sonarr {
		t.Errorf("expected type Sonarr, got %s", a.Type)
	}
	if !a.Cleanup {
		t.Error("expected Cleanup=true")
	}
	if a.SkipRepair {
		t.Error("expected SkipRepair=false")
	}
	if a.DownloadUncached == nil || !*a.DownloadUncached {
		t.Error("expected DownloadUncached=true")
	}
	if a.SelectedDebrid != "realdebrid" {
		t.Errorf("expected SelectedDebrid 'realdebrid', got '%s'", a.SelectedDebrid)
	}
	if a.Source != SourceAuto {
		t.Errorf("expected source 'auto', got '%s'", a.Source)
	}
}

func TestNew_InfersTypeFromName(t *testing.T) {
	t.Parallel()
	a := New("radarr", "http://localhost:7878", "token", false, false, nil, "", "manual")
	if a.Type != Radarr {
		t.Errorf("expected Radarr from name, got %s", a.Type)
	}
}

func TestStorage_AddOrUpdate(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	a := &Arr{Name: "sonarr", Host: "http://sonarr:8989", Token: "apikey"}
	s.AddOrUpdate(a)

	got := s.Get("sonarr")
	if got == nil {
		t.Fatal("expected to get arr 'sonarr'")
	}
	if got.Host != "http://sonarr:8989" {
		t.Errorf("expected host 'http://sonarr:8989', got '%s'", got.Host)
	}
}

func TestStorage_AddOrUpdate_SkipsInvalidURL(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	a := &Arr{Name: "bad", Host: "not-a-url", Token: "token"}
	s.AddOrUpdate(a)

	// Should be skipped due to invalid URL
	if got := s.Get("bad"); got != nil {
		t.Errorf("expected nil for invalid URL arr, got %+v", got)
	}
}

func TestStorage_AddOrUpdate_SkipsEmptyFields(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	// Empty name
	s.AddOrUpdate(&Arr{Host: "http://sonarr:8989", Token: "token"})
	// Empty host
	s.AddOrUpdate(&Arr{Name: "test", Token: "token"})
	// Empty token
	s.AddOrUpdate(&Arr{Name: "test", Host: "http://sonarr:8989"})

	if got := s.Get("test"); got != nil {
		t.Errorf("expected nil for incomplete arr, got %+v", got)
	}
}

func TestStorage_Get(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	s.AddOrUpdate(&Arr{Name: "radarr", Host: "http://radarr:7878", Token: "token"})

	got := s.Get("radarr")
	if got == nil {
		t.Fatal("expected arr")
	}

	// Missing key returns nil
	if got := s.Get("nonexistent"); got != nil {
		t.Errorf("expected nil for missing arr, got %+v", got)
	}
}

func TestStorage_GetAll(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	s.AddOrUpdate(&Arr{Name: "sonarr", Host: "http://sonarr:8989", Token: "t1"})
	s.AddOrUpdate(&Arr{Name: "radarr", Host: "http://radarr:7878", Token: "t2"})

	all := s.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 arrs, got %d", len(all))
	}
}

func TestStorage_GetOrCreate_Existing(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	s.AddOrUpdate(&Arr{Name: "sonarr", Host: "http://sonarr:8989", Token: "token"})

	got := s.GetOrCreate("sonarr")
	if got == nil {
		t.Fatal("expected arr")
	}
	if got.Host != "http://sonarr:8989" {
		t.Errorf("expected stored host, got '%s'", got.Host)
	}
}

func TestStorage_GetOrCreate_New(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	got := s.GetOrCreate("newclient")
	if got == nil {
		t.Fatal("expected new arr")
	}
	if got.Name != "newclient" {
		t.Errorf("expected name 'newclient', got '%s'", got.Name)
	}
	if got.Source != SourceManual {
		t.Errorf("expected source 'manual', got '%s'", got.Source)
	}
}

func TestStorage_GetOrCreate_EmptyName(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	got := s.GetOrCreate("")
	if got == nil {
		t.Fatal("expected arr")
	}
	if got.Name != "uncategorized" {
		t.Errorf("expected name 'uncategorized' for empty name, got '%s'", got.Name)
	}
}

func TestStorage_ConcurrentAccess(t *testing.T) {
	// Not parallel: uses config.Get()
	s := NewStorage()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "arr" + string(rune('a'+i%26))
			host := "http://sonarr-" + string(rune('a'+i%26)) + ":8989"
			s.AddOrUpdate(&Arr{Name: name, Host: host, Token: "token"})
			s.Get(name)
			s.GetAll()
		}(i)
	}
	wg.Wait()
}
