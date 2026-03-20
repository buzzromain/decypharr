package arr

import (
	"reflect"
	"testing"
)

func TestManagedPaths(t *testing.T) {
	tests := []struct {
		name    string
		payload WebhookPayload
		want    []string
	}{
		{
			name: "Download with EpisodeFile only",
			payload: WebhookPayload{
				EventType:   EventTypeDownload,
				EpisodeFile: &WebhookFile{Path: "/tv/show/s01e01.mkv"},
			},
			want: []string{"/tv/show/s01e01.mkv"},
		},
		{
			name: "Download with EpisodeFiles slice",
			payload: WebhookPayload{
				EventType: EventTypeDownload,
				EpisodeFiles: []WebhookFile{
					{Path: "/tv/show/s01e01.mkv"},
					{Path: "/tv/show/s01e02.mkv"},
				},
			},
			want: []string{"/tv/show/s01e01.mkv", "/tv/show/s01e02.mkv"},
		},
		{
			name: "Download with MovieFile",
			payload: WebhookPayload{
				EventType: EventTypeDownload,
				MovieFile: &WebhookFile{Path: "/movies/film.mkv"},
			},
			want: []string{"/movies/film.mkv"},
		},
		{
			name: "EpisodeFileDelete",
			payload: WebhookPayload{
				EventType:   EventTypeEpisodeFileDelete,
				EpisodeFile: &WebhookFile{Path: "/tv/show/s01e01.mkv"},
			},
			want: []string{"/tv/show/s01e01.mkv"},
		},
		{
			name: "MovieFileDelete",
			payload: WebhookPayload{
				EventType: EventTypeMovieFileDelete,
				MovieFile: &WebhookFile{Path: "/movies/film.mkv"},
			},
			want: []string{"/movies/film.mkv"},
		},
		{
			name: "Rename with RenamedEpisodeFiles",
			payload: WebhookPayload{
				EventType: EventTypeRename,
				RenamedEpisodeFiles: []WebhookRename{
					{PreviousPath: "/tv/show/old.mkv", Path: "/tv/show/new.mkv"},
				},
			},
			want: []string{"/tv/show/new.mkv"},
		},
		{
			name: "Rename with RenamedMovieFiles",
			payload: WebhookPayload{
				EventType: EventTypeRename,
				RenamedMovieFiles: []WebhookRename{
					{PreviousPath: "/movies/old.mkv", Path: "/movies/new.mkv"},
				},
			},
			want: []string{"/movies/new.mkv"},
		},
		{
			name: "Test event returns nil",
			payload: WebhookPayload{
				EventType:   EventTypeTest,
				EpisodeFile: &WebhookFile{Path: "/tv/show/s01e01.mkv"},
				MovieFile:   &WebhookFile{Path: "/movies/film.mkv"},
			},
			want: nil,
		},
		{
			name: "Download with nil EpisodeFile no panic",
			payload: WebhookPayload{
				EventType:   EventTypeDownload,
				EpisodeFile: nil,
			},
			want: nil,
		},
		{
			name: "Download with empty path skipped",
			payload: WebhookPayload{
				EventType: EventTypeDownload,
				EpisodeFiles: []WebhookFile{
					{Path: ""},
					{Path: "/tv/show/s01e02.mkv"},
				},
			},
			want: []string{"/tv/show/s01e02.mkv"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.payload.ManagedPaths()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ManagedPaths() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPreviousPaths(t *testing.T) {
	tests := []struct {
		name    string
		payload WebhookPayload
		want    []string
	}{
		{
			name: "Rename with episode files",
			payload: WebhookPayload{
				EventType: EventTypeRename,
				RenamedEpisodeFiles: []WebhookRename{
					{PreviousPath: "/tv/show/old1.mkv", Path: "/tv/show/new1.mkv"},
					{PreviousPath: "/tv/show/old2.mkv", Path: "/tv/show/new2.mkv"},
				},
			},
			want: []string{"/tv/show/old1.mkv", "/tv/show/old2.mkv"},
		},
		{
			name: "Rename with movie files",
			payload: WebhookPayload{
				EventType: EventTypeRename,
				RenamedMovieFiles: []WebhookRename{
					{PreviousPath: "/movies/old.mkv", Path: "/movies/new.mkv"},
				},
			},
			want: []string{"/movies/old.mkv"},
		},
		{
			name: "Non-rename event returns empty",
			payload: WebhookPayload{
				EventType:   EventTypeDownload,
				EpisodeFile: &WebhookFile{Path: "/tv/show/s01e01.mkv"},
			},
			want: nil,
		},
		{
			name: "Empty PreviousPath in rename skipped",
			payload: WebhookPayload{
				EventType: EventTypeRename,
				RenamedEpisodeFiles: []WebhookRename{
					{PreviousPath: "", Path: "/tv/show/new.mkv"},
					{PreviousPath: "/tv/show/old2.mkv", Path: "/tv/show/new2.mkv"},
				},
			},
			want: []string{"/tv/show/old2.mkv"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.payload.PreviousPaths()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("PreviousPaths() = %v, want %v", got, tc.want)
			}
		})
	}
}
