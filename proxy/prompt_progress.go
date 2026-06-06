package proxy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mostlygeek/llama-swap/internal/event"
)

const liveActivityStatusInProgress = "in_progress"

type promptProcessingProgress struct {
	Model       string
	SlotID      int
	TaskID      int
	Tokens      int
	BatchTokens int
	Progress    float64
	Speed       float64
	ParsedAt    time.Time
}

type promptProgressParser struct {
	mu          sync.Mutex
	model       string
	partialLine string
	reProgress  *regexp.Regexp // old llama.cpp format
	reProgress2 *regexp.Regexp // new llama.cpp format
}

func newPromptProgressParser(model string) *promptProgressParser {
	return &promptProgressParser{
		model: model,
		reProgress: regexp.MustCompile(
			`slot update_slots:\s+id\s+(\d+)\s+\|\s+task\s+(\d+)\s+\|.*prompt processing progress,\s+n_tokens\s*=\s*(\d+),\s*batch\.n_tokens\s*=\s*(\d+),\s*progress\s*=\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)`,
		),
		reProgress2: regexp.MustCompile(
			`slot print_timing:\s+id\s+(\d+)\s+\|\s+task\s+(\d+)\s+\|.*prompt processing,\s+n_tokens\s*=\s*(\d+),\s*progress\s*=\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)(?:,\s*t\s*=\s*[\d\.]+\s*s\s*/\s*([\d\.]+)\s*tokens per second)?`,
		),
	}
}

func (p *promptProgressParser) parseChunk(data []byte, callback func(promptProcessingProgress)) bool {
	p.mu.Lock()
	buffer := p.partialLine + string(data)
	parts := strings.Split(buffer, "\n")
	p.partialLine = parts[len(parts)-1]
	p.mu.Unlock()

	found := false
	parsedAt := time.Now()
	for _, part := range parts[:len(parts)-1] {
		if progress, ok := p.parseLine(part, parsedAt); ok {
			callback(progress)
			found = true
		}
	}

	p.mu.Lock()
	trailing := p.partialLine
	p.mu.Unlock()
	if trailing != "" {
		if progress, ok := p.parseLine(trailing, parsedAt); ok {
			callback(progress)
			p.mu.Lock()
			p.partialLine = ""
			p.mu.Unlock()
			found = true
		}
	}

	return found
}

func (p *promptProgressParser) parseLine(line string, parsedAt time.Time) (promptProcessingProgress, bool) {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "prompt processing") {
		return promptProcessingProgress{}, false
	}

	matches := p.reProgress.FindStringSubmatch(line)
	if len(matches) == 6 {
		slotID, err1 := strconv.Atoi(matches[1])
		taskID, err2 := strconv.Atoi(matches[2])
		tokens, err3 := strconv.Atoi(matches[3])
		batchTokens, err4 := strconv.Atoi(matches[4])
		progress, err5 := strconv.ParseFloat(matches[5], 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
			return promptProcessingProgress{}, false
		}
		return promptProcessingProgress{
			Model:       p.model,
			SlotID:      slotID,
			TaskID:      taskID,
			Tokens:      tokens,
			BatchTokens: batchTokens,
			Progress:    clampPromptProgress(progress),
			ParsedAt:    parsedAt,
		}, true
	}

	matches = p.reProgress2.FindStringSubmatch(line)
	if len(matches) >= 5 {
		slotID, err1 := strconv.Atoi(matches[1])
		taskID, err2 := strconv.Atoi(matches[2])
		tokens, err3 := strconv.Atoi(matches[3])
		progress, err4 := strconv.ParseFloat(matches[4], 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			return promptProcessingProgress{}, false
		}
		var speed float64
		if len(matches) >= 6 && matches[5] != "" {
			if s, err := strconv.ParseFloat(matches[5], 64); err == nil {
				speed = s
			}
		}
		return promptProcessingProgress{
			Model:       p.model,
			SlotID:      slotID,
			TaskID:      taskID,
			Tokens:      tokens,
			BatchTokens: tokens,
			Progress:    clampPromptProgress(progress),
			Speed:       speed,
			ParsedAt:    parsedAt,
		}, true
	}

	return promptProcessingProgress{}, false
}

func clampPromptProgress(progress float64) float64 {
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

type generationProgress struct {
	Model   string
	SlotID  int
	TaskID  int
	Decoded int
	Speed   float64
}

type generationTokenParser struct {
	mu          sync.Mutex
	model       string
	partialLine string
	re          *regexp.Regexp
}

func newGenerationTokenParser(model string) *generationTokenParser {
	return &generationTokenParser{
		model: model,
		re: regexp.MustCompile(
			`slot print_timing:\s+id\s+(\d+)\s+\|\s+task\s+(\d+)\s+\|.*n_decoded\s*=\s*(\d+)(?:,\s*tg\s*=\s*([\d\.]+)\s*t/s)?`,
		),
	}
}

func (p *generationTokenParser) parseChunk(data []byte, callback func(generationProgress)) bool {
	p.mu.Lock()
	buffer := p.partialLine + string(data)
	parts := strings.Split(buffer, "\n")
	p.partialLine = parts[len(parts)-1]
	p.mu.Unlock()

	found := false
	for _, part := range parts[:len(parts)-1] {
		if progress, ok := p.parseLine(part); ok {
			callback(progress)
			found = true
		}
	}

	return found
}

func (p *generationTokenParser) parseLine(line string) (generationProgress, bool) {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "slot print_timing:") {
		return generationProgress{}, false
	}
	if !strings.Contains(line, "n_decoded") {
		return generationProgress{}, false
	}

	matches := p.re.FindStringSubmatch(line)
	if len(matches) >= 4 {
		slotID, err1 := strconv.Atoi(matches[1])
		taskID, err2 := strconv.Atoi(matches[2])
		n, err3 := strconv.Atoi(matches[3])
		if err1 != nil || err2 != nil || err3 != nil {
			return generationProgress{}, false
		}
		var speed float64
		if len(matches) >= 5 && matches[4] != "" {
			if s, err := strconv.ParseFloat(matches[4], 64); err == nil {
				speed = s
			}
		}
		return generationProgress{
			Model:   p.model,
			SlotID:  slotID,
			TaskID:  taskID,
			Decoded: n,
			Speed:   speed,
		}, true
	}
	return generationProgress{}, false
}

type LiveActivityRow struct {
	ID              string     `json:"id"`
	Sequence        int64      `json:"sequence"`
	Timestamp       time.Time  `json:"timestamp"`
	Model           string     `json:"model"`
	Status          string     `json:"status"`
	SlotID          *int       `json:"slot_id,omitempty"`
	TaskID          *int       `json:"task_id,omitempty"`
	PPProgress      *float64   `json:"pp_progress,omitempty"`
	PPExact         bool       `json:"pp_exact"`
	PPSpeed         *float64   `json:"pp_speed,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	GeneratedTokens *int       `json:"generated_tokens,omitempty"`
	TGSpeed         *float64   `json:"tg_speed,omitempty"`
	generationExact bool
}

// tokenStreamChunk represents a single piece of streamed content
type tokenStreamChunk struct {
	Kind string `json:"kind"` // "content" or "reasoning"
	Text string `json:"text"`
}

// tokenStream accumulates token text for a live request and broadcasts new chunks
type tokenStream struct {
	mu        sync.Mutex
	chunks    []tokenStreamChunk
	content   strings.Builder
	reasoning strings.Builder
	closed    bool
	waiters   []chan tokenStreamChunk
}

func (ts *tokenStream) append(kind, text string) {
	ts.mu.Lock()
	chunk := tokenStreamChunk{Kind: kind, Text: text}
	ts.chunks = append(ts.chunks, chunk)
	switch kind {
	case "content":
		ts.content.WriteString(text)
	case "reasoning":
		ts.reasoning.WriteString(text)
	}
	waiters := make([]chan tokenStreamChunk, len(ts.waiters))
	copy(waiters, ts.waiters)
	ts.mu.Unlock()

	for _, ch := range waiters {
		select {
		case ch <- chunk:
		default:
		}
	}
}

func (ts *tokenStream) close() {
	ts.mu.Lock()
	ts.closed = true
	waiters := make([]chan tokenStreamChunk, len(ts.waiters))
	copy(waiters, ts.waiters)
	ts.waiters = nil
	ts.mu.Unlock()

	for _, ch := range waiters {
		close(ch)
	}
}

func (ts *tokenStream) snapshot() ([]tokenStreamChunk, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	result := make([]tokenStreamChunk, len(ts.chunks))
	copy(result, ts.chunks)
	return result, ts.closed
}

func (ts *tokenStream) addWaiter() chan tokenStreamChunk {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ch := make(chan tokenStreamChunk, 16)
	if ts.closed {
		close(ch)
		return ch
	}
	ts.waiters = append(ts.waiters, ch)
	return ch
}

func (ts *tokenStream) removeWaiter(ch chan tokenStreamChunk) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for i, w := range ts.waiters {
		if w == ch {
			ts.waiters = append(ts.waiters[:i], ts.waiters[i+1:]...)
			close(ch)
			break
		}
	}
}

type liveActivityTracker struct {
	mu             sync.RWMutex
	nextSequence   int64
	rows           map[string]LiveActivityRow
	activeByModel  map[string]map[string]struct{}
	pendingByModel map[string][]string
	slotTaskToRow  map[string]string
	tokenStreams   map[string]*tokenStream
}

func newLiveActivityTracker() *liveActivityTracker {
	return &liveActivityTracker{
		rows:           make(map[string]LiveActivityRow),
		activeByModel:  make(map[string]map[string]struct{}),
		pendingByModel: make(map[string][]string),
		slotTaskToRow:  make(map[string]string),
		tokenStreams:   make(map[string]*tokenStream),
	}
}

func (t *liveActivityTracker) Start(model string) string {
	if t == nil {
		return ""
	}

	t.mu.Lock()
	t.nextSequence++
	sequence := t.nextSequence
	id := "live-" + strconv.FormatInt(sequence, 10)
	row := LiveActivityRow{
		ID:        id,
		Sequence:  sequence,
		Timestamp: time.Now(),
		Model:     model,
		Status:    liveActivityStatusInProgress,
	}
	t.rows[id] = row
	if t.activeByModel[model] == nil {
		t.activeByModel[model] = make(map[string]struct{})
	}
	t.activeByModel[model][id] = struct{}{}
	t.pendingByModel[model] = append(t.pendingByModel[model], id)
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
	// Remove from pending queue.
	pending := t.pendingByModel[row.Model]
	for i, pid := range pending {
		if pid == id {
			t.pendingByModel[row.Model] = append(pending[:i], pending[i+1:]...)
			break
		}
	}
	// Remove from slot/task mappings.
	for key, rid := range t.slotTaskToRow {
		if rid == id {
			delete(t.slotTaskToRow, key)
		}
	}
	// Close and schedule cleanup of token stream.
	if ts, ok := t.tokenStreams[id]; ok {
		ts.close()
		go func(streamID string) {
			time.Sleep(30 * time.Second)
			t.mu.Lock()
			delete(t.tokenStreams, streamID)
			t.mu.Unlock()
		}(id)
	}
	rows := t.snapshotLocked()
	t.mu.Unlock()

	event.Emit(LiveActivityEvent{Rows: rows})
}

func (t *liveActivityTracker) assignSlotTask(model string, slotID, taskID int) string {
	if t == nil {
		return ""
	}

	key := fmt.Sprintf("%s/%d/%d", model, slotID, taskID)

	// Already mapped?
	if rowID, ok := t.slotTaskToRow[key]; ok {
		return rowID
	}

	// Pop oldest pending row for this model.
	pending := t.pendingByModel[model]
	if len(pending) == 0 {
		return ""
	}
	rowID := pending[0]
	t.pendingByModel[model] = pending[1:]
	t.slotTaskToRow[key] = rowID

	// Update row with slot/task info.
	row := t.rows[rowID]
	row.SlotID = &slotID
	row.TaskID = &taskID
	t.rows[rowID] = row

	return rowID
}

func (t *liveActivityTracker) SetPromptProgress(progress promptProcessingProgress) {
	if t == nil {
		return
	}

	t.mu.Lock()
	rowID := t.assignSlotTask(progress.Model, progress.SlotID, progress.TaskID)
	if rowID == "" {
		t.mu.Unlock()
		return
	}

	row := t.rows[rowID]
	now := time.Now()
	progressVal := clampPromptProgress(progress.Progress)
	row.PPProgress = &progressVal
	row.PPExact = true
	if progress.Speed > 0 {
		row.PPSpeed = &progress.Speed
	}
	row.UpdatedAt = &now
	t.rows[rowID] = row
	rows := t.snapshotLocked()
	t.mu.Unlock()

	event.Emit(LiveActivityEvent{Rows: rows})
}

func (t *liveActivityTracker) UpdateGeneratedTokens(id string, tokens int) {
	if t == nil || id == "" {
		return
	}

	t.mu.Lock()
	row, ok := t.rows[id]
	if !ok {
		t.mu.Unlock()
		return
	}

	if row.GeneratedTokens != nil && *row.GeneratedTokens == tokens {
		t.mu.Unlock()
		return
	}

	// Don't override an exact log count with an SSE approximation.
	if row.generationExact {
		t.mu.Unlock()
		return
	}

	now := time.Now()
	row.GeneratedTokens = &tokens
	row.UpdatedAt = &now
	t.rows[id] = row
	rows := t.snapshotLocked()
	t.mu.Unlock()

	event.Emit(LiveActivityEvent{Rows: rows})
}

func (t *liveActivityTracker) SetGeneratedTokens(progress generationProgress) {
	if t == nil {
		return
	}

	t.mu.Lock()
	rowID := t.assignSlotTask(progress.Model, progress.SlotID, progress.TaskID)
	if rowID == "" {
		t.mu.Unlock()
		return
	}

	row := t.rows[rowID]
	if row.GeneratedTokens != nil && *row.GeneratedTokens == progress.Decoded {
		t.mu.Unlock()
		return
	}

	now := time.Now()
	row.GeneratedTokens = &progress.Decoded
	row.generationExact = true
	if progress.Speed > 0 {
		row.TGSpeed = &progress.Speed
	}
	row.UpdatedAt = &now
	t.rows[rowID] = row
	rows := t.snapshotLocked()
	t.mu.Unlock()

	event.Emit(LiveActivityEvent{Rows: rows})
}

func (t *liveActivityTracker) AppendToken(id, kind, text string) {
	if t == nil || id == "" || text == "" {
		return
	}
	t.mu.RLock()
	ts, ok := t.tokenStreams[id]
	t.mu.RUnlock()
	if !ok {
		return
	}
	ts.append(kind, text)
}

func (t *liveActivityTracker) GetTokenStream(id string) *tokenStream {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.tokenStreams[id]
}

func (t *liveActivityTracker) Snapshot() []LiveActivityRow {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snapshotLocked()
}

func (t *liveActivityTracker) snapshotLocked() []LiveActivityRow {
	rows := make([]LiveActivityRow, 0, len(t.rows))
	for _, row := range t.rows {
		rows = append(rows, row)
	}
	return rows
}
