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

	tracker.SetPromptProgress("test-model", 0.47)

	rows := tracker.Snapshot()
	require.Len(t, rows, 1)
	require.Equal(t, id, rows[0].ID)
	require.Equal(t, "test-model", rows[0].Model)
	require.True(t, rows[0].PPExact)
	require.NotNil(t, rows[0].PPProgress)
	require.InDelta(t, 0.47, *rows[0].PPProgress, 0.000001)

	tracker.Finish(id)
	require.Empty(t, tracker.Snapshot())
}

func TestProxyManager_LiveActivityMarksOverlappingSameModelProgressUnknown(t *testing.T) {
	tracker := newLiveActivityTracker()
	tracker.Start("test-model")
	tracker.SetPromptProgress("test-model", 0.25)
	tracker.Start("test-model")

	tracker.SetPromptProgress("test-model", 0.47)

	rows := tracker.Snapshot()
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, "test-model", row.Model)
		require.False(t, row.PPExact)
		require.Nil(t, row.PPProgress)
	}
}
