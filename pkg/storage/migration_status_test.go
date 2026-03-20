package storage

import (
	"testing"
	"time"
)

func TestMigrationStatus_SaveGetRoundTrip(t *testing.T) {
	t.Parallel()

	st, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	in := &SystemMigrationStatus{
		Running:   true,
		Total:     10,
		Completed: 3,
		Errors:    1,
		StartedAt: now,
		UpdatedAt: now,
		ErrorList: []string{"first error"},
	}
	if err := st.SaveMigrationStatus(in); err != nil {
		t.Fatalf("SaveMigrationStatus() error = %v", err)
	}

	got, err := st.GetMigrationStatus()
	if err != nil {
		t.Fatalf("GetMigrationStatus() error = %v", err)
	}
	if got.Total != in.Total || got.Completed != in.Completed || got.Errors != in.Errors || got.Running != in.Running {
		t.Fatalf("status mismatch: got=%+v want=%+v", *got, *in)
	}
	if len(got.ErrorList) != 1 || got.ErrorList[0] != "first error" {
		t.Fatalf("error list mismatch: %+v", got.ErrorList)
	}
}

func TestMigrationStatus_GetCorruptedDataReturnsError(t *testing.T) {
	t.Parallel()

	st, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.entries.Put("__migration_status__", []byte("not-proto"), nil); err != nil {
		t.Fatalf("inject corrupted value: %v", err)
	}

	if _, err := st.GetMigrationStatus(); err == nil {
		t.Fatal("expected unmarshal error for corrupted migration status")
	}
}
