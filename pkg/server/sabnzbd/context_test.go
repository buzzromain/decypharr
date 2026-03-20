package sabnzbd

import (
	"context"
	"testing"

	"github.com/sirrobot01/decypharr/pkg/arr"
)

func TestGetMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"with mode", context.WithValue(context.Background(), modeKey, "queue"), "queue"},
		{"history mode", context.WithValue(context.Background(), modeKey, "history"), "history"},
		{"no mode", context.Background(), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getMode(tt.ctx); got != tt.want {
				t.Errorf("getMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetCategory_SABnzbd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"with category", context.WithValue(context.Background(), categoryKey, "sonarr"), "sonarr"},
		{"empty category", context.WithValue(context.Background(), categoryKey, ""), ""},
		{"no category", context.Background(), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getCategory(tt.ctx); got != tt.want {
				t.Errorf("getCategory() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetArrFromContext_SABnzbd(t *testing.T) {
	t.Parallel()
	a := &arr.Arr{Name: "sonarr"}
	ctx := context.WithValue(context.Background(), arrKey, a)

	got := getArrFromContext(ctx)
	if got == nil || got.Name != "sonarr" {
		t.Errorf("expected arr with name 'sonarr', got %+v", got)
	}

	if got := getArrFromContext(context.Background()); got != nil {
		t.Errorf("expected nil for missing arr, got %+v", got)
	}
}
