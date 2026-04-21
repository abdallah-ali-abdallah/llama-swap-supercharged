package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mostlygeek/llama-swap/proxy/config"
	"github.com/stretchr/testify/require"
)

func TestProxyManager_PersistenceSettingsWritesYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	writeTestConfig(t, configPath, dbPath)
	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)

	pm := New(cfg)
	defer pm.StopProcesses(StopImmediately)

	nextDBPath := filepath.Join(t.TempDir(), "next.db")
	body, err := json.Marshal(persistenceSettings{
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

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/settings/persistence", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	pm.apiUpdatePersistenceSettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	updatedConfig, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, nextDBPath, updatedConfig.MetricsDBPath)
	require.False(t, updatedConfig.LoggingEnabled)
	require.False(t, updatedConfig.UsageMetricsPersistence)
	require.True(t, updatedConfig.ActivityPersistence)
	require.True(t, updatedConfig.ActivityCapturePersistence)
	require.False(t, updatedConfig.CaptureRedactHeaders)
	require.False(t, updatedConfig.ActivityFields.Model)
	require.True(t, updatedConfig.ActivityFields.Tokens)
	require.False(t, updatedConfig.ActivityFields.Speeds)
	require.True(t, updatedConfig.ActivityFields.Duration)
}

func TestProxyManager_PersistenceSettingsDoesNotPartiallyUpdateOnDBSwitchFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	dbPath := filepath.Join(dir, "metrics.db")
	writeTestConfig(t, configPath, dbPath)
	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)

	pm := New(cfg)
	defer pm.StopProcesses(StopImmediately)
	require.True(t, pm.proxyLogger.Enabled())

	blockerPath := filepath.Join(dir, "not-a-directory")
	require.NoError(t, os.WriteFile(blockerPath, []byte("block"), 0o644))
	badDBPath := filepath.Join(blockerPath, "metrics.db")
	body, err := json.Marshal(persistenceSettings{
		DBPath:                     badDBPath,
		LoggingEnabled:             false,
		UsageMetricsPersistence:    false,
		ActivityPersistence:        false,
		ActivityCapturePersistence: true,
		CaptureRedactHeaders:       false,
		ActivityFields: activityFieldsSettings{
			Model:    false,
			Tokens:   false,
			Speeds:   false,
			Duration: false,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/settings/persistence", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	pm.apiUpdatePersistenceSettings(ctx)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	updatedConfig, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, dbPath, updatedConfig.MetricsDBPath)
	require.True(t, updatedConfig.LoggingEnabled)
	require.True(t, updatedConfig.UsageMetricsPersistence)
	require.True(t, updatedConfig.ActivityPersistence)
	require.False(t, updatedConfig.ActivityCapturePersistence)
	require.True(t, updatedConfig.CaptureRedactHeaders)
	require.True(t, updatedConfig.ActivityFields.Model)
	require.True(t, updatedConfig.ActivityFields.Tokens)
	require.True(t, updatedConfig.ActivityFields.Speeds)
	require.True(t, updatedConfig.ActivityFields.Duration)

	require.Equal(t, dbPath, pm.config.MetricsDBPath)
	require.True(t, pm.config.LoggingEnabled)
	require.True(t, pm.config.UsageMetricsPersistence)
	require.True(t, pm.config.ActivityPersistence)
	require.False(t, pm.config.ActivityCapturePersistence)
	require.True(t, pm.config.CaptureRedactHeaders)
	require.True(t, pm.config.ActivityFields.Model)
	require.True(t, pm.config.ActivityFields.Tokens)
	require.True(t, pm.config.ActivityFields.Speeds)
	require.True(t, pm.config.ActivityFields.Duration)
	require.True(t, pm.proxyLogger.Enabled())

	current := pm.metricsMonitor.persistenceSettings()
	require.Equal(t, dbPath, current.DBPath)
	require.True(t, current.LoggingEnabled)
	require.True(t, current.UsageMetricsPersistence)
	require.True(t, current.ActivityPersistence)
	require.False(t, current.ActivityCapturePersistence)
	require.True(t, current.CaptureRedactHeaders)
	require.True(t, current.ActivityFields.Model)
	require.True(t, current.ActivityFields.Tokens)
	require.True(t, current.ActivityFields.Speeds)
	require.True(t, current.ActivityFields.Duration)
}

func TestProxyManager_PersistenceSettingsYAMLOverridesSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	writeTestConfig(t, configPath, dbPath)
	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)

	pm := New(cfg)
	defer pm.StopProcesses(StopImmediately)

	store := pm.metricsMonitor.store
	require.NotNil(t, store)
	sqliteSettings := store.settings()
	sqliteSettings.LoggingEnabled = false
	sqliteSettings.UsageMetricsPersistence = false
	sqliteSettings.ActivityPersistence = false
	sqliteSettings.ActivityCapturePersistence = true
	sqliteSettings.CaptureRedactHeaders = false
	sqliteSettings.ActivityFields = activityFieldsSettings{
		Model:    false,
		Tokens:   false,
		Speeds:   false,
		Duration: false,
	}
	require.NoError(t, store.updateSettings(sqliteSettings))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	pm.apiGetPersistenceSettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response persistenceSettings
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotEmpty(t, response.YAMLConflicts)
	require.True(t, response.LoggingEnabled)
	require.True(t, response.UsageMetricsPersistence)
	require.True(t, response.ActivityPersistence)
	require.False(t, response.ActivityCapturePersistence)
	require.True(t, response.CaptureRedactHeaders)
	require.True(t, response.ActivityFields.Model)
	require.True(t, response.ActivityFields.Tokens)
	require.True(t, response.ActivityFields.Speeds)
	require.True(t, response.ActivityFields.Duration)

	applied := store.settings()
	require.True(t, applied.LoggingEnabled)
	require.True(t, applied.UsageMetricsPersistence)
	require.True(t, applied.ActivityPersistence)
	require.False(t, applied.ActivityCapturePersistence)
	require.True(t, applied.CaptureRedactHeaders)
}

func TestProxyManager_PersistenceSettingsReportsStartupYAMLConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	writeTestConfig(t, configPath, dbPath)

	logger := NewLogMonitorWriter(io.Discard)
	store, err := newMetricsStore(dbPath, 30, 100, logger)
	require.NoError(t, err)
	sqliteSettings := store.settings()
	sqliteSettings.LoggingEnabled = false
	require.NoError(t, store.updateSettings(sqliteSettings))
	store.close()

	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	pm := New(cfg)
	defer pm.StopProcesses(StopImmediately)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	pm.apiGetPersistenceSettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response persistenceSettings
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Contains(t, response.YAMLConflicts, persistenceConflict{
		Field:       "loggingEnabled",
		YAMLValue:   "true",
		SQLiteValue: "false",
	})
	require.True(t, response.LoggingEnabled)
}

func TestProxyManager_PersistenceSettingsIncludesStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	writeTestConfig(t, configPath, dbPath)
	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)

	pm := New(cfg)
	defer pm.StopProcesses(StopImmediately)
	pm.metricsMonitor.addMetrics(TokenMetrics{
		Timestamp:      time.Now(),
		Model:          "model-a",
		NewInputTokens: 10,
		OutputTokens:   5,
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	pm.apiGetPersistenceSettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response persistenceSettings
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotNil(t, response.Stats)
	require.Greater(t, response.Stats.TotalSizeBytes, int64(0))
	require.Equal(t, int64(1), response.Stats.UsageMetricsRows)
	require.Equal(t, int64(1), response.Stats.ActivityRows)
}

func TestProxyManager_PersistenceSettingsNormalizesCaptureWhenActivityDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	dbPath := filepath.Join(t.TempDir(), "metrics.db")
	writeTestConfig(t, configPath, dbPath)
	cfg, err := config.LoadConfig(configPath)
	require.NoError(t, err)

	pm := New(cfg)
	defer pm.StopProcesses(StopImmediately)

	body, err := json.Marshal(persistenceSettings{
		DBPath:                     dbPath,
		LoggingEnabled:             true,
		UsageMetricsPersistence:    true,
		ActivityPersistence:        false,
		ActivityCapturePersistence: true,
		CaptureRedactHeaders:       true,
		ActivityFields: activityFieldsSettings{
			Model:    true,
			Tokens:   true,
			Speeds:   true,
			Duration: true,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/settings/persistence", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	pm.apiUpdatePersistenceSettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response persistenceSettings
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.ActivityPersistence)
	require.False(t, response.ActivityCapturePersistence)

	updatedConfig, err := config.LoadConfig(configPath)
	require.NoError(t, err)
	require.False(t, updatedConfig.ActivityPersistence)
	require.False(t, updatedConfig.ActivityCapturePersistence)
}

func writeTestConfig(t *testing.T, configPath string, dbPath string) {
	t.Helper()
	configContent := []byte(`logLevel: error
loggingEnabled: true
metricsDBPath: ` + dbPath + `
usageMetricsPersistence: true
activityPersistence: true
activityCapturePersistence: false
captureRedactHeaders: true
activityFields:
  model: true
  tokens: true
  speeds: true
  duration: true
models: {}
`)
	require.NoError(t, os.WriteFile(configPath, configContent, 0o644))
}
