package logger

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

const DefaultRingSize = 2000

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Prefix  string    `json:"prefix"`
	Message string    `json:"message"`
}

type RingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	size    int
	head    int
	count   int
	subs    []chan LogEntry
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{entries: make([]LogEntry, size), size: size}
}

// Write implements io.Writer — receives zerolog JSON lines.
func (rb *RingBuffer) Write(p []byte) (n int, err error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(p, &raw); err != nil {
		return len(p), nil // ignore non-JSON lines
	}

	entry := LogEntry{
		Level:   getString(raw, "level"),
		Prefix:  getString(raw, "prefix"),
		Message: getString(raw, "message"),
	}
	if t, ok := raw["time"].(string); ok {
		entry.Time, _ = time.Parse(time.RFC3339, t)
	} else {
		entry.Time = time.Now()
	}

	rb.mu.Lock()
	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
	subs := append([]chan LogEntry(nil), rb.subs...)
	rb.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- entry:
		default:
		}
	}
	return len(p), nil
}

// List returns entries filtered by level and search, most recent last.
// Pass level="" and search="" to get all entries.
func (rb *RingBuffer) List(level, search string, limit, offset int) ([]LogEntry, int) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	// Reconstruct ordered slice (oldest → newest)
	ordered := make([]LogEntry, 0, rb.count)
	start := (rb.head - rb.count + rb.size) % rb.size
	for i := 0; i < rb.count; i++ {
		e := rb.entries[(start+i)%rb.size]
		if matchesFilter(e, level, search) {
			ordered = append(ordered, e)
		}
	}

	total := len(ordered)
	if offset >= total {
		return nil, total
	}
	end := offset + limit
	if end > total || limit == 0 {
		end = total
	}
	return ordered[offset:end], total
}

// Subscribe returns a channel that receives new entries in real-time.
func (rb *RingBuffer) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 64)
	rb.mu.Lock()
	rb.subs = append(rb.subs, ch)
	rb.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (rb *RingBuffer) Unsubscribe(ch chan LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for i, s := range rb.subs {
		if s == ch {
			rb.subs = append(rb.subs[:i], rb.subs[i+1:]...)
			close(ch)
			return
		}
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func matchesFilter(e LogEntry, level, search string) bool {
	if level != "" && e.Level != level {
		return false
	}
	if search != "" {
		s := strings.ToLower(search)
		if !strings.Contains(strings.ToLower(e.Message), s) &&
			!strings.Contains(strings.ToLower(e.Prefix), s) {
			return false
		}
	}
	return true
}
