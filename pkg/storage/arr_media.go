package storage

import (
	"fmt"
	json "github.com/bytedance/sonic"
	"path/filepath"
	"strings"
	"time"
)

// arrMediaVersion is bumped whenever stored records need a forced re-sync.
// v1 → v2: added movie/series metadata fields; old records lack metadata,
// so we clear the last-event sentinels to trigger a full re-sync from epoch.
const arrMediaVersion = "2"

// ArrMedia tracks a managed path (Sonarr/Radarr library file) and its Decypharr source,
// enriched with media metadata populated at import time.
type ArrMedia struct {
	ArrName     string `json:"arr_name"`
	ManagedPath string `json:"managed_path"` // store key (normalized lowercase)
	InfoHash    string `json:"info_hash"`
	FileName    string `json:"file_name"`

	Title        string   `json:"title,omitempty"`
	Year         int      `json:"year,omitempty"`
	MediaType    string   `json:"media_type,omitempty"` // "movie" | "show"
	Poster       string   `json:"poster,omitempty"`     // remoteUrl TMDB public
	TmdbId       int      `json:"tmdb_id,omitempty"`
	ImdbId       string   `json:"imdb_id,omitempty"`
	Overview     string   `json:"overview,omitempty"`
	Genres       []string `json:"genres,omitempty"`
	Quality      string   `json:"quality,omitempty"`
	ReleaseGroup string   `json:"release_group,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
}

// normalizePath returns a canonical, lowercase, cleaned path suitable as a store key.
func normalizePath(p string) string {
	return filepath.Clean(strings.ToLower(p))
}

// initArrFilesStore writes or migrates the version sentinel.
// On a version mismatch the last-event sentinels are cleared so syncArrMedia
// re-fetches history from epoch and enriches existing records with metadata.
func (s *Storage) initArrFilesStore() {
	data, _ := s.arrFiles.Get("__version__")
	if string(data) == arrMediaVersion {
		return
	}
	// Collect stale last-event keys before deleting to avoid mutating during iteration.
	var stale []string
	_ = s.arrFiles.ForEach(func(key string, _ []byte) error {
		if strings.HasPrefix(key, "__last_event__") {
			stale = append(stale, key)
		}
		return nil
	})
	for _, key := range stale {
		_ = s.arrFiles.Delete(key)
	}
	_ = s.arrFiles.Put("__version__", []byte(arrMediaVersion), nil)
}

// UpsertArrMedia stores or updates an ArrMedia record keyed by ref.ManagedPath.
func (s *Storage) UpsertArrMedia(ref *ArrMedia) error {
	if ref == nil {
		return fmt.Errorf("ref is nil")
	}
	ref.UpdatedAt = time.Now()
	key := normalizePath(ref.ManagedPath)
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	return s.arrFiles.Put(key, data, nil)
}

// GetArrMedia retrieves an ArrMedia record by its managed path. Returns nil, nil when not found.
func (s *Storage) GetArrMedia(managedPath string) (*ArrMedia, error) {
	key := normalizePath(managedPath)
	data, err := s.arrFiles.Get(key)
	if err != nil {
		return nil, nil
	}
	var ref ArrMedia
	if err := json.Unmarshal(data, &ref); err != nil {
		return nil, err
	}
	return &ref, nil
}

// DeleteArrMedia removes the ArrMedia record for managedPath and returns it (nil if absent).
func (s *Storage) DeleteArrMedia(managedPath string) (*ArrMedia, error) {
	ref, err := s.GetArrMedia(managedPath)
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, nil
	}
	key := normalizePath(managedPath)
	if err := s.arrFiles.Delete(key); err != nil {
		return nil, err
	}
	return ref, nil
}

// FindArrMediaByInfoHash returns all ArrMedia records that share the same infohash.
func (s *Storage) FindArrMediaByInfoHash(infohash string) ([]ArrMedia, error) {
	var refs []ArrMedia
	_ = s.arrFiles.ForEach(func(_ string, value []byte) error {
		var ref ArrMedia
		if err := json.Unmarshal(value, &ref); err != nil {
			return nil
		}
		if strings.EqualFold(ref.InfoHash, infohash) {
			refs = append(refs, ref)
		}
		return nil
	})
	return refs, nil
}

// ReferencedInfoHashes returns every infohash an ArrMedia record still points at.
// One pass over the index: callers use it to spot entries no arr references
// any more, without paying a lookup per entry.
func (s *Storage) ReferencedInfoHashes() (map[string]struct{}, error) {
	referenced := make(map[string]struct{})
	err := s.arrFiles.ForEach(func(_ string, value []byte) error {
		var ref ArrMedia
		if err := json.Unmarshal(value, &ref); err != nil {
			return nil
		}
		if ref.InfoHash != "" {
			referenced[strings.ToLower(ref.InfoHash)] = struct{}{}
		}
		return nil
	})
	return referenced, err
}

// GetArrMediaLastEventDate returns the date of the last processed history event for the given ARR.
func (s *Storage) GetArrMediaLastEventDate(arrName string) (time.Time, bool) {
	key := "__last_event__" + strings.ToLower(arrName)
	data, err := s.arrFiles.Get(key)
	if err != nil || len(data) == 0 {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, string(data))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// SetArrMediaLastEventDate stores the date of the last processed history event for the given ARR.
func (s *Storage) SetArrMediaLastEventDate(arrName string, t time.Time) error {
	key := "__last_event__" + strings.ToLower(arrName)
	return s.arrFiles.Put(key, []byte(t.UTC().Format(time.RFC3339Nano)), nil)
}

// FindArrMediaByFolder returns all ArrMedia records for arrName whose ManagedPath is nested
// under folderPath. Scoped to arrName so a SeriesDelete/MovieDelete from one ARR can't sweep
// up another ARR's tracked media when their library roots overlap.
func (s *Storage) FindArrMediaByFolder(arrName, folderPath string) ([]ArrMedia, error) {
	prefix := normalizePath(folderPath) + string(filepath.Separator)
	var refs []ArrMedia
	_ = s.arrFiles.ForEach(func(_ string, value []byte) error {
		var ref ArrMedia
		if err := json.Unmarshal(value, &ref); err != nil {
			return nil
		}
		if ref.ArrName == arrName && strings.HasPrefix(normalizePath(ref.ManagedPath), prefix) {
			refs = append(refs, ref)
		}
		return nil
	})
	return refs, nil
}
