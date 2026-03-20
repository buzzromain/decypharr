package fs

import (
	"errors"
	"io"
	iofs "io/fs"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/sirrobot01/decypharr/pkg/usenet/types"
)

// newTestFile creates a File with the given size and a nil NNTP manager.
// It is suitable for testing error paths that do not require a live connection.
func newTestFile(size int64) *File {
	vol := &types.Volume{
		Name: "test.mkv",
		Size: size,
	}
	return &File{
		volume: vol,
		info:   volumeInfo{name: vol.Name, size: vol.Size},
		logger: zerolog.Nop(),
		// manager is intentionally nil — triggers "no connection" error paths
	}
}

// ── Closed-file guard ─────────────────────────────────────────────────────────

func TestFile_Read_OnClosedFile(t *testing.T) {
	f := newTestFile(1000)
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, err := f.Read(make([]byte, 10))
	if !errors.Is(err, iofs.ErrClosed) {
		t.Errorf("Read on closed file = %v, want fs.ErrClosed", err)
	}
}

func TestFile_ReadAt_OnClosedFile(t *testing.T) {
	f := newTestFile(1000)
	_ = f.Close()
	_, err := f.ReadAt(make([]byte, 10), 0)
	if !errors.Is(err, iofs.ErrClosed) {
		t.Errorf("ReadAt on closed file = %v, want fs.ErrClosed", err)
	}
}

func TestFile_Write_OnClosedFile(t *testing.T) {
	f := newTestFile(1000)
	_ = f.Close()
	_, err := f.Write([]byte("data"))
	if !errors.Is(err, iofs.ErrClosed) {
		t.Errorf("Write on closed file = %v, want fs.ErrClosed", err)
	}
}

func TestFile_Seek_OnClosedFile(t *testing.T) {
	f := newTestFile(1000)
	_ = f.Close()
	_, err := f.Seek(0, io.SeekStart)
	if !errors.Is(err, iofs.ErrClosed) {
		t.Errorf("Seek on closed file = %v, want fs.ErrClosed", err)
	}
}

// ── Close idempotency ─────────────────────────────────────────────────────────

func TestFile_Close_IsIdempotent(t *testing.T) {
	f := newTestFile(1000)
	if err := f.Close(); err != nil {
		t.Fatalf("first Close() = %v, want nil", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("second Close() = %v, want nil (idempotent)", err)
	}
}

// ── ReadAt boundary conditions ────────────────────────────────────────────────

func TestFile_ReadAt_NilManager_ReturnsError(t *testing.T) {
	f := newTestFile(1000)
	_, err := f.ReadAt(make([]byte, 10), 0)
	if err == nil {
		t.Error("ReadAt with nil manager should return error")
	}
}

func TestFile_ReadAt_NegativeOffset_ReturnsError(t *testing.T) {
	f := newTestFile(1000)
	_, err := f.ReadAt(make([]byte, 10), -1)
	if err == nil {
		t.Error("ReadAt with negative offset should return error")
	}
}

func TestFile_ReadAt_AtFileSize_ReturnsEOF(t *testing.T) {
	f := newTestFile(100)
	_, err := f.ReadAt(make([]byte, 10), 100) // offset == size
	if !errors.Is(err, io.EOF) {
		t.Errorf("ReadAt at file size = %v, want io.EOF", err)
	}
}

func TestFile_ReadAt_BeyondFileSize_ReturnsEOF(t *testing.T) {
	f := newTestFile(100)
	_, err := f.ReadAt(make([]byte, 10), 200)
	if !errors.Is(err, io.EOF) {
		t.Errorf("ReadAt beyond file size = %v, want io.EOF", err)
	}
}

func TestFile_ReadAt_EmptyBuffer_ReturnsZero(t *testing.T) {
	f := newTestFile(1000)
	n, err := f.ReadAt(nil, 0)
	if n != 0 || err != nil {
		t.Errorf("ReadAt(nil, 0) = (%d, %v), want (0, nil)", n, err)
	}
}

// ── Seek boundary conditions ──────────────────────────────────────────────────

func TestFile_Seek_NegativeAbsolute_ReturnsError(t *testing.T) {
	f := newTestFile(1000)
	_, err := f.Seek(-1, io.SeekStart)
	if err == nil {
		t.Error("Seek(-1, SeekStart) should return error")
	}
}

func TestFile_Seek_BeyondSize_ClampsToSize(t *testing.T) {
	f := newTestFile(100)
	pos, err := f.Seek(200, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek beyond size error: %v", err)
	}
	if pos != 100 {
		t.Errorf("Seek beyond size = %d, want 100 (clamped)", pos)
	}
}

func TestFile_Seek_InvalidWhence_ReturnsError(t *testing.T) {
	f := newTestFile(1000)
	_, err := f.Seek(0, 99)
	if err == nil {
		t.Error("Seek with invalid whence should return error")
	}
}

func TestFile_Seek_SeekCurrent_UpdatesPosition(t *testing.T) {
	f := newTestFile(1000)
	pos, err := f.Seek(50, io.SeekStart)
	if err != nil || pos != 50 {
		t.Fatalf("Seek(50, Start) = (%d, %v)", pos, err)
	}
	pos, err = f.Seek(10, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek(10, Current) error: %v", err)
	}
	if pos != 60 {
		t.Errorf("Seek(10, Current) = %d, want 60", pos)
	}
}

func TestFile_Seek_SeekEnd(t *testing.T) {
	f := newTestFile(100)
	pos, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatalf("Seek(0, End) error: %v", err)
	}
	if pos != 100 {
		t.Errorf("Seek(0, End) = %d, want 100", pos)
	}
}

// ── Stat ─────────────────────────────────────────────────────────────────────

func TestFile_Stat_ReturnsCorrectInfo(t *testing.T) {
	f := newTestFile(12345)
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if fi.Name() != "test.mkv" {
		t.Errorf("Name = %q, want test.mkv", fi.Name())
	}
	if fi.Size() != 12345 {
		t.Errorf("Size = %d, want 12345", fi.Size())
	}
	if fi.IsDir() {
		t.Error("IsDir should be false")
	}
	if fi.Mode() != 0444 {
		t.Errorf("Mode = %v, want 0444", fi.Mode())
	}
}

// ── sync.Once: readerOnce initialises exactly once under concurrency ──────────

// TestFile_ConcurrentReadAt_NilManager_OnceInit verifies that concurrent ReadAt
// calls on a file with nil manager all return errors without panicking and that
// the sync.Once initializer fires exactly once (guaranteed by the nil-manager
// early-return path that sets readerErr before writing to streamingReader).
func TestFile_ConcurrentReadAt_NilManager_OnceInit(t *testing.T) {
	f := newTestFile(1000)

	const goroutines = 30
	start := make(chan struct{})
	errs := make([]error, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = f.ReadAt(make([]byte, 10), 0)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("goroutine %d: ReadAt with nil manager should return error", i)
		}
	}

	// After all goroutines finish, the error stored by readerOnce must be set.
	if f.readerErr == nil {
		t.Error("readerErr should be set after nil-manager initialization attempt")
	}
	// streamingReader must remain nil (manager was nil, nothing was stored).
	if f.streamingReader.Load() != nil {
		t.Error("streamingReader should be nil when manager is nil")
	}
}

// TestFile_ReaderOnce_CalledOnce verifies that concurrent ReadAt calls trigger
// readerOnce.Do exactly once: after wg.Wait() the readerOnce is in the done
// state even though every call returned an error.
func TestFile_ReaderOnce_CalledOnce(t *testing.T) {
	f := newTestFile(500)

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = f.ReadAt(make([]byte, 1), 0)
		}()
	}
	close(start)
	wg.Wait()

	// A second call to readerOnce.Do should be a no-op. If the error was set
	// by the first execution, subsequent calls see the stored error.
	before := f.readerErr
	f.readerOnce.Do(func() {
		t.Error("readerOnce.Do executed again — sync.Once contract violated")
	})
	if f.readerErr != before {
		t.Error("readerErr changed after readerOnce already fired")
	}
}

// ── Write on open file (no-op) ────────────────────────────────────────────────

func TestFile_Write_OnOpenFile_ReturnsError(t *testing.T) {
	f := newTestFile(1000)
	_, err := f.Write([]byte("data"))
	if err == nil {
		t.Error("Write on open file should return error (read-only)")
	}
}
