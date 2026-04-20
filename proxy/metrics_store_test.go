package proxy

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMetricsStore_PersistsAndQueriesRanges(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStore(path, 30, 100, logger)
	require.NoError(t, err)
	defer store.close()

	base := time.Now().Add(-2 * time.Hour).Truncate(time.Millisecond)
	metrics := []TokenMetrics{
		{ID: 0, Timestamp: base, Model: "model-a", NewInputTokens: 10, OutputTokens: 5, TokensPerSecond: 20},
		{ID: 1, Timestamp: base.Add(90 * time.Minute), Model: "model-b", CachedTokens: 4, NewInputTokens: 6, OutputTokens: 3, TokensPerSecond: 30},
		{ID: 2, Timestamp: base.Add(110 * time.Minute), Model: "model-a", NewInputTokens: 8, OutputTokens: 4, TokensPerSecond: 40},
	}
	for _, metric := range metrics {
		require.NoError(t, store.insert(metric))
	}

	maxID, err := store.maxID()
	require.NoError(t, err)
	require.Equal(t, 2, maxID)

	latest, err := store.latest(2)
	require.NoError(t, err)
	require.Len(t, latest, 2)
	require.Equal(t, 1, latest[0].ID)
	require.Equal(t, 2, latest[1].ID)

	from := base.Add(time.Hour)
	found, truncated, err := store.query(metricsQuery{From: &from, Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, found, 2)
	require.Equal(t, []int{1, 2}, []int{found[0].ID, found[1].ID})

	limited, truncated, err := store.query(metricsQuery{Limit: 2})
	require.NoError(t, err)
	require.True(t, truncated)
	require.Len(t, limited, 2)
	require.Equal(t, []int{0, 1}, []int{limited[0].ID, limited[1].ID})
}

func TestMetricsStore_AppliesRetention(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStore(path, 0, 100, logger)
	require.NoError(t, err)

	oldMetric := TokenMetrics{ID: 0, Timestamp: time.Now().Add(-48 * time.Hour), Model: "old", NewInputTokens: 1}
	newMetric := TokenMetrics{ID: 1, Timestamp: time.Now(), Model: "new", NewInputTokens: 1}
	require.NoError(t, store.insert(oldMetric))
	require.NoError(t, store.insert(newMetric))
	store.close()

	store, err = newMetricsStore(path, 1, 100, logger)
	require.NoError(t, err)
	defer store.close()

	metrics, truncated, err := store.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.Equal(t, "new", metrics[0].Model)
}

func TestMetricsMonitor_RestoresPersistedMetrics(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStore(path, 30, 100, logger)
	require.NoError(t, err)

	monitor := newMetricsMonitor(logger, 10, 0, store)
	require.Equal(t, 0, monitor.addMetrics(TokenMetrics{
		Timestamp:       time.Now(),
		Model:           "model-a",
		NewInputTokens:  10,
		OutputTokens:    5,
		TokensPerSecond: 20,
	}))
	require.Equal(t, 1, monitor.addMetrics(TokenMetrics{
		Timestamp:       time.Now(),
		Model:           "model-b",
		NewInputTokens:  8,
		OutputTokens:    4,
		TokensPerSecond: 30,
	}))
	monitor.close()

	store, err = newMetricsStore(path, 30, 100, logger)
	require.NoError(t, err)
	monitor = newMetricsMonitor(logger, 10, 0, store)
	defer monitor.close()

	restored := monitor.getMetrics()
	require.Len(t, restored, 2)
	require.Equal(t, []int{0, 1}, []int{restored[0].ID, restored[1].ID})
	require.Equal(t, 2, monitor.addMetrics(TokenMetrics{
		Timestamp:       time.Now(),
		Model:           "model-c",
		NewInputTokens:  6,
		OutputTokens:    3,
		TokensPerSecond: 40,
	}))
}
