package proxy

import (
	"testing"
	"time"
)

func TestTokenStream_AppendAndSnapshot(t *testing.T) {
	ts := &tokenStream{}

	ts.append("content", "Hello")
	ts.append("reasoning", "thinking...")
	ts.append("content", " world")

	chunks, closed := ts.snapshot()
	if closed {
		t.Error("expected stream to be open")
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Kind != "content" || chunks[0].Text != "Hello" {
		t.Errorf("chunk 0 mismatch: %+v", chunks[0])
	}
	if chunks[1].Kind != "reasoning" || chunks[1].Text != "thinking..." {
		t.Errorf("chunk 1 mismatch: %+v", chunks[1])
	}
	if chunks[2].Kind != "content" || chunks[2].Text != " world" {
		t.Errorf("chunk 2 mismatch: %+v", chunks[2])
	}
}

func TestTokenStream_WaiterReceivesChunks(t *testing.T) {
	ts := &tokenStream{}

	waiter := ts.addWaiter()

	ts.append("content", "chunk1")

	select {
	case chunk := <-waiter:
		if chunk.Text != "chunk1" {
			t.Errorf("expected chunk1, got %s", chunk.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for chunk")
	}
}

func TestTokenStream_CloseSignalsWaiter(t *testing.T) {
	ts := &tokenStream{}

	waiter := ts.addWaiter()
	ts.close()

	select {
	case _, ok := <-waiter:
		if ok {
			t.Error("expected waiter channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for close")
	}
}

func TestTokenStream_SnapshotAfterClose(t *testing.T) {
	ts := &tokenStream{}
	ts.append("content", "a")
	ts.close()

	chunks, closed := ts.snapshot()
	if !closed {
		t.Error("expected stream to be closed")
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestLiveActivityTracker_TokenStreamLifecycle(t *testing.T) {
	tracker := newLiveActivityTracker()

	id := tracker.Start("test-model")
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	tracker.AppendToken(id, "content", "Hello")
	tracker.AppendToken(id, "reasoning", " hmm")
	tracker.AppendToken(id, "content", " world")

	stream := tracker.GetTokenStream(id)
	if stream == nil {
		t.Fatal("expected token stream to exist")
	}

	chunks, closed := stream.snapshot()
	if closed {
		t.Error("expected stream to be open")
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}

	tracker.Finish(id)

	// Stream should be closed after Finish
	_, closed = stream.snapshot()
	if !closed {
		t.Error("expected stream to be closed after Finish")
	}
}

func TestLiveActivityTracker_TokenStreamAfterFinish(t *testing.T) {
	tracker := newLiveActivityTracker()

	id := tracker.Start("test-model")
	tracker.AppendToken(id, "content", "Hello")
	tracker.Finish(id)

	// Stream stays alive briefly after Finish; appends are still accepted
	tracker.AppendToken(id, "content", " post-finish")

	stream := tracker.GetTokenStream(id)
	if stream == nil {
		t.Fatal("expected stream to still exist shortly after Finish")
	}

	chunks, closed := stream.snapshot()
	if !closed {
		t.Error("expected stream to be closed after Finish")
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestLiveActivityTracker_GetTokenStream_UnknownID(t *testing.T) {
	tracker := newLiveActivityTracker()
	if tracker.GetTokenStream("nonexistent") != nil {
		t.Error("expected nil for unknown stream id")
	}
}

func TestLiveActivityTracker_AppendToken_NoStream(t *testing.T) {
	tracker := newLiveActivityTracker()
	// Should not panic
	tracker.AppendToken("nonexistent", "content", "text")
}
