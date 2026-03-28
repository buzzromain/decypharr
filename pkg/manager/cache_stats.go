package manager

import "time"

// CacheFileStat holds per-file VFS cache statistics.
// Defined here (not in pkg/mount/dfs/vfs) to avoid a circular import:
// vfs already imports pkg/manager, so the type lives alongside MountManager.
type CacheFileStat struct {
	Hash         string    `json:"hash"`
	Filename     string    `json:"filename"`
	Size         int64     `json:"size"`
	CachedBytes  int64     `json:"cached_bytes"`
	CachePercent float64   `json:"cache_percent"`
	LastAccess   time.Time `json:"last_access"`
}

// GetCacheFiles returns per-file cache stats from the active mount backend.
// hash="" returns all files; non-empty hash filters to that torrent only.
func (m *Manager) GetCacheFiles(hash string) []CacheFileStat {
	if m.mountManager == nil {
		return nil
	}
	files := m.mountManager.GetCacheFiles()
	if hash == "" {
		return files
	}
	var result []CacheFileStat
	for _, f := range files {
		if f.Hash == hash {
			result = append(result, f)
		}
	}
	return result
}

// GetMountStats exposes Stats() from the active mount backend.
func (m *Manager) GetMountStats() map[string]interface{} {
	if m.mountManager == nil {
		return nil
	}
	return m.mountManager.Stats()
}
