package manager

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// newTestQueue creates a Queue backed by real storage in a temp dir.
// Registers t.Cleanup to close both the queue and the storage.
func newTestQueue(t *testing.T) (*Queue, *storage.Storage) {
	t.Helper()
	stor, err := storage.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	q := newQueue(context.Background(), stor, 100, "")
	t.Cleanup(func() {
		q.Close()
		_ = stor.Close()
	})
	return q, stor
}

// makeRequest returns a minimal ImportRequest with a unique id (uses uuid internally).
func makeRequest(name string) *ImportRequest {
	return &ImportRequest{
		Id:     fmt.Sprintf("req-%s-%d", name, time.Now().UnixNano()),
		Name:   name,
		Status: "queued",
	}
}

// makeEntry returns a minimal storage.Entry suitable for queue storage tests.
func makeEntry(infoHash, category string, protocol config.Protocol) *storage.Entry {
	return &storage.Entry{
		InfoHash: infoHash,
		Name:     infoHash, // GetFolder() uses Name by default
		Category: category,
		Protocol: protocol,
		AddedOn:  time.Now(),
	}
}

// ── PushRequest / PopRequest ──────────────────────────────────────────────────

func TestQueue_PushAndPop_Single(t *testing.T) {
	q, _ := newTestQueue(t)

	req := makeRequest("alpha")
	if err := q.PushRequest(req); err != nil {
		t.Fatalf("PushRequest: %v", err)
	}

	got, err := q.PopRequest()
	if err != nil {
		t.Fatalf("PopRequest: %v", err)
	}
	if got.Id != req.Id {
		t.Errorf("PopRequest returned id %q, want %q", got.Id, req.Id)
	}
}

func TestQueue_Pop_EmptyReturnsError(t *testing.T) {
	q, _ := newTestQueue(t)

	_, err := q.PopRequest()
	if err == nil {
		t.Fatal("expected error popping from empty queue, got nil")
	}
}

func TestQueue_Push_NilReturnsError(t *testing.T) {
	q, _ := newTestQueue(t)

	err := q.PushRequest(nil)
	if err == nil {
		t.Fatal("expected error pushing nil request, got nil")
	}
}

func TestQueue_Push_BeyondCapacityReturnsError(t *testing.T) {
	// capacity = 3
	stor, err := storage.NewStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	q := newQueue(context.Background(), stor, 3, "")
	t.Cleanup(func() { q.Close(); _ = stor.Close() })

	for i := 0; i < 3; i++ {
		if err := q.PushRequest(makeRequest(fmt.Sprintf("r%d", i))); err != nil {
			t.Fatalf("PushRequest %d: %v", i, err)
		}
	}

	// One more should exceed capacity
	err = q.PushRequest(makeRequest("overflow"))
	if err == nil {
		t.Fatal("expected 'queue is full' error, got nil")
	}
}

// ── RequestsSize / IsEmpty ────────────────────────────────────────────────────

func TestQueue_SizeAndEmpty_Initial(t *testing.T) {
	q, _ := newTestQueue(t)

	if !q.IsEmpty() {
		t.Error("new queue should be empty")
	}
	if s := q.RequestsSize(); s != 0 {
		t.Errorf("RequestsSize = %d, want 0", s)
	}
}

func TestQueue_SizeAndEmpty_AfterPush(t *testing.T) {
	q, _ := newTestQueue(t)

	_ = q.PushRequest(makeRequest("x"))

	if q.IsEmpty() {
		t.Error("queue should not be empty after push")
	}
	if s := q.RequestsSize(); s != 1 {
		t.Errorf("RequestsSize = %d, want 1", s)
	}
}

// ── DeleteRequest ─────────────────────────────────────────────────────────────

func TestQueue_DeleteRequest_ExistingID(t *testing.T) {
	q, _ := newTestQueue(t)

	req := makeRequest("del-me")
	_ = q.PushRequest(req)

	if ok := q.DeleteRequest(req.Id); !ok {
		t.Fatal("DeleteRequest returned false for known id")
	}
}

func TestQueue_DeleteRequest_UnknownID(t *testing.T) {
	q, _ := newTestQueue(t)

	if ok := q.DeleteRequest("does-not-exist"); ok {
		t.Fatal("DeleteRequest should return false for unknown id")
	}
}

func TestQueue_DeleteRequest_FindAfterDelete(t *testing.T) {
	q, _ := newTestQueue(t)

	req := makeRequest("gone")
	_ = q.PushRequest(req)
	_ = q.DeleteRequest(req.Id)

	if got := q.FindRequest(req.Id); got != nil {
		t.Errorf("FindRequest after delete returned %+v, want nil", got)
	}
}

// ── FindRequest ───────────────────────────────────────────────────────────────

func TestQueue_FindRequest_Present(t *testing.T) {
	q, _ := newTestQueue(t)

	req := makeRequest("find-me")
	_ = q.PushRequest(req)

	got := q.FindRequest(req.Id)
	if got == nil {
		t.Fatal("FindRequest returned nil for pushed request")
	}
	if got.Id != req.Id {
		t.Errorf("FindRequest id = %q, want %q", got.Id, req.Id)
	}
	// Must still be in the queue (Find does not pop)
	if q.RequestsSize() != 1 {
		t.Errorf("RequestsSize after Find = %d, want 1", q.RequestsSize())
	}
}

func TestQueue_FindRequest_Absent(t *testing.T) {
	q, _ := newTestQueue(t)

	if got := q.FindRequest("nope"); got != nil {
		t.Errorf("FindRequest returned %+v, want nil", got)
	}
}

// ── DeleteRequestWhere ────────────────────────────────────────────────────────

func TestQueue_DeleteRequestWhere_Predicate(t *testing.T) {
	q, _ := newTestQueue(t)

	r1 := makeRequest("keep")
	r2 := makeRequest("remove")
	r3 := makeRequest("remove2")
	_ = q.PushRequest(r1)
	_ = q.PushRequest(r2)
	_ = q.PushRequest(r3)

	deleted := q.DeleteRequestWhere(func(r *ImportRequest) bool {
		return r.Name != "keep"
	})

	if deleted != 2 {
		t.Errorf("DeleteRequestWhere deleted %d, want 2", deleted)
	}
	if q.RequestsSize() != 1 {
		t.Errorf("RequestsSize = %d, want 1", q.RequestsSize())
	}
	if q.FindRequest(r1.Id) == nil {
		t.Error("kept request should still be in queue")
	}
}

func TestQueue_DeleteRequestWhere_NilPredicateDeletesAll(t *testing.T) {
	q, _ := newTestQueue(t)

	_ = q.PushRequest(makeRequest("a"))
	_ = q.PushRequest(makeRequest("b"))
	_ = q.PushRequest(makeRequest("c"))

	deleted := q.DeleteRequestWhere(nil)

	if deleted != 3 {
		t.Errorf("DeleteRequestWhere(nil) deleted %d, want 3", deleted)
	}
	if !q.IsEmpty() {
		t.Error("queue should be empty after deleting all")
	}
}

// ── FIFO order ────────────────────────────────────────────────────────────────

func TestQueue_PopRequest_FIFO(t *testing.T) {
	q, _ := newTestQueue(t)

	names := []string{"A", "B", "C"}
	reqs := make([]*ImportRequest, len(names))
	for i, n := range names {
		reqs[i] = makeRequest(n)
		_ = q.PushRequest(reqs[i])
	}

	for i, want := range reqs {
		got, err := q.PopRequest()
		if err != nil {
			t.Fatalf("PopRequest [%d]: %v", i, err)
		}
		if got.Id != want.Id {
			t.Errorf("pop[%d] id = %q, want %q", i, got.Id, want.Id)
		}
	}
}

// ── Concurrency ───────────────────────────────────────────────────────────────

func TestQueue_ConcurrentPush(t *testing.T) {
	q, _ := newTestQueue(t)

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			req := makeRequest(fmt.Sprintf("concurrent-%d", n))
			// Ignore "queue is full" errors if they occur, focus on races
			_ = q.PushRequest(req)
		}(i)
	}

	wg.Wait()

	// Drain whatever was pushed
	for !q.IsEmpty() {
		if _, err := q.PopRequest(); err != nil {
			break
		}
	}
}

// ── ListFilter (storage-backed) ───────────────────────────────────────────────

func TestQueue_ListFilter_Empty(t *testing.T) {
	q, _ := newTestQueue(t)

	result := q.ListFilter("", config.ProtocolAll, "", nil, "", false)
	if len(result) != 0 {
		t.Errorf("ListFilter on empty queue = %d entries, want 0", len(result))
	}
}

func TestQueue_ListFilter_ByCategory(t *testing.T) {
	q, _ := newTestQueue(t)

	entries := []*storage.Entry{
		makeEntry("hash1", "sonarr", config.ProtocolTorrent),
		makeEntry("hash2", "radarr", config.ProtocolTorrent),
		makeEntry("hash3", "sonarr", config.ProtocolTorrent),
	}
	for _, e := range entries {
		if err := q.Add(e); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	got := q.ListFilter("sonarr", config.ProtocolAll, "", nil, "", false)
	if len(got) != 2 {
		t.Errorf("ListFilter(sonarr) = %d entries, want 2", len(got))
	}
	for _, e := range got {
		if e.Category != "sonarr" {
			t.Errorf("unexpected category %q in sonarr-filtered result", e.Category)
		}
	}
}

func TestQueue_ListFilter_EmptyCategoryReturnsAll(t *testing.T) {
	q, _ := newTestQueue(t)

	hashes := []string{"h1", "h2", "h3"}
	for _, h := range hashes {
		if err := q.Add(makeEntry(h, "sonarr", config.ProtocolTorrent)); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}

	got := q.ListFilter("", config.ProtocolAll, "", nil, "", false)
	if len(got) != len(hashes) {
		t.Errorf("ListFilter(all) = %d, want %d", len(got), len(hashes))
	}
}

func TestQueue_ListFilter_ProtocolAll(t *testing.T) {
	q, _ := newTestQueue(t)

	_ = q.Add(makeEntry("t1", "sonarr", config.ProtocolTorrent))
	_ = q.Add(makeEntry("n1", "sonarr", config.ProtocolNZB))

	got := q.ListFilter("", config.ProtocolAll, "", nil, "", false)
	if len(got) != 2 {
		t.Errorf("ListFilter with ProtocolAll = %d, want 2", len(got))
	}
}

func TestQueue_ListFilter_ByProtocol(t *testing.T) {
	q, _ := newTestQueue(t)

	_ = q.Add(makeEntry("t1", "sonarr", config.ProtocolTorrent))
	_ = q.Add(makeEntry("n1", "sonarr", config.ProtocolNZB))

	got := q.ListFilter("", config.ProtocolTorrent, "", nil, "", false)
	if len(got) != 1 {
		t.Errorf("ListFilter(ProtocolTorrent) = %d, want 1", len(got))
	}
	if len(got) == 1 && got[0].Protocol != config.ProtocolTorrent {
		t.Errorf("result protocol = %q, want %q", got[0].Protocol, config.ProtocolTorrent)
	}
}
