package proxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPromptProgressParser_ParseLine(t *testing.T) {
	parser := newPromptProgressParser("test-model")

	progress, ok := parser.parseLine(
		"slot update_slots: id  2 | task 461 | prompt processing progress, n_tokens = 25904, batch.n_tokens = 4096, progress = 0.470913",
		time.Now(),
	)

	require.True(t, ok)
	require.Equal(t, "test-model", progress.Model)
	require.Equal(t, 2, progress.SlotID)
	require.Equal(t, 461, progress.TaskID)
	require.Equal(t, 25904, progress.Tokens)
	require.Equal(t, 4096, progress.BatchTokens)
	require.InDelta(t, 0.470913, progress.Progress, 0.000001)
	require.Equal(t, 0.0, progress.Speed)
}

func TestPromptProgressParser_ParseLine_NewFormat(t *testing.T) {
	parser := newPromptProgressParser("test-model")

	progress, ok := parser.parseLine(
		"slot print_timing: id  0 | task 0 | prompt processing, n_tokens =   2462, progress = 1.00, t =   4.73 s / 521.04 tokens per second",
		time.Now(),
	)

	require.True(t, ok)
	require.Equal(t, "test-model", progress.Model)
	require.Equal(t, 0, progress.SlotID)
	require.Equal(t, 0, progress.TaskID)
	require.Equal(t, 2462, progress.Tokens)
	require.Equal(t, 2462, progress.BatchTokens)
	require.InDelta(t, 1.00, progress.Progress, 0.000001)
	require.InDelta(t, 521.04, progress.Speed, 0.000001)
}

func TestPromptProgressParser_ParseLine_NewFormatNoSpeed(t *testing.T) {
	parser := newPromptProgressParser("test-model")

	progress, ok := parser.parseLine(
		"slot print_timing: id  0 | task 0 | prompt processing, n_tokens =   2462, progress = 1.00",
		time.Now(),
	)

	require.True(t, ok)
	require.Equal(t, 2462, progress.Tokens)
	require.InDelta(t, 1.00, progress.Progress, 0.000001)
	require.Equal(t, 0.0, progress.Speed)
}

func TestPromptProgressParser_ParseLine_NewFormatWithPrefix(t *testing.T) {
	parser := newPromptProgressParser("test-model")

	progress, ok := parser.parseLine(
		"0.20.012.814 I slot print_timing: id  0 | task 6815 | prompt processing, n_tokens = 500, progress = 0.50, t = 1.00 s / 500.00 tokens per second",
		time.Now(),
	)

	require.True(t, ok)
	require.Equal(t, 0, progress.SlotID)
	require.Equal(t, 6815, progress.TaskID)
	require.Equal(t, 500, progress.Tokens)
	require.InDelta(t, 0.50, progress.Progress, 0.000001)
	require.InDelta(t, 500.00, progress.Speed, 0.000001)
}

func TestPromptProgressParser_IgnoresMalformedLines(t *testing.T) {
	parser := newPromptProgressParser("test-model")

	_, ok := parser.parseLine("slot update_slots: id  2 | task 461 | n_tokens = 25904, memory_seq_rm [25904, end)", time.Now())

	require.False(t, ok)
}

func TestPromptProgressParser_ClampsProgress(t *testing.T) {
	parser := newPromptProgressParser("test-model")

	progress, ok := parser.parseLine(
		"slot update_slots: id  2 | task 461 | prompt processing progress, n_tokens = 25904, batch.n_tokens = 4096, progress = 1.25",
		time.Now(),
	)

	require.True(t, ok)
	require.Equal(t, 1.0, progress.Progress)
}

func TestPromptProgressParser_ClampsProgress_NewFormat(t *testing.T) {
	parser := newPromptProgressParser("test-model")

	progress, ok := parser.parseLine(
		"slot print_timing: id  0 | task 0 | prompt processing, n_tokens = 100, progress = 1.50, t = 0.10 s / 1000.00 tokens per second",
		time.Now(),
	)

	require.True(t, ok)
	require.Equal(t, 1.0, progress.Progress)
}

func TestProxyManager_LiveActivityTracksSingleModelProgress(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")

	tracker.SetPromptProgress(promptProcessingProgress{Model: "test-model", SlotID: 0, TaskID: 0, Progress: 0.47, Speed: 100.5})

	rows := tracker.Snapshot()
	require.Len(t, rows, 1)
	require.Equal(t, id, rows[0].ID)
	require.Equal(t, "test-model", rows[0].Model)
	require.True(t, rows[0].PPExact)
	require.NotNil(t, rows[0].PPProgress)
	require.InDelta(t, 0.47, *rows[0].PPProgress, 0.000001)
	require.NotNil(t, rows[0].PPSpeed)
	require.InDelta(t, 100.5, *rows[0].PPSpeed, 0.000001)
	require.NotNil(t, rows[0].SlotID)
	require.Equal(t, 0, *rows[0].SlotID)
	require.NotNil(t, rows[0].TaskID)
	require.Equal(t, 0, *rows[0].TaskID)

	tracker.Finish(id)
	require.Empty(t, tracker.Snapshot())
}

func TestProxyManager_LiveActivityTracksMultipleSameModelProgress(t *testing.T) {
	tracker := newLiveActivityTracker()
	id1 := tracker.Start("test-model")
	id2 := tracker.Start("test-model")

	// First request gets slot 0, task 0
	tracker.SetPromptProgress(promptProcessingProgress{Model: "test-model", SlotID: 0, TaskID: 0, Progress: 0.25, Speed: 120.0})
	// Second request gets slot 0, task 1
	tracker.SetPromptProgress(promptProcessingProgress{Model: "test-model", SlotID: 0, TaskID: 1, Progress: 0.60, Speed: 200.0})

	rows := tracker.Snapshot()
	require.Len(t, rows, 2)

	row1 := rows[0]
	row2 := rows[1]
	if row1.ID != id1 {
		row1, row2 = row2, row1
	}

	require.Equal(t, id1, row1.ID)
	require.True(t, row1.PPExact)
	require.NotNil(t, row1.PPProgress)
	require.InDelta(t, 0.25, *row1.PPProgress, 0.000001)
	require.NotNil(t, row1.PPSpeed)
	require.InDelta(t, 120.0, *row1.PPSpeed, 0.000001)
	require.NotNil(t, row1.SlotID)
	require.Equal(t, 0, *row1.SlotID)
	require.NotNil(t, row1.TaskID)
	require.Equal(t, 0, *row1.TaskID)

	require.Equal(t, id2, row2.ID)
	require.True(t, row2.PPExact)
	require.NotNil(t, row2.PPProgress)
	require.InDelta(t, 0.60, *row2.PPProgress, 0.000001)
	require.NotNil(t, row2.PPSpeed)
	require.InDelta(t, 200.0, *row2.PPSpeed, 0.000001)
	require.NotNil(t, row2.SlotID)
	require.Equal(t, 0, *row2.SlotID)
	require.NotNil(t, row2.TaskID)
	require.Equal(t, 1, *row2.TaskID)
}

func TestLiveActivityTracker_UpdateGeneratedTokens(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")

	tracker.UpdateGeneratedTokens(id, 10)
	rows := tracker.Snapshot()
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].GeneratedTokens)
	require.Equal(t, 10, *rows[0].GeneratedTokens)

	// Same value should not change state
	tracker.UpdateGeneratedTokens(id, 10)
	rows2 := tracker.Snapshot()
	require.Equal(t, *rows[0].GeneratedTokens, *rows2[0].GeneratedTokens)

	tracker.UpdateGeneratedTokens(id, 25)
	rows = tracker.Snapshot()
	require.Equal(t, 25, *rows[0].GeneratedTokens)

	tracker.Finish(id)
	require.Empty(t, tracker.Snapshot())
}

func TestGenerationTokenParser_ParseLine(t *testing.T) {
	parser := newGenerationTokenParser("test-model")

	progress, ok := parser.parseLine(
		"slot print_timing: id  0 | task 0 | n_decoded =    100, tg =  44.95 t/s",
	)

	require.True(t, ok)
	require.Equal(t, 100, progress.Decoded)
	require.InDelta(t, 44.95, progress.Speed, 0.000001)
	require.Equal(t, 0, progress.SlotID)
	require.Equal(t, 0, progress.TaskID)
	require.Equal(t, "test-model", progress.Model)
}

func TestGenerationTokenParser_ParseLine_WithPrefix(t *testing.T) {
	parser := newGenerationTokenParser("test-model")

	progress, ok := parser.parseLine(
		"0.17.821.721 I slot print_timing: id  0 | task 0 | n_decoded =    228, tg =  43.58 t/s",
	)

	require.True(t, ok)
	require.Equal(t, 228, progress.Decoded)
	require.InDelta(t, 43.58, progress.Speed, 0.000001)
}

func TestGenerationTokenParser_ParseLine_WithoutSpeed(t *testing.T) {
	parser := newGenerationTokenParser("test-model")

	progress, ok := parser.parseLine(
		"slot print_timing: id  0 | task 0 | n_decoded =    100",
	)

	require.True(t, ok)
	require.Equal(t, 100, progress.Decoded)
	require.Equal(t, 0.0, progress.Speed)
}

func TestGenerationTokenParser_IgnoresPromptProcessingLine(t *testing.T) {
	parser := newGenerationTokenParser("test-model")

	_, ok := parser.parseLine(
		"slot print_timing: id  0 | task 0 | prompt processing, n_tokens =   2709, progress = 1.00, t =   4.82 s / 561.87 tokens per second",
	)

	require.False(t, ok)
}

func TestGenerationTokenParser_IgnoresMalformedLines(t *testing.T) {
	parser := newGenerationTokenParser("test-model")

	_, ok := parser.parseLine("slot update_slots: id  0 | task 0 | prompt processing progress, n_tokens = 25904")
	require.False(t, ok)
}

func TestGenerationTokenParser_ParseChunk_SingleLine(t *testing.T) {
	parser := newGenerationTokenParser("test-model")
	var results []generationProgress

	parser.parseChunk([]byte("slot print_timing: id  0 | task 0 | n_decoded =    100, tg =  44.95 t/s\n"), func(p generationProgress) {
		require.Equal(t, "test-model", p.Model)
		results = append(results, p)
	})

	require.Len(t, results, 1)
	require.Equal(t, 100, results[0].Decoded)
}

func TestGenerationTokenParser_ParseChunk_MultipleLines(t *testing.T) {
	parser := newGenerationTokenParser("test-model")
	var results []generationProgress

	data := []byte(
		"slot print_timing: id  0 | task 0 | n_decoded =    100, tg =  44.95 t/s\n" +
			"0.17.821.721 I slot print_timing: id  0 | task 0 | n_decoded =    228, tg =  43.58 t/s\n",
	)
	parser.parseChunk(data, func(p generationProgress) {
		results = append(results, p)
	})

	require.Len(t, results, 2)
	require.Equal(t, 100, results[0].Decoded)
	require.Equal(t, 228, results[1].Decoded)
}

func TestGenerationTokenParser_ParseChunk_SplitLine(t *testing.T) {
	parser := newGenerationTokenParser("test-model")
	var results []generationProgress

	// first chunk is an incomplete line
	parser.parseChunk([]byte("slot print_timing: id  0 | task 0 | n_decoded =    10"), func(p generationProgress) {
		results = append(results, p)
	})
	require.Empty(t, results)

	// second chunk completes the line
	parser.parseChunk([]byte("0, tg =  44.95 t/s\n"), func(p generationProgress) {
		results = append(results, p)
	})
	require.Len(t, results, 1)
	require.Equal(t, 100, results[0].Decoded)
}

func TestLiveActivityTracker_UpdateGeneratedTokens_SuppressedWhenExact(t *testing.T) {
	tracker := newLiveActivityTracker()
	id := tracker.Start("test-model")

	// Approximate count arrives first via SSE
	tracker.UpdateGeneratedTokens(id, 10)
	rows := tracker.Snapshot()
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].GeneratedTokens)
	require.Equal(t, 10, *rows[0].GeneratedTokens)
	require.False(t, rows[0].generationExact)

	// Exact count arrives from llama.cpp logs
	tracker.SetGeneratedTokens(generationProgress{Model: "test-model", SlotID: 0, TaskID: 0, Decoded: 42, Speed: 30.5})
	rows = tracker.Snapshot()
	require.Equal(t, 42, *rows[0].GeneratedTokens)
	require.True(t, rows[0].generationExact)
	require.NotNil(t, rows[0].TGSpeed)
	require.InDelta(t, 30.5, *rows[0].TGSpeed, 0.000001)

	// Subsequent SSE approximation is ignored
	tracker.UpdateGeneratedTokens(id, 15)
	rows = tracker.Snapshot()
	require.Equal(t, 42, *rows[0].GeneratedTokens)
	require.True(t, rows[0].generationExact)

	tracker.Finish(id)
	require.Empty(t, tracker.Snapshot())
}

func TestLiveActivityTracker_SetGeneratedTokens(t *testing.T) {
	tracker := newLiveActivityTracker()
	tracker.Start("test-model")

	tracker.SetGeneratedTokens(generationProgress{Model: "test-model", SlotID: 0, TaskID: 0, Decoded: 42, Speed: 28.5})
	rows := tracker.Snapshot()
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].GeneratedTokens)
	require.Equal(t, 42, *rows[0].GeneratedTokens)
	require.True(t, rows[0].generationExact)
	require.NotNil(t, rows[0].TGSpeed)
	require.InDelta(t, 28.5, *rows[0].TGSpeed, 0.000001)

	// Duplicate value should not emit
	tracker.SetGeneratedTokens(generationProgress{Model: "test-model", SlotID: 0, TaskID: 0, Decoded: 42, Speed: 28.5})
	rows2 := tracker.Snapshot()
	require.Equal(t, rows[0].UpdatedAt, rows2[0].UpdatedAt)

	tracker.SetGeneratedTokens(generationProgress{Model: "test-model", SlotID: 0, TaskID: 0, Decoded: 100, Speed: 29.0})
	rows = tracker.Snapshot()
	require.Equal(t, 100, *rows[0].GeneratedTokens)
	require.True(t, rows[0].generationExact)
}

func TestLiveActivityTracker_SetGeneratedTokens_MultipleRequestsBySlotTask(t *testing.T) {
	tracker := newLiveActivityTracker()
	id1 := tracker.Start("test-model")
	id2 := tracker.Start("test-model")

	// Different task IDs map to different rows.
	tracker.SetGeneratedTokens(generationProgress{Model: "test-model", SlotID: 0, TaskID: 0, Decoded: 50, Speed: 30.0})
	tracker.SetGeneratedTokens(generationProgress{Model: "test-model", SlotID: 0, TaskID: 1, Decoded: 75, Speed: 25.0})

	rows := tracker.Snapshot()
	require.Len(t, rows, 2)

	row1 := rows[0]
	row2 := rows[1]
	if row1.ID != id1 {
		row1, row2 = row2, row1
	}

	require.Equal(t, id1, row1.ID)
	require.NotNil(t, row1.GeneratedTokens)
	require.Equal(t, 50, *row1.GeneratedTokens)
	require.NotNil(t, row1.TGSpeed)
	require.InDelta(t, 30.0, *row1.TGSpeed, 0.000001)

	require.Equal(t, id2, row2.ID)
	require.NotNil(t, row2.GeneratedTokens)
	require.Equal(t, 75, *row2.GeneratedTokens)
	require.NotNil(t, row2.TGSpeed)
	require.InDelta(t, 25.0, *row2.TGSpeed, 0.000001)
}

func TestLiveActivityTracker_FinishClearsSlotTaskMapping(t *testing.T) {
	tracker := newLiveActivityTracker()
	id1 := tracker.Start("test-model")

	tracker.SetPromptProgress(promptProcessingProgress{Model: "test-model", SlotID: 0, TaskID: 0, Progress: 0.5, Speed: 100.0})
	tracker.Finish(id1)

	// A new request with the same slot/task should not get the old mapping.
	id2 := tracker.Start("test-model")
	tracker.SetPromptProgress(promptProcessingProgress{Model: "test-model", SlotID: 0, TaskID: 0, Progress: 0.8, Speed: 200.0})

	rows := tracker.Snapshot()
	require.Len(t, rows, 1)
	require.Equal(t, id2, rows[0].ID)
	require.InDelta(t, 0.8, *rows[0].PPProgress, 0.000001)
}

func TestLiveActivityTracker_PromptProgressThenGenerationForSameSlotTask(t *testing.T) {
	tracker := newLiveActivityTracker()
	tracker.Start("test-model")

	tracker.SetPromptProgress(promptProcessingProgress{Model: "test-model", SlotID: 0, TaskID: 0, Progress: 1.0, Speed: 500.0})
	tracker.SetGeneratedTokens(generationProgress{Model: "test-model", SlotID: 0, TaskID: 0, Decoded: 100, Speed: 30.0})

	rows := tracker.Snapshot()
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].PPProgress)
	require.NotNil(t, rows[0].PPSpeed)
	require.NotNil(t, rows[0].GeneratedTokens)
	require.NotNil(t, rows[0].TGSpeed)
	require.Equal(t, 100, *rows[0].GeneratedTokens)
	require.InDelta(t, 30.0, *rows[0].TGSpeed, 0.000001)
}
