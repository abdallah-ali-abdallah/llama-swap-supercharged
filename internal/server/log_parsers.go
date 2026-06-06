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

// logParserBundle holds parsers for a single model and wires them to a tracker.
type logParserBundle struct {
	pp  *promptProgressParser
	gt  *generationTokenParser
}

func newLogParserBundle(model string, tracker *liveActivityTracker) *logParserBundle {
	return &logParserBundle{
		pp: newPromptProgressParser(model),
		gt: newGenerationTokenParser(model),
	}
}

func (b *logParserBundle) parse(data []byte, tracker *liveActivityTracker) {
	b.pp.parseChunk(data, func(p promptProcessingProgress) {
		tracker.SetPromptProgress(p.Model, p.SlotID, p.TaskID, p.Progress, p.Speed)
	})
	b.gt.parseChunk(data, func(g generationProgress) {
		tracker.SetGeneratedTokens(g.Model, g.SlotID, g.TaskID, g.Decoded, g.Speed)
	})
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
