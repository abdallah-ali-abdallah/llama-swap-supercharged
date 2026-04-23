package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mostlygeek/llama-swap/proxy/config"
	"github.com/stretchr/testify/require"
)

func TestProxyManager_ParseMetricsRangeQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pm := &ProxyManager{
		config: config.Config{
			MetricsQueryMaxRows: 250,
		},
	}

	tests := []struct {
		name           string
		target         string
		wantRange      string
		wantDuration   time.Duration
		wantFrom       bool
		wantTo         bool
		wantLimit      int
		wantScope      string
		wantErr        string
		wantExactFrom  time.Time
		wantExactTo    time.Time
		wantFromToSame bool
	}{
		{
			name:         "past 5 minutes",
			target:       "/api/metrics?range=5m",
			wantRange:    "5m",
			wantDuration: 5 * time.Minute,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past 10 minutes",
			target:       "/api/metrics?range=10m",
			wantRange:    "10m",
			wantDuration: 10 * time.Minute,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past 1 hour",
			target:       "/api/metrics?range=1h",
			wantRange:    "1h",
			wantDuration: time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past 8 hours",
			target:       "/api/metrics?range=8h",
			wantRange:    "8h",
			wantDuration: 8 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past day",
			target:       "/api/metrics?range=1d",
			wantRange:    "1d",
			wantDuration: 24 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past week",
			target:       "/api/metrics?range=1w",
			wantRange:    "1w",
			wantDuration: 7 * 24 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:         "past month",
			target:       "/api/metrics?range=1mo",
			wantRange:    "1mo",
			wantDuration: 30 * 24 * time.Hour,
			wantFrom:     true,
			wantLimit:    250,
		},
		{
			name:      "all",
			target:    "/api/metrics?range=all",
			wantRange: "all",
			wantLimit: 250,
		},
		{
			name:          "custom with RFC3339 from and to",
			target:        "/api/metrics?range=custom&from=2026-04-20T10:00:00Z&to=2026-04-20T11:00:00Z",
			wantRange:     "custom",
			wantFrom:      true,
			wantTo:        true,
			wantLimit:     250,
			wantExactFrom: time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC),
			wantExactTo:   time.Date(2026, 4, 20, 11, 0, 0, 0, time.UTC),
		},
		{
			name:          "custom with unix millisecond from",
			target:        "/api/metrics?range=custom&from=1776682800123",
			wantRange:     "custom",
			wantFrom:      true,
			wantLimit:     250,
			wantExactFrom: time.UnixMilli(1776682800123),
		},
		{
			name:      "limit clamps to configured max",
			target:    "/api/metrics?range=all&limit=999",
			wantRange: "all",
			wantLimit: 250,
		},
		{
			name:      "limit accepts smaller positive value",
			target:    "/api/metrics?range=all&limit=25",
			wantRange: "all",
			wantLimit: 25,
		},
		{
			name:      "activity scope",
			target:    "/api/metrics?range=all&scope=activity",
			wantRange: "all",
			wantLimit: 250,
			wantScope: "activity",
		},
		{
			name:    "unsupported scope",
			target:  "/api/metrics?range=all&scope=unknown",
			wantErr: `unsupported metrics scope "unknown"`,
		},
		{
			name:    "custom requires a bound",
			target:  "/api/metrics?range=custom",
			wantErr: "custom range requires from or to",
		},
		{
			name:    "custom rejects reversed bounds",
			target:  "/api/metrics?range=custom&from=2026-04-20T11:00:00Z&to=2026-04-20T10:00:00Z",
			wantErr: "from must be before to",
		},
		{
			name:    "unsupported range",
			target:  "/api/metrics?range=2h",
			wantErr: `unsupported metrics range "2h"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", tt.target, nil)

			query, normalizedRange, err := pm.parseMetricsRangeQuery(c)
			after := time.Now()

			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantRange, normalizedRange)
			require.Equal(t, tt.wantLimit, query.Limit)
			require.Equal(t, tt.wantScope, query.Scope)

			if tt.wantFrom {
				require.NotNil(t, query.From)
			} else {
				require.Nil(t, query.From)
			}
			if tt.wantTo {
				require.NotNil(t, query.To)
			} else {
				require.Nil(t, query.To)
			}

			if tt.wantDuration > 0 {
				require.False(t, query.From.Before(before.Add(-tt.wantDuration)), "from should not be older than range duration")
				require.False(t, query.From.After(after.Add(-tt.wantDuration)), "from should not be newer than range duration")
			}
			if !tt.wantExactFrom.IsZero() {
				require.True(t, tt.wantExactFrom.Equal(*query.From), "unexpected exact from")
			}
			if !tt.wantExactTo.IsZero() {
				require.True(t, tt.wantExactTo.Equal(*query.To), "unexpected exact to")
			}
		})
	}
}

func TestProxyManager_PersistenceSettingsAPI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := NewLogMonitorWriter(io.Discard)
	store, err := newMetricsStoreWithOptions(filepath.Join(t.TempDir(), "metrics.db"), 14, 100, true, true, false, allActivityFields(), logger)
	require.NoError(t, err)
	monitor := newMetricsMonitor(logger, 10, 0, nil, store)
	defer monitor.close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("models: {}\n"), 0o644))
	pm := &ProxyManager{
		config: config.Config{
			ConfigPath:                 configPath,
			MetricsRetentionDays:       14,
			MetricsQueryMaxRows:        100,
			LoggingEnabled:             true,
			UsageMetricsPersistence:    true,
			ActivityPersistence:        true,
			ActivityCapturePersistence: false,
			CaptureRedactHeaders:       true,
			ActivityFields:             allActivityFields(),
		},
		metricsMonitor: monitor,
		proxyLogger:    logger,
		upstreamLogger: logger,
		muxLogger:      logger,
	}

	getRecorder := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRecorder)
	pm.apiGetPersistenceSettings(getCtx)

	require.Equal(t, http.StatusOK, getRecorder.Code)
	var current persistenceSettings
	require.NoError(t, json.Unmarshal(getRecorder.Body.Bytes(), &current))
	require.True(t, current.SQLiteAvailable)
	require.True(t, current.UsageMetricsPersistence)
	require.True(t, current.ActivityPersistence)
	require.False(t, current.ActivityCapturePersistence)
	require.True(t, current.LoggingEnabled)
	require.True(t, current.CaptureRedactHeaders)
	require.Equal(t, 14, current.RetentionDays)

	nextDBPath := filepath.Join(t.TempDir(), "next.db")
	updateBody, err := json.Marshal(persistenceSettings{
		DBPath:                     nextDBPath,
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
	})
	require.NoError(t, err)
	updateRecorder := httptest.NewRecorder()
	updateCtx, _ := gin.CreateTestContext(updateRecorder)
	updateCtx.Request = httptest.NewRequest(http.MethodPut, "/api/settings/persistence", bytes.NewReader(updateBody))
	updateCtx.Request.Header.Set("Content-Type", "application/json")

	pm.apiUpdatePersistenceSettings(updateCtx)

	require.Equal(t, http.StatusOK, updateRecorder.Code)
	var updated persistenceSettings
	require.NoError(t, json.Unmarshal(updateRecorder.Body.Bytes(), &updated))
	require.Equal(t, nextDBPath, updated.DBPath)
	require.False(t, updated.LoggingEnabled)
	require.False(t, updated.UsageMetricsPersistence)
	require.True(t, updated.ActivityPersistence)
	require.True(t, updated.ActivityCapturePersistence)
	require.False(t, updated.CaptureRedactHeaders)
	require.False(t, updated.ActivityFields.Model)
	require.True(t, updated.ActivityFields.Tokens)
	require.False(t, updated.ActivityFields.Speeds)
	require.True(t, updated.ActivityFields.Duration)
	require.False(t, logger.Enabled())
}

func TestProxyManager_ExcludeFromMetricsRealtime(t *testing.T) {
	pm := newExcludeMetricsAPIProxyManager(t, nil)
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "visible", NewInputTokens: 1})
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "hidden", NewInputTokens: 2})

	metrics := getMetricsAPIResponse(t, pm, "/api/metrics")

	require.Equal(t, []string{"visible"}, metricModels(metrics))
}

func TestProxyManager_ExcludeFromMetricsRangeUsage(t *testing.T) {
	pm := newExcludeMetricsAPIProxyManagerWithStore(t)
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "visible", NewInputTokens: 1})
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "hidden", NewInputTokens: 2})

	metrics := getMetricsAPIResponse(t, pm, "/api/metrics?range=all")

	require.Equal(t, []string{"visible"}, metricModels(metrics))
}

func TestProxyManager_ExcludeFromMetricsRangeActivity(t *testing.T) {
	pm := newExcludeMetricsAPIProxyManagerWithStore(t)
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "visible", NewInputTokens: 1})
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "hidden", NewInputTokens: 2})

	metrics := getMetricsAPIResponse(t, pm, "/api/metrics?range=all&scope=activity")

	require.Equal(t, []string{"visible"}, metricModels(metrics))
}

func TestProxyManager_ExcludeFromMetricsSSE(t *testing.T) {
	pm := newExcludeMetricsAPIProxyManager(t, nil)
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "visible", NewInputTokens: 1})
	pm.metricsMonitor.addMetrics(TokenMetrics{Timestamp: time.Now(), Model: "hidden", NewInputTokens: 2})

	reqCtx, cancel := context.WithCancel(context.Background())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(reqCtx)

	done := make(chan struct{})
	go func() {
		pm.apiSendEvents(c)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE stream to stop")
	}

	metrics := metricsFromSSEBody(t, w.Body.String())
	require.Equal(t, []string{"visible"}, metricModels(metrics))
}

func newExcludeMetricsAPIProxyManagerWithStore(t *testing.T) *ProxyManager {
	t.Helper()
	logger := NewLogMonitorWriter(io.Discard)
	store, err := newMetricsStore(filepath.Join(t.TempDir(), "metrics.db"), 30, 100, logger)
	require.NoError(t, err)
	t.Cleanup(store.close)
	return newExcludeMetricsAPIProxyManager(t, newMetricsMonitor(logger, 100, 0, nil, store))
}

func newExcludeMetricsAPIProxyManager(t *testing.T, monitor *metricsMonitor) *ProxyManager {
	t.Helper()
	logger := NewLogMonitorWriter(io.Discard)
	if monitor == nil {
		monitor = newMetricsMonitor(logger, 100, 0, nil)
	}
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	t.Cleanup(shutdownCancel)
	return &ProxyManager{
		config: config.Config{
			MetricsQueryMaxRows: 100,
			Models: map[string]config.ModelConfig{
				"visible": {Cmd: "visible"},
				"hidden":  {Cmd: "hidden", ExcludeFromMetrics: true},
			},
		},
		metricsMonitor:  monitor,
		proxyLogger:     logger,
		upstreamLogger:  logger,
		shutdownCtx:     shutdownCtx,
		shutdownCancel:  shutdownCancel,
		inFlightCounter: newInflightCounter(),
	}
}

func getMetricsAPIResponse(t *testing.T, pm *ProxyManager, target string) []TokenMetrics {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)

	pm.apiGetMetrics(c)

	require.Equal(t, http.StatusOK, w.Code)
	var metrics []TokenMetrics
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &metrics))
	return metrics
}

func metricsFromSSEBody(t *testing.T, body string) []TokenMetrics {
	t.Helper()
	metrics := []TokenMetrics{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var envelope messageEnvelope
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &envelope); err != nil {
			continue
		}
		if envelope.Type != msgTypeMetrics {
			continue
		}
		var eventMetrics []TokenMetrics
		require.NoError(t, json.Unmarshal([]byte(envelope.Data), &eventMetrics))
		metrics = append(metrics, eventMetrics...)
	}
	return metrics
}

func metricModels(metrics []TokenMetrics) []string {
	models := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		models = append(models, metric.Model)
	}
	return models
}
