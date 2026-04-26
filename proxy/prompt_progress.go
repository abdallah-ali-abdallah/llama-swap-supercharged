package proxy

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mostlygeek/llama-swap/event"
)

const liveActivityStatusInProgress = "in_progress"

type promptProcessingProgress struct {
	Model       string
	SlotID      int
	TaskID      int
	Tokens      int
	BatchTokens int
	Progress    float64
	ParsedAt    time.Time
}

type promptProgressParser struct {
	mu          sync.Mutex
	model       string
	partialLine string
	reProgress  *regexp.Regexp
}

func newPromptProgressParser(model string) *promptProgressParser {
	return &promptProgressParser{
		model: model,
		reProgress: regexp.MustCompile(
			`slot update_slots:\s+id\s+(\d+)\s+\|\s+task\s+(\d+)\s+\|.*prompt processing progress,\s+n_tokens\s*=\s*(\d+),\s*batch\.n_tokens\s*=\s*(\d+),\s*progress\s*=\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)`,
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
	if !strings.Contains(line, "prompt processing progress") {
		return promptProcessingProgress{}, false
	}

	matches := p.reProgress.FindStringSubmatch(line)
	if len(matches) != 6 {
		return promptProcessingProgress{}, false
	}

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

func clampPromptProgress(progress float64) float64 {
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

type LiveActivityRow struct {
	ID         string     `json:"id"`
	Sequence   int64      `json:"sequence"`
	Timestamp  time.Time  `json:"timestamp"`
	Model      string     `json:"model"`
	Status     string     `json:"status"`
	PPProgress *float64   `json:"pp_progress,omitempty"`
	PPExact    bool       `json:"pp_exact"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type liveActivityTracker struct {
	mu            sync.RWMutex
	nextSequence  int64
	rows          map[string]LiveActivityRow
	activeByModel map[string]map[string]struct{}
}

func newLiveActivityTracker() *liveActivityTracker {
	return &liveActivityTracker{
		rows:          make(map[string]LiveActivityRow),
		activeByModel: make(map[string]map[string]struct{}),
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
	if len(t.activeByModel[model]) > 1 {
		now := time.Now()
		for activeID := range t.activeByModel[model] {
			activeRow := t.rows[activeID]
			activeRow.PPProgress = nil
			activeRow.PPExact = false
			activeRow.UpdatedAt = &now
			t.rows[activeID] = activeRow
		}
	}
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
	rows := t.snapshotLocked()
	t.mu.Unlock()

	event.Emit(LiveActivityEvent{Rows: rows})
}

func (t *liveActivityTracker) SetPromptProgress(model string, progress float64) {
	if t == nil {
		return
	}

	t.mu.Lock()
	active := t.activeByModel[model]
	if len(active) == 0 {
		t.mu.Unlock()
		return
	}

	now := time.Now()
	if len(active) == 1 {
		for id := range active {
			row := t.rows[id]
			progress = clampPromptProgress(progress)
			row.PPProgress = &progress
			row.PPExact = true
			row.UpdatedAt = &now
			t.rows[id] = row
		}
	} else {
		for id := range active {
			row := t.rows[id]
			row.PPProgress = nil
			row.PPExact = false
			row.UpdatedAt = &now
			t.rows[id] = row
		}
	}
	rows := t.snapshotLocked()
	t.mu.Unlock()

	event.Emit(LiveActivityEvent{Rows: rows})
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
