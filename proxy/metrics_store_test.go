package proxy

import (
	"database/sql"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mostlygeek/llama-swap/proxy/config"
	"github.com/stretchr/testify/require"
)

func allActivityFields() config.ActivityFieldsConfig {
	return config.ActivityFieldsConfig{
		Model:    true,
		Tokens:   true,
		Speeds:   true,
		Duration: true,
	}
}

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
	store, err := newMetricsStoreWithOptions(path, 0, 100, true, true, true, allActivityFields(), logger)
	require.NoError(t, err)

	oldMetric := TokenMetrics{ID: 0, Timestamp: time.Now().Add(-48 * time.Hour), Model: "old", NewInputTokens: 1}
	newMetric := TokenMetrics{ID: 1, Timestamp: time.Now(), Model: "new", NewInputTokens: 1}
	require.NoError(t, store.insert(oldMetric))
	require.NoError(t, store.insert(newMetric))
	require.NoError(t, store.insertCapture(0, []byte("old-capture")))
	require.NoError(t, store.insertCapture(1, []byte("new-capture")))
	store.close()

	store, err = newMetricsStoreWithOptions(path, 1, 100, true, true, true, allActivityFields(), logger)
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
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, true, allActivityFields(), logger)
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

	store, err = newMetricsStoreWithOptions(path, 30, 100, true, true, true, allActivityFields(), logger)
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
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, false, allActivityFields(), logger)
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
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, false, true, allActivityFields(), logger)
	require.NoError(t, err)
	defer store.close()

	require.NoError(t, store.insert(TokenMetrics{ID: 4, Timestamp: time.Now(), Model: "model-a", HasCapture: true}))
	require.NoError(t, store.insertCapture(4, []byte("capture")))

	maxID, err := store.maxID()
	require.NoError(t, err)
	require.Equal(t, 4, maxID)

	metrics, truncated, err := store.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.False(t, metrics[0].HasCapture)

	activityMetrics, truncated, err := store.query(metricsQuery{Limit: 10, Scope: "activity"})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Empty(t, activityMetrics)

	_, exists, err := store.getCapture(4)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestMetricsStore_PersistenceSettingsRoundTrip(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, false, allActivityFields(), logger)
	require.NoError(t, err)

	require.NoError(t, store.updateSettings(persistenceSettings{
		DBPath:                     path,
		LoggingEnabled:             false,
		UsageMetricsPersistence:    false,
		ActivityPersistence:        true,
		ActivityCapturePersistence: true,
		CaptureRedactHeaders:       false,
		ActivityFields: activityFieldsSettings{
			Model:    false,
			Tokens:   true,
			Speeds:   false,
			Duration: true,
		},
	}))
	store.close()

	store, err = newMetricsStoreWithOptions(path, 30, 100, true, true, false, allActivityFields(), logger)
	require.NoError(t, err)
	defer store.close()

	settings := store.settings()
	require.Equal(t, path, settings.DBPath)
	require.False(t, settings.LoggingEnabled)
	require.False(t, settings.UsageMetricsPersistence)
	require.True(t, settings.ActivityPersistence)
	require.True(t, settings.ActivityCapturePersistence)
	require.False(t, settings.CaptureRedactHeaders)
	require.False(t, settings.ActivityFields.Model)
	require.True(t, settings.ActivityFields.Tokens)
	require.False(t, settings.ActivityFields.Speeds)
	require.True(t, settings.ActivityFields.Duration)

	require.NoError(t, store.updateSettings(persistenceSettings{
		DBPath:                     settings.DBPath,
		LoggingEnabled:             settings.LoggingEnabled,
		UsageMetricsPersistence:    true,
		ActivityPersistence:        false,
		ActivityCapturePersistence: true,
		CaptureRedactHeaders:       settings.CaptureRedactHeaders,
		ActivityFields:             settings.ActivityFields,
	}))
	settings = store.settings()
	require.False(t, settings.ActivityPersistence)
	require.False(t, settings.ActivityCapturePersistence)
}

func TestMetricsStore_SeparatesUsageAndActivityPersistence(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	now := time.Now()

	usageOnlyPath := filepath.Join(t.TempDir(), "usage-only.db")
	usageOnly, err := newMetricsStoreWithOptions(usageOnlyPath, 30, 100, true, false, false, allActivityFields(), logger)
	require.NoError(t, err)
	defer usageOnly.close()
	require.NoError(t, usageOnly.insert(TokenMetrics{ID: 1, Timestamp: now, Model: "model-a", NewInputTokens: 10, OutputTokens: 5}))

	usageMetrics, truncated, err := usageOnly.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, usageMetrics, 1)
	require.Equal(t, "model-a", usageMetrics[0].Model)

	activityMetrics, truncated, err := usageOnly.query(metricsQuery{Limit: 10, Scope: "activity"})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Empty(t, activityMetrics)

	activityOnlyPath := filepath.Join(t.TempDir(), "activity-only.db")
	activityOnly, err := newMetricsStoreWithOptions(activityOnlyPath, 30, 100, false, true, false, allActivityFields(), logger)
	require.NoError(t, err)
	defer activityOnly.close()
	require.NoError(t, activityOnly.insert(TokenMetrics{ID: 2, Timestamp: now, Model: "model-b", NewInputTokens: 20, OutputTokens: 7}))

	usageMetrics, truncated, err = activityOnly.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Empty(t, usageMetrics)

	activityMetrics, truncated, err = activityOnly.query(metricsQuery{Limit: 10, Scope: "activity"})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, activityMetrics, 1)
	require.Equal(t, "model-b", activityMetrics[0].Model)

	maxID, err := activityOnly.maxID()
	require.NoError(t, err)
	require.Equal(t, 2, maxID)
}

func TestMetricsStore_StatsCountsPersistedRows(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, true, allActivityFields(), logger)
	require.NoError(t, err)
	defer store.close()

	base := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	require.NoError(t, store.insert(TokenMetrics{ID: 1, Timestamp: base, Model: "model-a", NewInputTokens: 10, OutputTokens: 5, HasCapture: true}))
	require.NoError(t, store.insert(TokenMetrics{ID: 2, Timestamp: base.Add(time.Minute), Model: "model-b", NewInputTokens: 20, OutputTokens: 10}))
	require.NoError(t, store.insertCapture(1, []byte("compressed-capture")))

	stats, err := store.stats()
	require.NoError(t, err)
	require.Equal(t, int64(2), stats.UsageMetricsRows)
	require.Equal(t, int64(2), stats.ActivityRows)
	require.Equal(t, int64(1), stats.ActivityCaptures)
	require.Equal(t, int64(len("compressed-capture")), stats.CaptureBytes)
	require.Equal(t, base.UnixMilli(), stats.OldestMetricMs)
	require.Equal(t, base.Add(time.Minute).UnixMilli(), stats.NewestMetricMs)
	require.Equal(t, base.UnixMilli(), stats.OldestActivityMs)
	require.Equal(t, base.Add(time.Minute).UnixMilli(), stats.NewestActivityMs)
	require.GreaterOrEqual(t, stats.SettingsRows, int64(1))
}

func TestMetricsStore_StatsReportsDatabaseFootprint(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStore(path, 30, 100, logger)
	require.NoError(t, err)
	defer store.close()

	require.NoError(t, store.insert(TokenMetrics{ID: 1, Timestamp: time.Now(), Model: "model-a", NewInputTokens: 10}))

	stats, err := store.stats()
	require.NoError(t, err)
	require.Greater(t, stats.DBSizeBytes, int64(0))
	require.GreaterOrEqual(t, stats.WALSizeBytes, int64(0))
	require.GreaterOrEqual(t, stats.SHMSizeBytes, int64(0))
	require.Equal(t, stats.DBSizeBytes+stats.WALSizeBytes+stats.SHMSizeBytes, stats.TotalSizeBytes)
}

func TestMetricsMonitor_SwitchesPersistenceStore(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.db")
	secondPath := filepath.Join(dir, "second.db")
	store, err := newMetricsStoreWithOptions(firstPath, 30, 100, true, true, false, allActivityFields(), logger)
	require.NoError(t, err)
	monitor := newMetricsMonitor(logger, 10, 0, store)
	defer monitor.close()

	monitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "first", NewInputTokens: 1})

	updated, err := monitor.updatePersistenceSettings(persistenceSettings{
		DBPath:                     secondPath,
		LoggingEnabled:             true,
		UsageMetricsPersistence:    true,
		ActivityPersistence:        true,
		ActivityCapturePersistence: false,
		CaptureRedactHeaders:       true,
		ActivityFields: activityFieldsSettings{
			Model:    true,
			Tokens:   true,
			Speeds:   true,
			Duration: true,
		},
	})
	require.NoError(t, err)
	require.Equal(t, secondPath, updated.DBPath)
	monitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "second", NewInputTokens: 2})

	firstStore, err := newMetricsStoreWithOptions(firstPath, 30, 100, true, true, false, allActivityFields(), logger)
	require.NoError(t, err)
	firstMetrics, truncated, err := firstStore.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, firstMetrics, 1)
	require.Equal(t, "first", firstMetrics[0].Model)
	require.Equal(t, secondPath, firstStore.preferredPath())
	firstStore.close()

	secondStore, err := newMetricsStoreWithOptions(secondPath, 30, 100, true, true, false, allActivityFields(), logger)
	require.NoError(t, err)
	defer secondStore.close()
	secondMetrics, truncated, err := secondStore.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, secondMetrics, 1)
	require.Equal(t, "second", secondMetrics[0].Model)
}

func TestMetricsStore_AppliesActivityFields(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, false, config.ActivityFieldsConfig{
		Model:    false,
		Tokens:   false,
		Speeds:   true,
		Duration: false,
	}, logger)
	require.NoError(t, err)
	defer store.close()

	metric := TokenMetrics{
		ID:              10,
		Timestamp:       time.Now(),
		Model:           "private-model",
		CachedTokens:    11,
		NewInputTokens:  22,
		OutputTokens:    33,
		PromptPerSecond: 44.5,
		TokensPerSecond: 55.5,
		DurationMs:      660,
	}
	require.NoError(t, store.insert(metric))

	usageMetrics, truncated, err := store.query(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, usageMetrics, 1)
	require.Equal(t, "private-model", usageMetrics[0].Model)
	require.Equal(t, 11, usageMetrics[0].CachedTokens)
	require.Equal(t, 22, usageMetrics[0].NewInputTokens)
	require.Equal(t, 33, usageMetrics[0].OutputTokens)
	require.Equal(t, 660, usageMetrics[0].DurationMs)

	activityMetrics, truncated, err := store.query(metricsQuery{Limit: 10, Scope: "activity"})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, activityMetrics, 1)
	require.Empty(t, activityMetrics[0].Model)
	require.Zero(t, activityMetrics[0].CachedTokens)
	require.Zero(t, activityMetrics[0].NewInputTokens)
	require.Zero(t, activityMetrics[0].OutputTokens)
	require.Equal(t, 44.5, activityMetrics[0].PromptPerSecond)
	require.Equal(t, 55.5, activityMetrics[0].TokensPerSecond)
	require.Zero(t, activityMetrics[0].DurationMs)
}

func TestMetricsStore_MigratesLegacyActivityRows(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	path := filepath.Join(t.TempDir(), "metrics.db")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE token_metrics (
		id INTEGER PRIMARY KEY,
		timestamp_ms INTEGER NOT NULL,
		model TEXT NOT NULL,
		cache_tokens INTEGER NOT NULL,
		new_input_tokens INTEGER NOT NULL,
		output_tokens INTEGER NOT NULL,
		prompt_per_second REAL NOT NULL,
		tokens_per_second REAL NOT NULL,
		duration_ms INTEGER NOT NULL,
		has_capture INTEGER NOT NULL
	);
	CREATE TABLE request_captures (
		id INTEGER PRIMARY KEY,
		created_ms INTEGER NOT NULL,
		capture_zstd BLOB NOT NULL
	);`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO token_metrics (
		id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
		prompt_per_second, tokens_per_second, duration_ms, has_capture
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, 21, time.Now().UnixMilli(), "legacy-model", 1, 2, 3, 4.5, 6.7, 890, 1)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO request_captures (id, created_ms, capture_zstd) VALUES (?, ?, ?)", 21, time.Now().UnixMilli(), []byte("legacy-capture"))
	require.NoError(t, err)
	require.NoError(t, db.Close())

	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, true, allActivityFields(), logger)
	require.NoError(t, err)
	defer store.close()

	activityMetrics, truncated, err := store.query(metricsQuery{Limit: 10, Scope: "activity"})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, activityMetrics, 1)
	require.Equal(t, "legacy-model", activityMetrics[0].Model)
	require.True(t, activityMetrics[0].HasCapture)

	capture, exists, err := store.getCapture(21)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, []byte("legacy-capture"), capture)
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
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, true, allActivityFields(), logger)
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

	store, err = newMetricsStoreWithOptions(path, 30, 100, true, true, true, allActivityFields(), logger)
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
	store, err := newMetricsStoreWithOptions(path, 30, 100, true, true, false, allActivityFields(), logger)
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

	store, err = newMetricsStoreWithOptions(path, 30, 100, true, true, false, allActivityFields(), logger)
	require.NoError(t, err)
	monitor = newMetricsMonitor(logger, 10, 5, store)
	defer monitor.close()

	metrics, truncated, err = monitor.getMetricsForRange(metricsQuery{Limit: 10})
	require.NoError(t, err)
	require.False(t, truncated)
	require.Len(t, metrics, 1)
	require.False(t, metrics[0].HasCapture)
}
