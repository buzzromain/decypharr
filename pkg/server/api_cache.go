package server

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sirrobot01/decypharr/internal/utils"
)

// handleGetCacheStats serves /api/cache/stats — sidebar summary.
func (s *Server) handleGetCacheStats(w http.ResponseWriter, r *http.Request) {
	stats := s.manager.GetMountStats()

	getInt64 := func(key string) int64 {
		if stats == nil {
			return 0
		}
		if v, ok := stats[key]; ok {
			switch val := v.(type) {
			case int64:
				return val
			case int:
				return int64(val)
			}
		}
		return 0
	}

	getBool := func(key string) bool {
		if stats == nil {
			return false
		}
		if v, ok := stats[key]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
		return false
	}

	result := map[string]interface{}{
		"cache_total_size": getInt64("cache_total_size"),
		"cache_max_size":   getInt64("cache_max_size"),
		"cache_item_count": getInt64("cache_item_count"),
		"mount_ready":      getBool("ready"),
	}
	utils.JSONResponse(w, result, http.StatusOK)
}

// handleGetCacheFiles serves /api/cache/files — full file list.
func (s *Server) handleGetCacheFiles(w http.ResponseWriter, r *http.Request) {
	files := s.manager.GetCacheFiles("")
	utils.JSONResponse(w, files, http.StatusOK)
}

// handleGetCacheFilesByHash serves /api/cache/files/{hash} — MediaDetail.
func (s *Server) handleGetCacheFilesByHash(w http.ResponseWriter, r *http.Request) {
	hash := strings.ToLower(chi.URLParam(r, "hash"))
	files := s.manager.GetCacheFiles(hash)
	utils.JSONResponse(w, files, http.StatusOK)
}
