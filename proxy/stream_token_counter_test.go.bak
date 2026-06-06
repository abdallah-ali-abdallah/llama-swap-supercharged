package proxy

import (
	"fmt"
	"testing"
	"time"
)

func TestStreamTokenCounter_ParseOpenAIContent(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	// Simulate llama.cpp OpenAI-compatible SSE chunk
	sse := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
	counter.Write(sse)

	// Allow append to propagate
	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	if stream == nil {
		t.Fatal("token stream not found")
	}
	chunks, _ := stream.snapshot()
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Kind != "content" || chunks[0].Text != "Hello" {
		t.Errorf("unexpected chunk: %+v", chunks[0])
	}
}

func TestStreamTokenCounter_ParseReasoningContent(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	sse := []byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"Let me think\",\"content\":\"\"}}]}\n\n")
	counter.Write(sse)

	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	chunks, _ := stream.snapshot()
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Kind != "reasoning" || chunks[0].Text != "Let me think" {
		t.Errorf("unexpected chunk: %+v", chunks[0])
	}
}

func TestStreamTokenCounter_MultipleLines(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	// Multiple SSE lines in one Write
	sse := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\n\n")
	counter.Write(sse)

	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	chunks, _ := stream.snapshot()
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Text != "A" || chunks[1].Text != "B" {
		t.Errorf("chunks mismatch: %+v", chunks)
	}
}

func TestStreamTokenCounter_IgnoresNonContent(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	// A line with no content field
	sse := []byte("data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\n")
	counter.Write(sse)

	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	chunks, _ := stream.snapshot()
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks, got %d", len(chunks))
	}
}

func TestStreamTokenCounter_WithRealLlamaCppSSEFormat(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	// Real llama.cpp SSE format
	lines := []string{
		`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1700000000,"model":"llama-model","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1700000000,"model":"llama-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1700000000,"model":"llama-model","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
		``,
		`data: [DONE]`,
	}

	for _, line := range lines {
		counter.Write([]byte(line + "\n"))
	}

	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	chunks, _ := stream.snapshot()
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Text != "Hello" || chunks[1].Text != " world" {
		t.Errorf("unexpected chunks: %+v", chunks)
	}
}

func TestStreamTokenCounter_BatchedWrites(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	// Simulate chunked writes where a single data line is split across writes
	counter.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel"))
	counter.Write([]byte("lo\"}}]}\n\n"))

	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	chunks, _ := stream.snapshot()
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != "Hello" {
		t.Errorf("unexpected chunk: %+v", chunks[0])
	}
}

func TestStreamTokenCounter_LlamaCppNativeFormat(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	// llama.cpp native /completion format with "content" at root
	sse := []byte("data: {\"content\":\"Native\"}\n\n")
	counter.Write(sse)

	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	chunks, _ := stream.snapshot()
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != "Native" {
		t.Errorf("unexpected chunk: %+v", chunks[0])
	}
}

func TestStreamTokenCounter_ResponseFieldFormat(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")
	counter := newStreamTokenCounter(tracker, id)

	// Anthropic-style or older format with "response" field
	sse := []byte("data: {\"response\":\"Answer\"}\n\n")
	counter.Write(sse)

	time.Sleep(10 * time.Millisecond)

	stream := tracker.GetTokenStream(id)
	chunks, _ := stream.snapshot()
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != "Answer" {
		t.Errorf("unexpected chunk: %+v", chunks[0])
	}
}

func TestLiveTokenStreamSSE(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")

	// Simulate what the API endpoint does
	var received []tokenStreamChunk
	waiter := tracker.GetTokenStream(id).addWaiter()
	go func() {
		for chunk := range waiter {
			received = append(received, chunk)
		}
	}()

	counter := newStreamTokenCounter(tracker, id)
	counter.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"X\"}}]}\n\n"))
	counter.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Y\"}}]}\n\n"))

	time.Sleep(50 * time.Millisecond)
	tracker.Finish(id)
	time.Sleep(50 * time.Millisecond)

	if len(received) != 2 {
		t.Fatalf("expected 2 SSE chunks, got %d: %+v", len(received), received)
	}
	fmt.Printf("received: %+v\n", received)
}
