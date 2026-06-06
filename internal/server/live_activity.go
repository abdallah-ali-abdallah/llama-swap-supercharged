package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mostlygeek/llama-swap/internal/chain"
	"github.com/mostlygeek/llama-swap/internal/event"
	"github.com/mostlygeek/llama-swap/internal/router"
)

const liveActivityStatusInProgress = "in_progress"

// LiveActivityRow represents a single in-flight request.
type LiveActivityRow struct {
	ID              string  `json:"id"`
	Sequence        int64   `json:"sequence"`
	Timestamp       int64   `json:"timestamp"`
	Model           string  `json:"model"`
	Status          string  `json:"status"`
	SlotID          int     `json:"slot_id,omitempty"`
	TaskID          int     `json:"task_id,omitempty"`
	PPProgress      float64 `json:"pp_progress,omitempty"`
	PPExact         bool    `json:"pp_exact,omitempty"`
	PPSpeed         float64 `json:"pp_speed,omitempty"`
	UpdatedAt       int64   `json:"updated_at,omitempty"`
	GeneratedTokens int     `json:"generated_tokens,omitempty"`
	TGSpeed         float64 `json:"tg_speed,omitempty"`
}

// LiveActivityEvent is emitted when live activity rows change.
type LiveActivityEvent struct {
	Rows []LiveActivityRow `json:"rows"`
}

func (e LiveActivityEvent) Type() uint32 {
	return 0x08 // LiveActivityEventID
}

type tokenStream struct {
	mu       sync.RWMutex
	chunks   []TokenStreamChunk
	closed   bool
	waiters  []chan TokenStreamChunk
}

func (ts *tokenStream) append(chunk TokenStreamChunk) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return
	}
	ts.chunks = append(ts.chunks, chunk)
	for _, w := range ts.waiters {
		select {
		case w <- chunk:
		default:
		}
	}
}

func (ts *tokenStream) close() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.closed = true
	for _, w := range ts.waiters {
		close(w)
	}
	ts.waiters = nil
}

func (ts *tokenStream) snapshot() ([]TokenStreamChunk, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make([]TokenStreamChunk, len(ts.chunks))
	copy(out, ts.chunks)
	return out, ts.closed
}

func (ts *tokenStream) addWaiter() chan TokenStreamChunk {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ch := make(chan TokenStreamChunk, 16)
	for _, c := range ts.chunks {
		ch <- c
	}
	if ts.closed {
		close(ch)
	} else {
		ts.waiters = append(ts.waiters, ch)
	}
	return ch
}

// liveActivityTracker tracks in-flight requests and their token streams.
type liveActivityTracker struct {
	mu            sync.RWMutex
	nextSequence  int64
	rows          map[string]LiveActivityRow
	activeByModel map[string]map[string]struct{}
	tokenStreams  map[string]*tokenStream
}

func newLiveActivityTracker() *liveActivityTracker {
	return &liveActivityTracker{
		rows:          make(map[string]LiveActivityRow),
		activeByModel: make(map[string]map[string]struct{}),
		tokenStreams:  make(map[string]*tokenStream),
	}
}

func (t *liveActivityTracker) Start(model string) string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	t.nextSequence++
	seq := t.nextSequence
	id := "live-" + strconv.FormatInt(seq, 10)
	now := time.Now().UnixMilli()
	row := LiveActivityRow{
		ID:        id,
		Sequence:  seq,
		Timestamp: now,
		Model:     model,
		Status:    liveActivityStatusInProgress,
	}
	t.rows[id] = row
	if t.activeByModel[model] == nil {
		t.activeByModel[model] = make(map[string]struct{})
	}
	t.activeByModel[model][id] = struct{}{}
	t.tokenStreams[id] = &tokenStream{}
	rows := t.snapshotLocked()
	t.mu.Unlock()

	event.Emit(LiveActivityEvent{Rows: rows})
	return id
}

func (t *liveActivityTracker) Finish(id string) {
	if t == nil || id == "" {
		return
	}
	t.mu.Lock()
	row, ok := t.rows[id]
	if !ok {
		t.mu.Unlock()
		return
	}
	delete(t.rows, id)
	if active := t.activeByModel[row.Model]; active != nil {
		delete(active, id)
		if len(active) == 0 {
			delete(t.activeByModel, row.Model)
		}
	}
	if ts, ok := t.tokenStreams[id]; ok {
		ts.close()
		delete(t.tokenStreams, id)
	}
	rows := t.snapshotLocked()
	t.mu.Unlock()
	event.Emit(LiveActivityEvent{Rows: rows})
}

func (t *liveActivityTracker) GetTokenStream(id string) *tokenStream {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tokenStreams[id]
}

func (t *liveActivityTracker) snapshotLocked() []LiveActivityRow {
	out := make([]LiveActivityRow, 0, len(t.rows))
	for _, r := range t.rows {
		out = append(out, r)
	}
	return out
}

// TokenStreamChunk is a single content or reasoning token block.
type TokenStreamChunk struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// CreateLiveActivityMiddleware returns middleware that tracks in-flight
// requests for live activity streaming.
func CreateLiveActivityMiddleware(tracker *liveActivityTracker) chain.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, ok := router.ReadContext(r.Context())
			if !ok || data.ModelID == "" {
				next.ServeHTTP(w, r)
				return
			}
			id := tracker.Start(data.ModelID)
			defer tracker.Finish(id)
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) handleAPILiveTokenStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
		return
	}
	if s.liveActivity == nil {
		http.Error(w, `{"error":"live activity tracking unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	ts := s.liveActivity.GetTokenStream(id)
	if ts == nil {
		http.Error(w, `{"error":"stream not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
		return
	}

	// Send accumulated chunks immediately
	chunks, closed := ts.snapshot()
	for _, chunk := range chunks {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	if closed {
		fmt.Fprintf(w, "data: {\"done\":true}\n\n")
		flusher.Flush()
		return
	}
	flusher.Flush()

	// Stream new chunks as they arrive
	waiter := ts.addWaiter()
	defer func() {
		ts.mu.Lock()
		for i, ch := range ts.waiters {
			if ch == waiter {
				ts.waiters = append(ts.waiters[:i], ts.waiters[i+1:]...)
				close(ch)
				break
			}
		}
		ts.mu.Unlock()
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case chunk, ok := <-waiter:
			if !ok {
				fmt.Fprintf(w, "data: {\"done\":true}\n\n")
				flusher.Flush()
				return
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
