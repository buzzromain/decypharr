package hybrid

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendLog_CreateAndOpen(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create new log
	l, err := openAppendLog(path)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer l.Close()

	// Verify header was written
	header := make([]byte, logHeaderSize)
	if _, err := l.file.ReadAt(header, 0); err != nil {
		t.Fatalf("failed to read header: %v", err)
	}
	if string(header[0:4]) != logMagic {
		t.Errorf("expected magic %s, got %s", logMagic, string(header[0:4]))
	}
	if ver := binary.LittleEndian.Uint32(header[4:8]); ver != logVersion {
		t.Errorf("expected version %d, got %d", logVersion, ver)
	}
	if l.writePos != logHeaderSize {
		t.Errorf("expected writePos %d, got %d", logHeaderSize, l.writePos)
	}
}

func TestAppendLog_AppendAndRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := openAppendLog(path)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer l.Close()

	value := []byte("hello world")
	offset, size, err := l.Append("key1", value, false, "sonarr", "rd", "completed", "test", 1024, "torrent", false, 1000)
	if err != nil {
		t.Fatalf("failed to append: %v", err)
	}

	if size != int32(len(value)) {
		t.Errorf("expected size %d, got %d", len(value), size)
	}

	got, err := l.ReadAt(offset, size)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(got))
	}
}

func TestAppendLog_MultipleAppends(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := openAppendLog(path)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer l.Close()

	offsets := make([]int64, 3)
	sizes := make([]int32, 3)

	for i, val := range []string{"first", "second", "third"} {
		o, s, err := l.Append("key"+string(rune('0'+i)), []byte(val), false, "", "", "", "", 0, "", false, 0)
		if err != nil {
			t.Fatalf("append %d failed: %v", i, err)
		}
		offsets[i] = o
		sizes[i] = s
	}

	// Read them back
	for i, val := range []string{"first", "second", "third"} {
		got, err := l.ReadAt(offsets[i], sizes[i])
		if err != nil {
			t.Fatalf("read %d failed: %v", i, err)
		}
		if string(got) != val {
			t.Errorf("expected %q, got %q", val, string(got))
		}
	}
}

func TestAppendLog_Iterate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := openAppendLog(path)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer l.Close()

	// Append some records
	l.Append("k1", []byte("v1"), false, "cat1", "prov1", "ok", "name1", 100, "torrent", false, 1000)
	l.Append("k2", []byte("v2"), true, "", "", "", "", 0, "", false, 0) // deleted
	l.Append("k3", []byte("v3"), false, "cat2", "prov2", "ok", "name3", 300, "nzb", true, 2000)

	var records []*LogRecord
	err = l.Iterate(func(r *LogRecord) error {
		records = append(records, r)
		return nil
	})
	if err != nil {
		t.Fatalf("iterate failed: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// Record 1
	if records[0].Key != "k1" || records[0].Category != "cat1" || records[0].Protocol != "torrent" || records[0].Bad {
		t.Errorf("record 0 mismatch: %+v", records[0])
	}

	// Record 2 (deleted)
	if records[1].Key != "k2" || !records[1].Deleted {
		t.Errorf("record 1 should be deleted: %+v", records[1])
	}

	// Record 3 (bad flag, nzb protocol)
	if records[2].Key != "k3" || records[2].Protocol != "nzb" || !records[2].Bad || records[2].AddedOn != 2000 {
		t.Errorf("record 2 mismatch: %+v", records[2])
	}
}

func TestAppendLog_Size(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := openAppendLog(path)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer l.Close()

	sizeBefore := l.Size()
	if sizeBefore != logHeaderSize {
		t.Errorf("expected initial size %d, got %d", logHeaderSize, sizeBefore)
	}

	l.Append("key", []byte("value"), false, "", "", "", "", 0, "", false, 0)
	sizeAfter := l.Size()
	if sizeAfter <= sizeBefore {
		t.Error("expected size to increase after append")
	}
}

func TestAppendLog_InvalidMagic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.log")

	// Write a file with bad magic
	data := make([]byte, logHeaderSize)
	copy(data[0:4], "NOPE")
	binary.LittleEndian.PutUint32(data[4:8], logVersion)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}

	_, err := openAppendLog(path)
	if err == nil {
		t.Fatal("expected error for invalid magic")
	}
}

func TestAppendLog_FutureVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "future.log")

	// Write a file with future version
	data := make([]byte, logHeaderSize)
	copy(data[0:4], logMagic)
	binary.LittleEndian.PutUint32(data[4:8], logVersion+100)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err := openAppendLog(path)
	if err == nil {
		t.Fatal("expected error for future version")
	}
}

func TestAppendLog_ReopenExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Create and write
	l, err := openAppendLog(path)
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	l.Append("k1", []byte("v1"), false, "cat", "", "", "", 0, "", false, 0)
	l.Close()

	// Reopen
	l2, err := openAppendLog(path)
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	defer l2.Close()

	// Verify we can iterate records
	count := 0
	l2.Iterate(func(r *LogRecord) error {
		count++
		if r.Key != "k1" {
			t.Errorf("expected key k1, got %s", r.Key)
		}
		return nil
	})
	if count != 1 {
		t.Errorf("expected 1 record, got %d", count)
	}
}
