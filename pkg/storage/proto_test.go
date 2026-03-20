package storage

import (
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/pkg/arr"
)

// ── EntryItemToProto / ProtoToEntryItem ───────────────────────────────────────

func TestEntryItemProtoRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)

	ei := &EntryItem{
		Name: "Breaking Bad",
		Size: 12345,
		Files: map[string]*File{
			"s01e01.mkv": {
				Name:     "s01e01.mkv",
				Path:     "/tv/bb/s01e01.mkv",
				Size:     5000,
				AddedOn:  now,
				InfoHash: "abc123",
			},
			"s01e02.mkv": {
				Name:    "s01e02.mkv",
				Size:    6000,
				Deleted: true,
			},
		},
	}

	got := ProtoToEntryItem(EntryItemToProto(ei))

	if got.Name != ei.Name {
		t.Errorf("Name = %q, want %q", got.Name, ei.Name)
	}
	if got.Size != ei.Size {
		t.Errorf("Size = %d, want %d", got.Size, ei.Size)
	}
	if len(got.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(got.Files))
	}
	f := got.Files["s01e01.mkv"]
	if f == nil {
		t.Fatal("s01e01.mkv missing from round-tripped EntryItem")
	}
	if f.Path != "/tv/bb/s01e01.mkv" {
		t.Errorf("Path = %q, want /tv/bb/s01e01.mkv", f.Path)
	}
	if f.AddedOn.Unix() != now.Unix() {
		t.Errorf("AddedOn = %v, want %v", f.AddedOn, now)
	}
	if !got.Files["s01e02.mkv"].Deleted {
		t.Error("Deleted flag not preserved")
	}
}

func TestEntryItemProtoRoundTrip_ByteRange(t *testing.T) {
	t.Parallel()
	br := [2]int64{0, 999}
	ei := &EntryItem{
		Name: "single",
		Files: map[string]*File{
			"f": {Name: "f", ByteRange: &br},
		},
	}
	got := ProtoToEntryItem(EntryItemToProto(ei))
	if got.Files["f"].ByteRange == nil {
		t.Fatal("ByteRange should not be nil after round-trip")
	}
	if (*got.Files["f"].ByteRange)[0] != 0 || (*got.Files["f"].ByteRange)[1] != 999 {
		t.Errorf("ByteRange = %v, want [0 999]", *got.Files["f"].ByteRange)
	}
}

func TestEntryItemProtoRoundTrip_Empty(t *testing.T) {
	t.Parallel()
	ei := &EntryItem{Name: "empty", Files: map[string]*File{}}
	got := ProtoToEntryItem(EntryItemToProto(ei))
	if got.Name != "empty" {
		t.Errorf("Name = %q, want empty", got.Name)
	}
	if len(got.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(got.Files))
	}
}

// ── JobToProto / ProtoToJob ───────────────────────────────────────────────────

func TestJobProtoRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)

	j := &Job{
		ID:          "job-abc",
		Arrs:        []string{"sonarr", "radarr"},
		MediaIDs:    []string{"m1", "m2"},
		Status:      "completed",
		AutoProcess: true,
		Recurrent:   false,
		Error:       "some error",
		StartedAt:   now.Add(-time.Minute),
		CompletedAt: now,
		BrokenItems: map[string][]arr.ContentFile{
			"sonarr": {
				{
					Name:         "ep.mkv",
					Path:         "/tv/ep.mkv",
					Id:           42,
					EpisodeId:    10,
					FileId:       5,
					SeasonNumber: 1,
					IsBroken:     true,
					Size:         999,
				},
			},
		},
	}

	got := ProtoToJob(JobToProto(j))

	if got.ID != j.ID {
		t.Errorf("ID = %q, want %q", got.ID, j.ID)
	}
	if len(got.Arrs) != 2 || got.Arrs[0] != "sonarr" {
		t.Errorf("Arrs = %v, want [sonarr radarr]", got.Arrs)
	}
	if string(got.Status) != "completed" {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if !got.AutoProcess {
		t.Error("AutoProcess should be true")
	}
	if got.Recurrent {
		t.Error("Recurrent should be false")
	}
	if got.Error != "some error" {
		t.Errorf("Error = %q, want some error", got.Error)
	}
	if got.StartedAt.Unix() != j.StartedAt.Unix() {
		t.Errorf("StartedAt mismatch: got %v, want %v", got.StartedAt, j.StartedAt)
	}
	if got.CompletedAt.Unix() != j.CompletedAt.Unix() {
		t.Errorf("CompletedAt mismatch: got %v, want %v", got.CompletedAt, j.CompletedAt)
	}
	sonarrFiles, ok := got.BrokenItems["sonarr"]
	if !ok || len(sonarrFiles) != 1 {
		t.Fatalf("BrokenItems[sonarr] missing or wrong length")
	}
	f := sonarrFiles[0]
	if f.Name != "ep.mkv" {
		t.Errorf("ContentFile.Name = %q, want ep.mkv", f.Name)
	}
	if f.Id != 42 || f.EpisodeId != 10 || f.FileId != 5 || f.SeasonNumber != 1 {
		t.Errorf("ContentFile IDs mismatch: %+v", f)
	}
	if !f.IsBroken || f.Size != 999 {
		t.Errorf("ContentFile flags mismatch: IsBroken=%v Size=%d", f.IsBroken, f.Size)
	}
}

func TestJobProtoRoundTrip_ZeroTimestamps(t *testing.T) {
	t.Parallel()
	j := &Job{
		ID:          "empty",
		BrokenItems: map[string][]arr.ContentFile{},
	}
	got := ProtoToJob(JobToProto(j))
	// Zero timestamps should remain zero after round-trip
	if !got.StartedAt.IsZero() {
		t.Errorf("StartedAt should be zero, got %v", got.StartedAt)
	}
	if !got.CompletedAt.IsZero() {
		t.Errorf("CompletedAt should be zero, got %v", got.CompletedAt)
	}
	if !got.FailedAt.IsZero() {
		t.Errorf("FailedAt should be zero, got %v", got.FailedAt)
	}
}

// ── SwitcherJobToProto / ProtoToSwitcherJob ───────────────────────────────────

func TestSwitcherJobProtoRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)
	completedAt := now

	sj := &SwitcherJob{
		ID:             "switcher-1",
		InfoHash:       "abc123",
		SourceProvider: "rd",
		TargetProvider: "tb",
		Status:         SwitcherStatusCompleted,
		Progress:       100.0,
		Error:          "",
		CreatedAt:      now.Add(-time.Minute),
		CompletedAt:    &completedAt,
		KeepOld:        true,
		WaitComplete:   false,
	}

	got := ProtoToSwitcherJob(SwitcherJobToProto(sj))

	if got.ID != sj.ID {
		t.Errorf("ID = %q, want %q", got.ID, sj.ID)
	}
	if got.InfoHash != sj.InfoHash {
		t.Errorf("InfoHash = %q, want %q", got.InfoHash, sj.InfoHash)
	}
	if got.SourceProvider != "rd" || got.TargetProvider != "tb" {
		t.Errorf("Providers = (%q, %q), want (rd, tb)", got.SourceProvider, got.TargetProvider)
	}
	if got.Status != SwitcherStatusCompleted {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.Progress != 100.0 {
		t.Errorf("Progress = %f, want 100.0", got.Progress)
	}
	if !got.KeepOld {
		t.Error("KeepOld should be true")
	}
	if got.WaitComplete {
		t.Error("WaitComplete should be false")
	}
	if got.CreatedAt.Unix() != sj.CreatedAt.Unix() {
		t.Errorf("CreatedAt mismatch")
	}
	if got.CompletedAt == nil {
		t.Fatal("CompletedAt should not be nil")
	}
	if got.CompletedAt.Unix() != completedAt.Unix() {
		t.Errorf("CompletedAt = %v, want %v", got.CompletedAt, completedAt)
	}
}

func TestSwitcherJobProtoRoundTrip_NoCompletedAt(t *testing.T) {
	t.Parallel()
	sj := &SwitcherJob{
		ID:     "pending",
		Status: SwitcherStatusPending,
	}
	got := ProtoToSwitcherJob(SwitcherJobToProto(sj))
	if got.CompletedAt != nil {
		t.Error("CompletedAt should be nil for pending job")
	}
	if got.ID != "pending" {
		t.Errorf("ID = %q, want pending", got.ID)
	}
}

// ── SystemMigrationStatusToProto / ProtoToSystemMigrationStatus ──────────────

func TestSystemMigrationStatusProtoRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().Truncate(time.Second)

	sms := &SystemMigrationStatus{
		Running:   true,
		Total:     100,
		Completed: 50,
		Errors:    3,
		StartedAt: now.Add(-time.Hour),
		UpdatedAt: now,
		ErrorList: []string{"err1", "err2"},
	}

	got := ProtoToSystemMigrationStatus(SystemMigrationStatusToProto(sms))

	if !got.Running {
		t.Error("Running should be true")
	}
	if got.Total != 100 || got.Completed != 50 || got.Errors != 3 {
		t.Errorf("Counts = (%d, %d, %d), want (100, 50, 3)", got.Total, got.Completed, got.Errors)
	}
	if got.StartedAt.Unix() != sms.StartedAt.Unix() {
		t.Errorf("StartedAt mismatch: got %v, want %v", got.StartedAt, sms.StartedAt)
	}
	if got.UpdatedAt.Unix() != sms.UpdatedAt.Unix() {
		t.Errorf("UpdatedAt mismatch: got %v, want %v", got.UpdatedAt, sms.UpdatedAt)
	}
	if len(got.ErrorList) != 2 || got.ErrorList[0] != "err1" || got.ErrorList[1] != "err2" {
		t.Errorf("ErrorList = %v, want [err1, err2]", got.ErrorList)
	}
}

func TestSystemMigrationStatusProtoRoundTrip_ZeroTimestamps(t *testing.T) {
	t.Parallel()
	sms := &SystemMigrationStatus{Running: false}
	got := ProtoToSystemMigrationStatus(SystemMigrationStatusToProto(sms))
	if !got.StartedAt.IsZero() {
		t.Errorf("StartedAt should be zero, got %v", got.StartedAt)
	}
	if !got.UpdatedAt.IsZero() {
		t.Errorf("UpdatedAt should be zero, got %v", got.UpdatedAt)
	}
}
