package manager

import (
	"testing"

	"github.com/sirrobot01/decypharr/pkg/storage"
)

// ── flattenProbeResults ───────────────────────────────────────────────────────

func TestFlattenProbeResults_Empty(t *testing.T) {
	out := flattenProbeResults(nil, nil)
	if len(out) != 0 {
		t.Errorf("expected empty, got %d", len(out))
	}
}

func TestFlattenProbeResults_AllPresent(t *testing.T) {
	files := map[string]*storage.File{
		"a.mkv": {Name: "a.mkv", InfoHash: "hash1"},
		"b.mkv": {Name: "b.mkv", InfoHash: "hash2"},
	}
	results := map[string]FileProbeResult{
		"a.mkv": {Name: "a.mkv", InfoHash: "hash1", Status: FileProbeHealthy},
		"b.mkv": {Name: "b.mkv", InfoHash: "hash2", Status: FileProbeBroken},
	}

	out := flattenProbeResults(results, files)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}

	byName := make(map[string]FileProbeResult, len(out))
	for _, r := range out {
		byName[r.Name] = r
	}
	if byName["a.mkv"].Status != FileProbeHealthy {
		t.Errorf("a.mkv: got %s, want healthy", byName["a.mkv"].Status)
	}
	if byName["b.mkv"].Status != FileProbeBroken {
		t.Errorf("b.mkv: got %s, want broken", byName["b.mkv"].Status)
	}
}

func TestFlattenProbeResults_MissingResultBecomesUnknown(t *testing.T) {
	files := map[string]*storage.File{
		"missing.mkv": {Name: "missing.mkv", InfoHash: "abc"},
	}
	// No matching result → synthesised as unknown
	out := flattenProbeResults(nil, files)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Status != FileProbeUnknown {
		t.Errorf("status: got %s, want unknown", out[0].Status)
	}
	if out[0].Reason != "not_probed" {
		t.Errorf("reason: got %q, want not_probed", out[0].Reason)
	}
	if out[0].InfoHash != "abc" {
		t.Errorf("infohash: got %q, want abc", out[0].InfoHash)
	}
}

func TestFlattenProbeResults_EmptyNameFilledFromKey(t *testing.T) {
	files := map[string]*storage.File{
		"video.mkv": {Name: "video.mkv", InfoHash: "xyz"},
	}
	// Result exists but has empty Name → should be filled with the map key
	results := map[string]FileProbeResult{
		"video.mkv": {Name: "", InfoHash: "xyz", Status: FileProbeHealthy},
	}
	out := flattenProbeResults(results, files)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Name != "video.mkv" {
		t.Errorf("name: got %q, want video.mkv", out[0].Name)
	}
}
