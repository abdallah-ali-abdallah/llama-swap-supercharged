package proxy

import (
	"testing"
	"time"
)

func TestSpecDecodeParser_parseLine(t *testing.T) {
	parsedAt := time.Now()
	tests := []struct {
		name      string
		line      string
		found     bool
		rate      float64
		accepted  int
		generated int
	}{
		{
			name:      "parses standard draft acceptance rate",
			line:      "draft acceptance rate = 1.00000 (    4 accepted /     4 generated)",
			found:     true,
			rate:      1.0,
			accepted:  4,
			generated: 4,
		},
		{
			name:      "parses fractional rate",
			line:      "draft acceptance rate = 0.75000 (   12 accepted /    16 generated)",
			found:     true,
			rate:      0.75,
			accepted:  12,
			generated: 16,
		},
		{
			name:  "ignores non-draft lines",
			line:  "slot print_timing: id  0 | task 1234 |",
			found: false,
		},
		{
			name:  "ignores empty line",
			line:  "",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := newSpecDecodeParser(nil)
			stats, found := parser.parseLine(tt.line, parsedAt)

			if found != tt.found {
				t.Fatalf("parseLine() found = %v, want %v", found, tt.found)
			}
			if !found {
				return
			}

			if stats.AcceptanceRate != tt.rate {
				t.Fatalf("AcceptanceRate = %v, want %v", stats.AcceptanceRate, tt.rate)
			}
			if stats.AcceptedDrafts != tt.accepted {
				t.Fatalf("AcceptedDrafts = %v, want %v", stats.AcceptedDrafts, tt.accepted)
			}
			if stats.GeneratedDrafts != tt.generated {
				t.Fatalf("GeneratedDrafts = %v, want %v", stats.GeneratedDrafts, tt.generated)
			}
			if !stats.ParsedAt.Equal(parsedAt) {
				t.Fatalf("ParsedAt = %v, want %v", stats.ParsedAt, parsedAt)
			}
		})
	}
}

func TestSpecDecodeParser_parseChunk(t *testing.T) {
	parser := newSpecDecodeParser(nil)

	if parser.parseChunk([]byte("draft acceptance")) {
		t.Fatal("expected partial chunk not to match")
	}
	if !parser.parseChunk([]byte(" rate = 0.75000 (    6 accepted /     8 generated)")) {
		t.Fatal("expected completed chunk to match")
	}

	stats, ok := parser.consumeClosestTo(time.Now(), 0)
	if !ok {
		t.Fatal("expected queued draft stats")
	}
	if stats.AcceptanceRate != 0.75 {
		t.Fatalf("AcceptanceRate = %v, want 0.75", stats.AcceptanceRate)
	}
	if stats.AcceptedDrafts != 6 {
		t.Fatalf("AcceptedDrafts = %v, want 6", stats.AcceptedDrafts)
	}
	if stats.GeneratedDrafts != 8 {
		t.Fatalf("GeneratedDrafts = %v, want 8", stats.GeneratedDrafts)
	}
}

func TestSpecDecodeParser_consumeClosestTo(t *testing.T) {
	parser := newSpecDecodeParser(nil)
	requestEnd := time.Now()

	if !parser.queueLine("draft acceptance rate = 0.50000 (    2 accepted /     4 generated)", requestEnd.Add(-300*time.Millisecond)) {
		t.Fatal("expected first line to queue")
	}
	if !parser.queueLine("draft acceptance rate = 0.80000 (    8 accepted /    10 generated)", requestEnd.Add(-40*time.Millisecond)) {
		t.Fatal("expected second line to queue")
	}

	stats, ok := parser.consumeClosestTo(requestEnd, 0)
	if !ok {
		t.Fatal("expected closest draft stats")
	}
	if stats.AcceptanceRate != 0.8 {
		t.Fatalf("AcceptanceRate = %v, want 0.8", stats.AcceptanceRate)
	}

	remaining, ok := parser.consumeClosestTo(requestEnd, 0)
	if !ok {
		t.Fatal("expected remaining draft stats")
	}
	if remaining.AcceptanceRate != 0.5 {
		t.Fatalf("AcceptanceRate = %v, want 0.5", remaining.AcceptanceRate)
	}
}
