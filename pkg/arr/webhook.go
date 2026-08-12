package arr

// Webhook event type constants as sent by Sonarr/Radarr v4.
const (
	EventTypeTest              = "Test"
	EventTypeGrab              = "Grab"
	EventTypeMovieAdded        = "MovieAdded"
	EventTypeSeriesAdd         = "SeriesAdd"
	EventTypeDownload          = "Download"
	EventTypeEpisodeFileDelete = "EpisodeFileDelete"
	EventTypeMovieFileDelete   = "MovieFileDelete"
	EventTypeRename            = "Rename"
	EventTypeSeriesDelete      = "SeriesDelete"
	EventTypeMovieDelete       = "MovieDelete"
)

type WebhookImage struct {
	CoverType string `json:"coverType"`
	RemoteUrl string `json:"remoteUrl"`
}

type WebhookRelease struct {
	Quality      string `json:"quality"`
	ReleaseGroup string `json:"releaseGroup"`
}

// WebhookPayload represents the common payload sent by Sonarr/Radarr v4 webhooks.
type WebhookPayload struct {
	EventType    string `json:"eventType"`
	InstanceName string `json:"instanceName"`
	DownloadId   string `json:"downloadId,omitempty"` // = infohash for torrent grabs
	IsUpgrade    bool   `json:"isUpgrade,omitempty"`
	SourceFolder string `json:"sourceFolder,omitempty"`
	DeleteReason string `json:"deleteReason,omitempty"`
	DeletedFiles bool   `json:"deletedFiles,omitempty"`

	Release *WebhookRelease `json:"release,omitempty"`

	// Sonarr-specific fields
	Series              *WebhookSeries  `json:"series,omitempty"`
	EpisodeFile         *WebhookFile    `json:"episodeFile,omitempty"`
	EpisodeFiles        []WebhookFile   `json:"episodeFiles,omitempty"`
	RenamedEpisodeFiles []WebhookRename `json:"renamedEpisodeFiles,omitempty"`

	// Radarr-specific fields
	Movie             *WebhookMovie   `json:"movie,omitempty"`
	MovieFile         *WebhookFile    `json:"movieFile,omitempty"`
	RenamedMovieFiles []WebhookRename `json:"renamedMovieFiles,omitempty"`
}

// WebhookSeries carries series-level information (used in SeriesDelete and Download events).
type WebhookSeries struct {
	Id     int            `json:"id"`
	Title  string         `json:"title"`
	Year   int            `json:"year"`
	TmdbId int            `json:"tmdbId"`
	TvdbId int            `json:"tvdbId"`
	ImdbId string         `json:"imdbId"`
	Path   string         `json:"path"`
	Genres []string       `json:"genres"`
	Images []WebhookImage `json:"images"`
}

// PosterURL returns the remote URL of the poster image, or "" if absent.
func (s *WebhookSeries) PosterURL() string {
	for _, img := range s.Images {
		if img.CoverType == "poster" {
			return img.RemoteUrl
		}
	}
	return ""
}

// WebhookMovie carries movie-level information (used in MovieDelete and Download events).
type WebhookMovie struct {
	Id         int            `json:"id"`
	Title      string         `json:"title"`
	Year       int            `json:"year"`
	TmdbId     int            `json:"tmdbId"`
	ImdbId     string         `json:"imdbId"`
	FolderPath string         `json:"folderPath"`
	Overview   string         `json:"overview"`
	Genres     []string       `json:"genres"`
	Images     []WebhookImage `json:"images"`
}

// PosterURL returns the remote URL of the poster image, or "" if absent.
func (m *WebhookMovie) PosterURL() string {
	for _, img := range m.Images {
		if img.CoverType == "poster" {
			return img.RemoteUrl
		}
	}
	return ""
}

// WebhookFile carries file information within a webhook payload.
type WebhookFile struct {
	Id           int    `json:"id"`
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
}

// WebhookRename pairs a previous path with its new path for rename events.
type WebhookRename struct {
	PreviousPath string `json:"previousPath"`
	Path         string `json:"path"`
}

// ManagedPaths returns the current managed file paths described by the payload:
// imported paths for Download, the deleted file for EpisodeFileDelete/MovieFileDelete,
// and new paths for Rename events.
func (p *WebhookPayload) ManagedPaths() []string {
	var paths []string
	switch p.EventType {
	case EventTypeDownload:
		if p.EpisodeFile != nil && p.EpisodeFile.Path != "" {
			paths = append(paths, p.EpisodeFile.Path)
		}
		for _, f := range p.EpisodeFiles {
			if f.Path != "" {
				paths = append(paths, f.Path)
			}
		}
		if p.MovieFile != nil && p.MovieFile.Path != "" {
			paths = append(paths, p.MovieFile.Path)
		}
	case EventTypeEpisodeFileDelete:
		if p.EpisodeFile != nil && p.EpisodeFile.Path != "" {
			paths = append(paths, p.EpisodeFile.Path)
		}
	case EventTypeMovieFileDelete:
		if p.MovieFile != nil && p.MovieFile.Path != "" {
			paths = append(paths, p.MovieFile.Path)
		}
	case EventTypeRename:
		for _, r := range p.RenamedEpisodeFiles {
			if r.Path != "" {
				paths = append(paths, r.Path)
			}
		}
		for _, r := range p.RenamedMovieFiles {
			if r.Path != "" {
				paths = append(paths, r.Path)
			}
		}
	}
	return paths
}

// PreviousPaths returns the old file paths for Rename events.
func (p *WebhookPayload) PreviousPaths() []string {
	var paths []string
	for _, r := range p.RenamedEpisodeFiles {
		if r.PreviousPath != "" {
			paths = append(paths, r.PreviousPath)
		}
	}
	for _, r := range p.RenamedMovieFiles {
		if r.PreviousPath != "" {
			paths = append(paths, r.PreviousPath)
		}
	}
	return paths
}
