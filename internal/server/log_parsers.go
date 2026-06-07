package server

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mostlygeek/llama-swap/internal/router"
)

// promptProcessingProgress holds parsed prompt processing stats.
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

// promptProgressParser parses llama-server stderr for prompt processing progress.
type promptProgressParser struct {
	mu          sync.Mutex
	model       string
	partialLine string
	reProgress  *regexp.Regexp
	reProgress2 *regexp.Regexp
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
			Progress:    progress,
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
			Model:    p.model,
			SlotID:   slotID,
			TaskID:   taskID,
			Tokens:   tokens,
			Progress: progress,
			Speed:    speed,
			ParsedAt: parsedAt,
		}, true
	}
	return promptProcessingProgress{}, false
}

// generationProgress holds parsed generation token stats.
type generationProgress struct {
	Model   string
	SlotID  int
	TaskID  int
	Decoded int
	Speed   float64
}

// generationTokenParser parses llama-server stderr for generation stats.
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

// llamaCppMemorySnapshot captures parsed llama-server memory breakdown.
type LlamaCppMemorySnapshot struct {
	Source            string                    `json:"source"`
	UpdatedAt         time.Time                 `json:"updated_at"`
	DeviceTotalBytes  uint64                    `json:"device_total_bytes"`
	HostTotalBytes    uint64                    `json:"host_total_bytes"`
	TotalTrackedBytes uint64                    `json:"total_tracked_bytes"`
	Devices           []LlamaCppMemoryComponent `json:"devices,omitempty"`
	Host              []LlamaCppMemoryComponent `json:"host,omitempty"`
	Unknown           []LlamaCppMemoryComponent `json:"unknown,omitempty"`
}

// LlamaCppMemoryComponent holds memory usage for a single device/host component.
type LlamaCppMemoryComponent struct {
	Name                string  `json:"name"`
	ModelBytes          uint64  `json:"model_bytes,omitempty"`
	KVBytes             uint64  `json:"kv_bytes,omitempty"`
	ComputeBytes        uint64  `json:"compute_bytes,omitempty"`
	OutputBytes         uint64  `json:"output_bytes,omitempty"`
	TrackedBytes        uint64  `json:"tracked_bytes"`
	DeviceCapacityBytes uint64  `json:"device_capacity_bytes,omitempty"`
	DeviceFreeBytes     uint64  `json:"device_free_bytes,omitempty"`
	UnaccountedBytes    *uint64 `json:"unaccounted_bytes,omitempty"`
}

var (
	llamaCppBufferSizeRe      = regexp.MustCompile(`^[^:]+:	+(.+?)\t+(model|KV|output|compute) buffer size =	+([0-9]+(?:\.[0-9]+)?)\t+([KMGT]i?B|[KMGT]B|B)\b`)
	llamaCppDeviceBreakdownRe = regexp.MustCompile(`^llama_memory_breakdown_print:\s+\|\s+-\s+(.+?)\s+\|\s+([0-9]+)\s*=\s*([0-9]+)\s*\+\s*\(\s*([0-9]+)\s*=\s*([0-9]+)\s*\+\s*([0-9]+)\s*\+\s*([0-9]+)\s*\)\s*\+\s*([0-9]+)\s*\|`)
	llamaCppOtherBreakdownRe  = regexp.MustCompile(`^llama_memory_breakdown_print:\s+\|\s+-\s+(.+?)\s+\|\s+([0-9]+)\s*=\s*([0-9]+)\s*\+\s*([0-9]+)\s*\+\s*([0-9]+)\s*\|`)
)

type llamaCppMemoryTracker struct {
	mu         sync.RWMutex
	partial    string
	components map[string]*LlamaCppMemoryComponent
	updatedAt  time.Time
	hasData    bool
}

func newLlamaCppMemoryTracker() *llamaCppMemoryTracker {
	return &llamaCppMemoryTracker{
		components: make(map[string]*LlamaCppMemoryComponent),
	}
}

func (t *llamaCppMemoryTracker) parseChunk(data []byte) {
	if len(data) == 0 {
		return
	}

	t.mu.Lock()
	buffer := t.partial + string(data)
	lines := strings.Split(buffer, "\n")
	t.partial = lines[len(lines)-1]
	if len(t.partial) > 64*1024 {
		t.partial = t.partial[len(t.partial)-64*1024:]
	}
	t.mu.Unlock()

	for _, line := range lines[:len(lines)-1] {
		t.parseLine(strings.TrimRight(line, "\r"))
	}
}

func (t *llamaCppMemoryTracker) parseLine(line string) {
	if matches := llamaCppBufferSizeRe.FindStringSubmatch(line); matches != nil {
		name := strings.TrimSpace(matches[1])
		bytes, ok := parseLlamaCppMemoryBytes(matches[3], matches[4])
		if !ok {
			return
		}
		component := t.component(name)
		switch matches[2] {
		case "model":
			component.ModelBytes = bytes
		case "KV":
			component.KVBytes = bytes
		case "output":
			component.OutputBytes = bytes
		case "compute":
			component.ComputeBytes = bytes
		}
		t.markUpdated()
		return
	}

	if matches := llamaCppDeviceBreakdownRe.FindStringSubmatch(line); matches != nil {
		name := strings.TrimSpace(matches[1])
		component := t.component(name)
		component.DeviceCapacityBytes = parseMiB(matches[2])
		component.DeviceFreeBytes = parseMiB(matches[3])
		if component.ModelBytes == 0 {
			component.ModelBytes = parseMiB(matches[5])
		}
		if component.KVBytes == 0 {
			component.KVBytes = parseMiB(matches[6])
		}
		if component.ComputeBytes == 0 {
			component.ComputeBytes = parseMiB(matches[7])
		}
		unaccounted := parseMiB(matches[8])
		component.UnaccountedBytes = &unaccounted
		t.markUpdated()
		return
	}

	if matches := llamaCppOtherBreakdownRe.FindStringSubmatch(line); matches != nil {
		name := strings.TrimSpace(matches[1])
		component := t.component(name)
		if component.ModelBytes == 0 {
			component.ModelBytes = parseMiB(matches[3])
		}
		if component.KVBytes == 0 {
			component.KVBytes = parseMiB(matches[4])
		}
		if component.ComputeBytes == 0 {
			component.ComputeBytes = parseMiB(matches[5])
		}
		t.markUpdated()
	}
}

func (t *llamaCppMemoryTracker) component(name string) *LlamaCppMemoryComponent {
	if component, ok := t.components[name]; ok {
		return component
	}
	component := &LlamaCppMemoryComponent{Name: name}
	t.components[name] = component
	return component
}

func (t *llamaCppMemoryTracker) markUpdated() {
	t.updatedAt = time.Now()
	t.hasData = true
}

func (t *llamaCppMemoryTracker) snapshot() *LlamaCppMemorySnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.hasData {
		return nil
	}

	snapshot := &LlamaCppMemorySnapshot{
		Source:    "llamacpp_logs",
		UpdatedAt: t.updatedAt,
	}

	for _, component := range t.components {
		copied := *component
		copied.TrackedBytes = copied.ModelBytes + copied.KVBytes + copied.ComputeBytes + copied.OutputBytes
		if component.UnaccountedBytes != nil {
			unaccounted := *component.UnaccountedBytes
			copied.UnaccountedBytes = &unaccounted
		}

		switch classifyLlamaCppMemoryComponent(copied.Name) {
		case "device":
			snapshot.DeviceTotalBytes += copied.TrackedBytes
			snapshot.Devices = append(snapshot.Devices, copied)
		case "host":
			snapshot.HostTotalBytes += copied.TrackedBytes
			snapshot.Host = append(snapshot.Host, copied)
		default:
			snapshot.Unknown = append(snapshot.Unknown, copied)
		}
		snapshot.TotalTrackedBytes += copied.TrackedBytes
	}
	return snapshot
}

func parseLlamaCppMemoryBytes(value, unit string) (uint64, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}

	multiplier := float64(1)
	switch strings.ToLower(unit) {
	case "kib":
		multiplier = 1024
	case "kb":
		multiplier = 1000
	case "mib":
		multiplier = 1024 * 1024
	case "mb":
		multiplier = 1000 * 1000
	case "gib":
		multiplier = 1024 * 1024 * 1024
	case "gb":
		multiplier = 1000 * 1000 * 1000
	case "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "tb":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "b":
		multiplier = 1
	default:
		return 0, false
	}
	return uint64(parsed*multiplier + 0.5), true
}

func parseMiB(value string) uint64 {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed * 1024 * 1024
}

func classifyLlamaCppMemoryComponent(name string) string {
	normalized := strings.TrimSpace(name)
	if normalized == "Host" || strings.HasPrefix(normalized, "CPU") || strings.Contains(normalized, "_Host") || strings.Contains(normalized, "_Mapped") {
		return "host"
	}

	for _, prefix := range []string{"CUDA", "ROCm", "Vulkan", "SYCL", "MUSA", "CANN", "Metal"} {
		if strings.HasPrefix(normalized, prefix) {
			return "device"
		}
	}
	return "unknown"
}

// specDecodeStats holds parsed speculative decoding statistics.
type specDecodeStats struct {
	AcceptanceRate  float64
	AcceptedDrafts  int
	GeneratedDrafts int
	ParsedAt        time.Time
}

// specDecodeParser parses llama-server stderr for speculative decoding stats.
type specDecodeParser struct {
	mu          sync.Mutex
	partialLine string
	re          *regexp.Regexp
	latest      *specDecodeStats
}

func newSpecDecodeParser() *specDecodeParser {
	return &specDecodeParser{
		re: regexp.MustCompile(
			`draft acceptance rate = ([\d.]+)\s*\(\s*(\d+)\s+accepted\s*/\s*(\d+)\s+generated\)`,
		),
	}
}

func (p *specDecodeParser) parseChunk(data []byte) bool {
	p.mu.Lock()
	buffer := p.partialLine + string(data)
	parts := strings.Split(buffer, "\n")
	p.partialLine = parts[len(parts)-1]
	p.mu.Unlock()

	found := false
	parsedAt := time.Now()
	for _, part := range parts[:len(parts)-1] {
		if stats, ok := p.parseLine(part, parsedAt); ok {
			p.mu.Lock()
			p.latest = &stats
			p.mu.Unlock()
			found = true
		}
	}

	p.mu.Lock()
	trailing := p.partialLine
	p.mu.Unlock()
	if trailing != "" {
		if stats, ok := p.parseLine(trailing, parsedAt); ok {
			p.mu.Lock()
			p.latest = &stats
			p.partialLine = ""
			p.mu.Unlock()
			found = true
		}
	}
	return found
}

func (p *specDecodeParser) parseLine(line string, parsedAt time.Time) (specDecodeStats, bool) {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "draft acceptance rate") {
		return specDecodeStats{}, false
	}

	matches := p.re.FindStringSubmatch(line)
	if len(matches) != 4 {
		return specDecodeStats{}, false
	}

	rate, err1 := strconv.ParseFloat(matches[1], 64)
	accepted, err2 := strconv.Atoi(matches[2])
	generated, err3 := strconv.Atoi(matches[3])

	if err1 != nil || err2 != nil || err3 != nil {
		return specDecodeStats{}, false
	}

	return specDecodeStats{
		AcceptanceRate:  rate,
		AcceptedDrafts:  accepted,
		GeneratedDrafts: generated,
		ParsedAt:        parsedAt,
	}, true
}

func (p *specDecodeParser) latestStats() *specDecodeStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.latest
}

// logParserBundle holds parsers for a single model and wires them to a tracker.
type logParserBundle struct {
	pp *promptProgressParser
	gt *generationTokenParser
	mt *llamaCppMemoryTracker
	sd *specDecodeParser
}

func newLogParserBundle(model string, tracker *liveActivityTracker) *logParserBundle {
	return &logParserBundle{
		pp: newPromptProgressParser(model),
		gt: newGenerationTokenParser(model),
		mt: newLlamaCppMemoryTracker(),
		sd: newSpecDecodeParser(),
	}
}

func (b *logParserBundle) parse(data []byte, tracker *liveActivityTracker) {
	b.pp.parseChunk(data, func(p promptProcessingProgress) {
		tracker.SetPromptProgress(p.Model, p.SlotID, p.TaskID, p.Progress, p.Speed)
	})
	b.gt.parseChunk(data, func(g generationProgress) {
		tracker.SetGeneratedTokens(g.Model, g.SlotID, g.TaskID, g.Decoded, g.Speed)
	})
	b.mt.parseChunk(data)
	if tracker != nil {
		if snapshot := b.mt.snapshot(); snapshot != nil {
			tracker.SetMemorySnapshot(b.pp.model, snapshot)
		}
	}
	if b.sd.parseChunk(data) && tracker != nil {
		if stats := b.sd.latestStats(); stats != nil {
			tracker.SetSpecDecodeStats(b.pp.model, stats.AcceptanceRate, stats.AcceptedDrafts, stats.GeneratedDrafts)
		}
	}
}

// parserRegistry lazily wires log parsers to model process loggers.
type parserRegistry struct {
	mu      sync.Mutex
	wired   map[string]bool
	local   router.LocalRouter
	tracker *liveActivityTracker
}

func newParserRegistry(local router.LocalRouter, tracker *liveActivityTracker) *parserRegistry {
	return &parserRegistry{
		wired:   make(map[string]bool),
		local:   local,
		tracker: tracker,
	}
}

func (r *parserRegistry) ensureWired(modelID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.wired[modelID] || r.local == nil || r.tracker == nil {
		return
	}
	logger, ok := r.local.ProcessLogger(modelID)
	if !ok {
		return
	}
	r.wired[modelID] = true
	bundle := newLogParserBundle(modelID, r.tracker)
	logger.OnLogData(func(data []byte) {
		bundle.parse(data, r.tracker)
	})
}
