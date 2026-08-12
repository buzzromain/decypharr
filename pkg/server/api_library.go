package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

type LibraryItem struct {
	Hash         string    `json:"hash"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Status       string    `json:"status"`   // "debrid" | "error" (+ "local" when CopyEntry is implemented)
	Protocol     string    `json:"protocol"` // "torrent" | "nzb"
	Category     string    `json:"category"`
	ArrName      string    `json:"arr_name"`
	MediaType    string    `json:"media_type"` // "movie" | "show" | ""
	Title        string    `json:"title"`
	Year         int       `json:"year"`
	Poster       string    `json:"poster"`
	TmdbId       int       `json:"tmdb_id"`
	Overview     string    `json:"overview"`
	Genres       []string  `json:"genres"`
	Quality      string    `json:"quality"`
	ReleaseGroup string    `json:"release_group"`
	AddedOn      time.Time `json:"added_on"`
}

func entryStatus(e *storage.Entry) string {
	if e.State == storage.EntryStateError {
		return "error"
	}
	return "debrid"
}

func buildLibraryItem(entry *storage.Entry, media *storage.ArrMedia) LibraryItem {
	item := LibraryItem{
		Hash:     entry.InfoHash,
		Name:     entry.Name,
		Size:     entry.Size,
		Status:   entryStatus(entry),
		Protocol: string(entry.Protocol),
		Category: entry.Category,
		AddedOn:  entry.AddedOn,
	}
	if media != nil {
		item.ArrName = media.ArrName
		item.MediaType = media.MediaType
		item.Title = media.Title
		item.Year = media.Year
		item.Poster = media.Poster
		item.TmdbId = media.TmdbId
		item.Overview = media.Overview
		item.Genres = media.Genres
		item.Quality = media.Quality
		item.ReleaseGroup = media.ReleaseGroup
	}
	return item
}

func (s *Server) handleGetLibrary(w http.ResponseWriter, r *http.Request) {
	filterType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	filterProtocol := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("protocol")))
	filterStatus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filterArr := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("arr")))
	filterSearch := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))

	entries, err := s.manager.Storage().List(nil)
	if err != nil {
		http.Error(w, "failed to list entries", http.StatusInternalServerError)
		return
	}

	items := make([]LibraryItem, 0, len(entries))
	for _, entry := range entries {
		refs, _ := s.manager.Storage().FindArrMediaByInfoHash(entry.InfoHash)
		var media *storage.ArrMedia
		if len(refs) > 0 {
			media = &refs[0]
		}

		if media == nil {
			continue
		}

		item := buildLibraryItem(entry, media)

		if filterType != "" && item.MediaType != filterType {
			continue
		}
		if filterProtocol != "" && item.Protocol != filterProtocol {
			continue
		}
		if filterStatus != "" && item.Status != filterStatus {
			continue
		}
		if filterArr != "" && strings.ToLower(item.ArrName) != filterArr {
			continue
		}
		if filterSearch != "" {
			haystack := strings.ToLower(item.Title + " " + item.Name)
			if !strings.Contains(haystack, filterSearch) {
				continue
			}
		}

		items = append(items, item)
	}

	utils.JSONResponse(w, items, http.StatusOK)
}

func (s *Server) handleGetLibraryItem(w http.ResponseWriter, r *http.Request) {
	hash := strings.ToLower(chi.URLParam(r, "hash"))
	if hash == "" {
		http.Error(w, "hash required", http.StatusBadRequest)
		return
	}

	entry, err := s.manager.GetEntry(hash)
	if err != nil || entry == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	refs, _ := s.manager.Storage().FindArrMediaByInfoHash(hash)
	var media *storage.ArrMedia
	if len(refs) > 0 {
		media = &refs[0]
	}

	utils.JSONResponse(w, buildLibraryItem(entry, media), http.StatusOK)
}
