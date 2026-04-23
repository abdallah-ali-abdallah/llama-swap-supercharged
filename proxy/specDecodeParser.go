package proxy

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	specDecodeMatchWindow  = 2 * time.Second
	specDecodeWaitTimeout  = 150 * time.Millisecond
	specDecodePollInterval = 10 * time.Millisecond
	specDecodeMaxPending   = 128
)

// specDecodeParser subscribes to llama-server log output and queues
// speculative decoding statistics near request completion.
type specDecodeParser struct {
	mu sync.Mutex

	// regex for parsing: "draft acceptance rate = 1.00000 (    4 accepted /     4 generated)"
	reDraftAcceptance *regexp.Regexp

	partialLine string
	pending     []draftStats
}

type draftStats struct {
	AcceptanceRate  float64
	AcceptedDrafts  int
	GeneratedDrafts int
	ParsedAt        time.Time
}

// newSpecDecodeParser creates a new spec decode parser.
func newSpecDecodeParser(_ *LogMonitor) *specDecodeParser {
	return &specDecodeParser{
		reDraftAcceptance: regexp.MustCompile(
			`draft acceptance rate = ([\d.]+)\s*\(\s*(\d+)\s+accepted\s*/\s*(\d+)\s+generated\)`,
		),
		pending: make([]draftStats, 0, 8),
	}
}

// parseChunk processes streamed log bytes and queues any complete spec decode lines.
func (p *specDecodeParser) parseChunk(data []byte) bool {
	p.mu.Lock()
	buffer := p.partialLine + string(data)
	parts := strings.Split(buffer, "\n")
	p.partialLine = parts[len(parts)-1]
	p.mu.Unlock()

	found := false
	parsedAt := time.Now()
	for _, part := range parts[:len(parts)-1] {
		if p.queueLine(part, parsedAt) {
			found = true
		}
	}

	// Logger writes often deliver complete lines without a trailing newline.
	p.mu.Lock()
	trailing := p.partialLine
	p.mu.Unlock()
	if trailing != "" && p.queueLine(trailing, parsedAt) {
		p.mu.Lock()
		p.partialLine = ""
		p.mu.Unlock()
		found = true
	}

	return found
}

func (p *specDecodeParser) queueLine(line string, parsedAt time.Time) bool {
	stats, ok := p.parseLine(line, parsedAt)
	if !ok {
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.pending = append(p.pending, stats)
	if len(p.pending) > specDecodeMaxPending {
		p.pending = p.pending[len(p.pending)-specDecodeMaxPending:]
	}
	return true
}

// parseLine processes a single log line for speculative decoding stats.
func (p *specDecodeParser) parseLine(line string, parsedAt time.Time) (draftStats, bool) {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, "draft acceptance rate") {
		return draftStats{}, false
	}

	matches := p.reDraftAcceptance.FindStringSubmatch(line)
	if len(matches) != 4 {
		return draftStats{}, false
	}

	rate, err1 := strconv.ParseFloat(matches[1], 64)
	accepted, err2 := strconv.Atoi(matches[2])
	generated, err3 := strconv.Atoi(matches[3])

	if err1 != nil || err2 != nil || err3 != nil {
		return draftStats{}, false
	}

	return draftStats{
		AcceptanceRate:  rate,
		AcceptedDrafts:  accepted,
		GeneratedDrafts: generated,
		ParsedAt:        parsedAt,
	}, true
}

// consumeClosestTo returns the closest queued stats to a request end time.
func (p *specDecodeParser) consumeClosestTo(requestEnd time.Time, wait time.Duration) (draftStats, bool) {
	deadline := time.Now().Add(wait)

	for {
		if stats, ok := p.takeClosestMatch(requestEnd); ok {
			return stats, true
		}
		if time.Now().After(deadline) {
			return draftStats{}, false
		}

		time.Sleep(specDecodePollInterval)
	}
}

func (p *specDecodeParser) takeClosestMatch(requestEnd time.Time) (draftStats, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	kept := p.pending[:0]
	for _, stats := range p.pending {
		if now.Sub(stats.ParsedAt) <= specDecodeMatchWindow*4 {
			kept = append(kept, stats)
		}
	}
	p.pending = kept

	bestIdx := -1
	bestDelta := specDecodeMatchWindow + time.Nanosecond
	for i, stats := range p.pending {
		delta := stats.ParsedAt.Sub(requestEnd)
		if delta < 0 {
			delta = -delta
		}
		if delta > specDecodeMatchWindow {
			continue
		}
		if delta < bestDelta {
			bestIdx = i
			bestDelta = delta
		}
	}
	if bestIdx == -1 {
		return draftStats{}, false
	}

	stats := p.pending[bestIdx]
	copy(p.pending[bestIdx:], p.pending[bestIdx+1:])
	p.pending = p.pending[:len(p.pending)-1]
	return stats, true
}
