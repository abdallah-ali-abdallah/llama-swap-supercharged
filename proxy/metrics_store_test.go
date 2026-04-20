package proxy

import (
	"encoding/json"
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
	store, err := newMetricsStoreWithOptions(path, 0, 100, true, true, logger)
	require.NoError(t, err)

	oldMetric := TokenMetrics{ID: 0, Timestamp: time.Now().Add(-48 * time.Hour), Model: "old", NewInputTokens: 1}
	newMetric := TokenMetrics{ID: 1, Timestamp: time.Now(), Model: "new", NewInputTokens: 1}
	require.NoError(t, store.insert(oldMetric))
	require.NoError(t, store.insert(newMetric))
	require.NoError(t, store.insertCapture(0, []byte("old-capture")))
	require.NoError(t, store.insertCapture(1, []byte("new-capture")))
	store.close()

	store, err = newMetricsStoreWithOptions(path, 1, 100, true, true, logger)
	require.NoError(t, err)
	defer store.close()

	metrics, truncated, err := store.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.Equal(t, "new", metrics[0].Model)
	require.True(t, metrics[0].HasCapture)

	_, exists, err := store.getCapture(0)
	require.NoError(t, err)
	require.False(t, exists)
	_, exists, err = store.getCapture(1)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestMetricsStore_CapturePersistence(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, logger)
	require.NoError(t, err)

	capture := ReqRespCapture{
		ID:          7,
		ReqPath:     "/v1/chat/completions",
		ReqHeaders:  map[string]string{"Content-Type": "application/json"},
		ReqBody:     []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
		RespHeaders: map[string]string{"Content-Type": "application/json"},
		RespBody:    []byte(`{"choices":[{"message":{"content":"hi"}}]}`),
	}
	compressed, _, err := compressCapture(&capture)
	require.NoError(t, err)

	require.NoError(t, store.insert(TokenMetrics{ID: 7, Timestamp: time.Now(), Model: "model-a", HasCapture: true}))
	require.NoError(t, store.insertCapture(7, compressed))
	store.close()

	store, err = newMetricsStoreWithOptions(path, 30, 100, true, true, logger)
	require.NoError(t, err)
	defer store.close()

	metrics, truncated, err := store.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].HasCapture)

	persisted, exists, err := store.getCapture(7)
	require.NoError(t, err)
	require.True(t, exists)

	decompressed, err := decompressCapture(persisted)
	require.NoError(t, err)
	var decoded ReqRespCapture
	require.NoError(t, json.Unmarshal(decompressed, &decoded))
	require.Equal(t, capture.ReqPath, decoded.ReqPath)
	require.Equal(t, capture.ReqBody, decoded.ReqBody)
	require.Equal(t, capture.RespBody, decoded.RespBody)
}

func TestMetricsStore_CapturePersistenceDisabled(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, false, logger)
	require.NoError(t, err)
	defer store.close()

	require.NoError(t, store.insert(TokenMetrics{ID: 3, Timestamp: time.Now(), Model: "model-a", HasCapture: true}))
	require.NoError(t, store.insertCapture(3, []byte("capture")))

	metrics, truncated, err := store.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.False(t, metrics[0].HasCapture)

	_, exists, err := store.getCapture(3)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestMetricsStore_ActivityPersistenceDisabled(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, false, true, logger)
	require.NoError(t, err)
	defer store.close()

	require.NoError(t, store.insert(TokenMetrics{ID: 4, Timestamp: time.Now(), Model: "model-a", HasCapture: true}))
	require.NoError(t, store.insertCapture(4, []byte("capture")))

	maxID, err := store.maxID()
	require.NoError(t, err)
	require.Equal(t, -1, maxID)

	metrics, truncated, err := store.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Empty(t, metrics)
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

func TestMetricsMonitor_RestoresPersistedCapture(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, logger)
	require.NoError(t, err)

	monitor := newMetricsMonitor(logger, 10, 5, store)
	metricID := monitor.addMetrics(TokenMetrics{
		Timestamp:  time.Now(),
		Model:      "model-a",
		HasCapture: true,
	})
	monitor.addCapture(ReqRespCapture{
		ID:          metricID,
		ReqPath:     "/v1/chat/completions",
		ReqHeaders:  map[string]string{"Content-Type": "application/json"},
		ReqBody:     []byte(`{"prompt":"hello"}`),
		RespHeaders: map[string]string{"Content-Type": "application/json"},
		RespBody:    []byte(`{"text":"hi"}`),
	})
	monitor.close()

	store, err = newMetricsStoreWithOptions(path, 30, 100, true, true, logger)
	require.NoError(t, err)
	monitor = newMetricsMonitor(logger, 10, 5, store)
	defer monitor.close()

	restored := monitor.getMetrics()
	require.Len(t, restored, 1)
	require.True(t, restored[0].HasCapture)

	compressed, exists := monitor.getCompressedBytes(metricID)
	require.True(t, exists)
	decompressed, err := decompressCapture(compressed)
	require.NoError(t, err)
	var decoded ReqRespCapture
	require.NoError(t, json.Unmarshal(decompressed, &decoded))
	require.Equal(t, "/v1/chat/completions", decoded.ReqPath)
	require.Equal(t, []byte(`{"prompt":"hello"}`), decoded.ReqBody)
}

func TestMetricsMonitor_RangeIncludesMemoryCaptureAvailability(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, false, logger)
	require.NoError(t, err)

	monitor := newMetricsMonitor(logger, 10, 5, store)
	metricID := monitor.addMetrics(TokenMetrics{
		Timestamp:  time.Now(),
		Model:      "model-a",
		HasCapture: true,
	})
	monitor.addCapture(ReqRespCapture{ID: metricID, ReqBody: []byte("live only")})

	metrics, truncated, err := monitor.getMetricsForRange(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.True(t, metrics[0].HasCapture)
	monitor.close()

	store, err = newMetricsStoreWithOptions(path, 30, 100, true, false, logger)
	require.NoError(t, err)
	monitor = newMetricsMonitor(logger, 10, 5, store)
	defer monitor.close()

	metrics, truncated, err = monitor.getMetricsForRange(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.False(t, metrics[0].HasCapture)
}
