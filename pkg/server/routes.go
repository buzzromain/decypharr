package server

import (
	"io/fs"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

func (s *Server) WebRoutes() http.Handler {
	r := chi.NewRouter()

	// Allow CORS for local dev (e.g. Vite on a different port)
	if devOrigin := os.Getenv("DECYPHARR_DEV_ORIGIN"); devOrigin != "" {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Access-Control-Allow-Origin", devOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
	}

	// Apply setup redirect middleware globally
	r.Use(s.setupRedirectMiddleware)

	useV2 := os.Getenv("DECYPHARR_UI_V2") == "true"

	// Static assets - always public
	buildFS, _ := fs.Sub(assetsEmbed, "assets/build")
	// Vite writes hashed JS/CSS into a nested "assets/" subdirectory by
	// default (build.assetsDir); index.html references them at /assets/*.
	hashedAssetsFS, _ := fs.Sub(buildFS, "assets")
	imagesFS, _ := fs.Sub(imagesEmbed, "assets/images")
	r.Handle("/assets/*", http.StripPrefix(s.urlBase+"assets/", http.FileServer(http.FS(hashedAssetsFS))))
	r.Handle("/images/*", http.StripPrefix(s.urlBase+"images/", http.FileServer(http.FS(imagesFS))))
	// Everything else Vite copies verbatim from web/public (favicons, logo,
	// icons.svg) lands at the build root and is referenced by absolute root
	// paths (e.g. /favicon.ico, /logo.png), so serve it as the final fallback.
	r.NotFound(http.FileServer(http.FS(buildFS)).ServeHTTP)

	// Version endpoint - always available regardless of UI mode
	r.Get("/version", s.handleGetVersion)

	if useV2 {
		// React SPA: serve index.html for all page routes
		spaHandler := func(w http.ResponseWriter, r *http.Request) {
			data, err := assetsEmbed.ReadFile("assets/build/index.html")
			if err != nil {
				http.Error(w, "React build not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
		}
		for _, route := range []string{"/", "/download", "/repair", "/stats", "/settings", "/browse", "/login", "/register", "/setup", "/logs", "/library"} {
			r.Get(route, spaHandler)
		}
		r.Get("/library/{hash}", spaHandler)
	} else {
		// Public routes - no auth needed (template-based GET pages)
		r.Get("/login", s.LoginHandler)
		r.Get("/register", s.RegisterHandler)

		// Setup wizard - public, no auth required
		r.Get("/setup", s.SetupHandler)
	}

	// Auth POST endpoints - available in both UI modes
	r.Post("/login", s.LoginHandler)
	r.Post("/register", s.RegisterHandler)
	r.Post("/skip-auth", s.skipAuthHandler)

	r.Post("/api/setup/complete", s.setupCompleteHandler)

	// Protected routes - require auth
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		if !useV2 {
			// Web pages (template-based)
			r.Get("/", s.IndexHandler)
			r.Get("/browse", s.BrowseHandler)
			r.Get("/download", s.DownloadHandler)
			r.Get("/repair", s.RepairHandler)
			r.Get("/stats", s.StatsHandler)
			r.Get("/settings", s.ConfigHandler)
		}

		// API routes
		r.Route("/api", func(r chi.Router) {
			// Arr management
			r.Get("/arrs", s.handleGetArrs)
			r.Post("/add", s.handleAddContent)

			// Repair / health-checker operations
			r.Get("/repair/config", s.handleGetRepairConfig)
			r.Put("/repair/config", s.handleUpdateRepairConfig)
			r.Get("/repair/status", s.handleRepairStatus)
			r.Post("/repair/run", s.handleRunRepair)
			r.Post("/repair/stop", s.handleStopRepair)
			r.Post("/repair/recheck/media", s.handleRecheckMedia)
			r.Post("/repair/fix", s.handleFixBroken)
			r.Post("/repair/clear", s.handleClearBroken)
			r.Post("/repair/clear-state", s.handleClearRepairState)
			r.Get("/repair/runs", s.handleListRepairRuns)
			r.Get("/repair/runs/{id}", s.handleGetRepairRun)
			r.Delete("/repair/runs", s.handleClearRepairRuns)
			r.Get("/repair/health", s.handleListEntryHealth)
			r.Get("/repair/health/{name}", s.handleGetEntryHealth)
			r.Post("/repair/health/{name}/check", s.handleRecheckEntry)

			// Torrent management
			r.Get("/torrents", s.handleGetTorrents)
			r.Delete("/torrents/{category}/{hash}", s.handleDeleteTorrent)
			r.Delete("/torrents", s.handleDeleteTorrents) // Fixed trailing slash

			// Browse - WebDAV-style hierarchical file browser
			r.Route("/browse", func(r chi.Router) {
				// Hierarchical browse endpoints
				r.Get("/", s.handleBrowseMount)                                    // Mount: groups (__all__, __bad__, etc.)
				r.Get("/{group}", s.handleBrowseGroup)                             // Group: torrents
				r.Get("/{group}/{subgroup}/{torrent}", s.handleBrowseTorrentFiles) // Torrent files (with subgroup)
				r.Get("/{group}/{torrent}", s.handleBrowseTorrentFiles)            // Torrent files (without subgroup) - This route needs to come after the subgroup route

				// Torrent operations
				r.Delete("/torrents/{id}", s.handleDeleteBrowseTorrent)
				r.Delete("/torrents/batch", s.handleBatchDeleteBrowseTorrents)

				// File download
				r.Get("/download/{torrent}/{file}", s.handleDownloadFile)
			})

			// Maintenance
			r.Route("/maintenance/purge", func(r chi.Router) {
				r.Get("/local", s.handlePurgeLocalPreview)
				r.Delete("/local", s.handlePurgeLocalExecute)
				r.Get("/provider/{name}", s.handlePurgeProviderPreview)
				r.Delete("/provider/{name}", s.handlePurgeProviderExecute)
			})

			// Library
			r.Get("/library", s.handleGetLibrary)
			r.Get("/library/{hash}", s.handleGetLibraryItem)

			// Cache
			r.Get("/cache/stats", s.handleGetCacheStats)
			r.Get("/cache/files", s.handleGetCacheFiles)
			r.Get("/cache/files/{hash}", s.handleGetCacheFilesByHash)

			// Logs
			r.Get("/logs", s.handleGetLogs)
			r.Get("/logs/stream", s.handleStreamLogs)

			// Config/Auth
			r.Get("/config", s.handleGetConfig)
			r.Post("/config", s.handleUpdateConfig)
			r.Post("/virtual-folders/preview", s.handlePreviewVirtualFolder)
			r.Post("/mount/cache/cleanup", s.handleRunMountCacheCleanup)
			r.Post("/mount/cache/purge", s.handlePurgeMountCache)
			r.Post("/refresh-token", s.handleRefreshAPIToken)
			r.Post("/update-auth", s.handleUpdateAuth)
		})
	})

	return r
}
