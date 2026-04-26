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
