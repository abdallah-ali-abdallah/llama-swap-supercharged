package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mostlygeek/llama-swap/internal/config"
	"github.com/mostlygeek/llama-swap/internal/logmon"
	_ "modernc.org/sqlite"
)

const defaultMetricsQueryMaxRows = 100000

type activityFieldsSettings struct {
	Model    bool `json:"model"`
	Tokens   bool `json:"tokens"`
	Speeds   bool `json:"speeds"`
	Duration bool `json:"duration"`
}

type persistenceSettings struct {
	SQLiteAvailable            bool                   `json:"sqlite_available"`
	YAMLAvailable              bool                   `json:"yaml_available"`
	YAMLPath                   string                 `json:"yaml_path"`
	DBPath                     string                 `json:"db_path"`
	RetentionDays              int                    `json:"retention_days"`
	LoggingEnabled             bool                   `json:"logging_enabled"`
	UsageMetricsPersistence    bool                   `json:"usage_metrics_persistence"`
	ActivityPersistence        bool                   `json:"activity_persistence"`
	ActivityCapturePersistence bool                   `json:"activity_capture_persistence"`
	CaptureRedactHeaders       bool                   `json:"capture_redact_headers"`
	ActivityFields             activityFieldsSettings `json:"activity_fields"`
	Stats                      *persistenceStats      `json:"stats,omitempty"`
}

type persistenceStats struct {
	DBSizeBytes      int64 `json:"db_size_bytes"`
	WALSizeBytes     int64 `json:"wal_size_bytes"`
	SHMSizeBytes     int64 `json:"shm_size_bytes"`
	TotalSizeBytes   int64 `json:"total_size_bytes"`
	UsageMetricsRows int64 `json:"usage_metrics_rows"`
	ActivityRows     int64 `json:"activity_rows"`
	ActivityCaptures int64 `json:"activity_captures"`
	CaptureBytes     int64 `json:"capture_bytes"`
	SettingsRows     int64 `json:"settings_rows"`
	OldestMetricMs   int64 `json:"oldest_metric_ms,omitempty"`
	NewestMetricMs   int64 `json:"newest_metric_ms,omitempty"`
	OldestActivityMs int64 `json:"oldest_activity_ms,omitempty"`
	NewestActivityMs int64 `json:"newest_activity_ms,omitempty"`
}

// metricsStore persists metrics and captures to SQLite.
type metricsStore struct {
	mu                         sync.RWMutex
	db                         *sql.DB
	path                       string
	selectedPath               string
	retentionDays              int
	defaultQueryRows           int
	loggingEnabled             bool
	usageMetricsPersistence    bool
	activityPersistence        bool
	activityCapturePersistence bool
	captureRedactHeaders       bool
	activityFields             activityFieldsSettings
	logger                     *logmon.Monitor
}

func newMetricsStoreWithOptions(
	path string,
	retentionDays int,
	defaultQueryRows int,
	usageMetricsPersistence bool,
	activityPersistence bool,
	activityCapturePersistence bool,
	activityFields config.ActivityFieldsConfig,
	logger *logmon.Monitor,
) (*metricsStore, error) {
	if path == "" {
		return nil, errors.New("metrics database path is empty")
	}
	if defaultQueryRows <= 0 {
		defaultQueryRows = defaultMetricsQueryMaxRows
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create metrics database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open metrics database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &metricsStore{
		db:                         db,
		path:                       path,
		selectedPath:               path,
		retentionDays:              retentionDays,
		defaultQueryRows:           defaultQueryRows,
		loggingEnabled:             true,
		usageMetricsPersistence:    usageMetricsPersistence,
		activityPersistence:        activityPersistence,
		activityCapturePersistence: activityCapturePersistence,
		captureRedactHeaders:       true,
		activityFields: activityFieldsSettings{
			Model:    activityFields.Model,
			Tokens:   activityFields.Tokens,
			Speeds:   activityFields.Speeds,
			Duration: activityFields.Duration,
		},
		logger: logger,
	}
	if err := store.init(); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.loadSettings(); err != nil && logger != nil {
		logger.Warnf("failed to load metrics persistence settings from %s: %v", path, err)
	}
	if err := store.cleanup(); err != nil && logger != nil {
		logger.Warnf("failed to clean old metrics from %s: %v", path, err)
	}
	return store, nil
}

func (s *metricsStore) init() error {
	commands := []string{
		"PRAGMA foreign_keys=ON;",
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		`CREATE TABLE IF NOT EXISTS activity_metrics (
			id INTEGER PRIMARY KEY,
			timestamp_ms INTEGER NOT NULL,
			model TEXT NOT NULL,
			req_path TEXT NOT NULL DEFAULT '',
			resp_content_type TEXT NOT NULL DEFAULT '',
			resp_status_code INTEGER NOT NULL DEFAULT 200,
			tokens_json TEXT NOT NULL DEFAULT '{}',
			duration_ms INTEGER NOT NULL,
			prompt_ms INTEGER NOT NULL DEFAULT 0,
			predicted_ms INTEGER NOT NULL DEFAULT 0,
			has_capture INTEGER NOT NULL,
			multimodal INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS activity_request_captures (
			id INTEGER PRIMARY KEY,
			created_ms INTEGER NOT NULL,
			capture_zstd BLOB NOT NULL,
			FOREIGN KEY(id) REFERENCES activity_metrics(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS persistence_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_ms INTEGER NOT NULL
		);`,
		"CREATE INDEX IF NOT EXISTS idx_activity_metrics_timestamp ON activity_metrics(timestamp_ms);",
		"CREATE INDEX IF NOT EXISTS idx_activity_metrics_model_timestamp ON activity_metrics(model, timestamp_ms);",
	}
	for _, cmd := range commands {
		if _, err := s.db.Exec(cmd); err != nil {
			return fmt.Errorf("initialize metrics database: %w", err)
		}
	}
	return nil
}

func (s *metricsStore) persistMetric(entry ActivityLogEntry) error {
	if s == nil || s.db == nil || !s.activityPersistence {
		return nil
	}
	tokensJSON, err := json.Marshal(entry.Tokens)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO activity_metrics (
		id, timestamp_ms, model, req_path, resp_content_type, resp_status_code,
		tokens_json, duration_ms, prompt_ms, predicted_ms, has_capture, multimodal
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		unixMs(entry.Timestamp),
		entry.Model,
		entry.ReqPath,
		entry.RespContentType,
		entry.RespStatusCode,
		tokensJSON,
		entry.DurationMs,
		0, // prompt_ms not available in ActivityLogEntry
		0, // predicted_ms not available
		boolInt(entry.HasCapture),
		0, // multimodal requires extra detection
	)
	return err
}

func (s *metricsStore) persistCapture(id int, capture ReqRespCapture) error {
	if s == nil || s.db == nil || !s.activityCapturePersistence {
		return nil
	}
	data, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO activity_request_captures (id, created_ms, capture_zstd) VALUES (?, ?, ?)`,
		id, time.Now().UnixMilli(), data)
	return err
}

func (s *metricsStore) cleanup() error {
	if s == nil || s.db == nil || s.retentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -s.retentionDays).UnixMilli()
	_, err := s.db.Exec(`DELETE FROM activity_metrics WHERE timestamp_ms < ?`, cutoff)
	return err
}

func (s *metricsStore) loadSettings() error {
	// minimal stub; full persistence settings sync can be added later
	return nil
}

func (s *metricsStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func unixMs(t time.Time) int64 {
	return t.UnixMilli()
}

func (s *metricsStore) getSettings() persistenceSettings {
	if s == nil || s.db == nil {
		return persistenceSettings{}
	}
	ps := persistenceSettings{
		DBPath:                     s.path,
		RetentionDays:              s.retentionDays,
		LoggingEnabled:             s.loggingEnabled,
		UsageMetricsPersistence:    s.usageMetricsPersistence,
		ActivityPersistence:        s.activityPersistence,
		ActivityCapturePersistence: s.activityCapturePersistence,
		CaptureRedactHeaders:       s.captureRedactHeaders,
		ActivityFields:             s.activityFields,
		SQLiteAvailable:            true,
	}

	rows, err := s.db.Query(`SELECT key, value FROM persistence_settings`)
	if err != nil {
		return ps
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch key {
		case "logging_enabled":
			ps.LoggingEnabled = value == "true"
		case "usage_metrics_persistence":
			ps.UsageMetricsPersistence = value == "true"
		case "activity_persistence":
			ps.ActivityPersistence = value == "true"
		case "activity_capture_persistence":
			ps.ActivityCapturePersistence = value == "true"
		case "capture_redact_headers":
			ps.CaptureRedactHeaders = value == "true"
		}
	}
	return ps
}

func (s *metricsStore) updateSettings(ps persistenceSettings) error {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loggingEnabled = ps.LoggingEnabled
	s.usageMetricsPersistence = ps.UsageMetricsPersistence
	s.activityPersistence = ps.ActivityPersistence
	s.activityCapturePersistence = ps.ActivityCapturePersistence
	s.captureRedactHeaders = ps.CaptureRedactHeaders
	s.activityFields = ps.ActivityFields

	pairs := map[string]string{
		"logging_enabled":              fmt.Sprintf("%t", ps.LoggingEnabled),
		"usage_metrics_persistence":    fmt.Sprintf("%t", ps.UsageMetricsPersistence),
		"activity_persistence":         fmt.Sprintf("%t", ps.ActivityPersistence),
		"activity_capture_persistence": fmt.Sprintf("%t", ps.ActivityCapturePersistence),
		"capture_redact_headers":       fmt.Sprintf("%t", ps.CaptureRedactHeaders),
	}
	for key, value := range pairs {
		if _, err := s.db.Exec(`INSERT INTO persistence_settings (key, value, updated_ms) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_ms=excluded.updated_ms`, key, value, time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// openMetricsStore creates a metricsStore from config settings.
func openMetricsStore(cfg config.Config, logger *logmon.Monitor) *metricsStore {
	dbPath := strings.TrimSpace(cfg.MetricsDBPath)
	if dbPath == "" {
		if cfg.ConfigPath == "" {
			return nil
		}
		dbPath = filepath.Join(filepath.Dir(cfg.ConfigPath), "llama-swap-metrics.db")
	} else {
		if !filepath.IsAbs(dbPath) {
			baseDir := "."
			if cfg.ConfigPath != "" {
				baseDir = filepath.Dir(cfg.ConfigPath)
			}
			dbPath = filepath.Clean(filepath.Join(baseDir, dbPath))
		}
	}
	store, err := newMetricsStoreWithOptions(
		dbPath,
		cfg.MetricsRetentionDays,
		cfg.MetricsQueryMaxRows,
		cfg.UsageMetricsPersistence,
		cfg.ActivityPersistence,
		cfg.ActivityCapturePersistence,
		cfg.ActivityFields,
		logger,
	)
	if err != nil {
		if logger != nil {
			logger.Warnf("metrics persistence disabled: %v", err)
		}
		return nil
	}
	if logger != nil {
		logger.Infof("metrics persistence enabled: %s", dbPath)
	}
	return store
}
