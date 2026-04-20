package proxy

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/mostlygeek/llama-swap/proxy/config"
	_ "modernc.org/sqlite"
)

const defaultMetricsQueryMaxRows = 100000

type metricsQuery struct {
	From  *time.Time
	To    *time.Time
	Limit int
	Scope string
}

type activityFieldsSettings struct {
	Model    bool `json:"model"`
	Tokens   bool `json:"tokens"`
	Speeds   bool `json:"speeds"`
	Duration bool `json:"duration"`
}

type persistenceSettings struct {
	SQLiteAvailable            bool                   `json:"sqlite_available"`
	DBPath                     string                 `json:"db_path"`
	RetentionDays              int                    `json:"retention_days"`
	LoggingEnabled             bool                   `json:"logging_enabled"`
	UsageMetricsPersistence    bool                   `json:"usage_metrics_persistence"`
	ActivityPersistence        bool                   `json:"activity_persistence"`
	ActivityCapturePersistence bool                   `json:"activity_capture_persistence"`
	CaptureRedactHeaders       bool                   `json:"capture_redact_headers"`
	ActivityFields             activityFieldsSettings `json:"activity_fields"`
}

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
	logger                     *LogMonitor
}

func newMetricsStore(path string, retentionDays int, defaultQueryRows int, logger *LogMonitor) (*metricsStore, error) {
	return newMetricsStoreWithOptions(path, retentionDays, defaultQueryRows, true, true, false, config.ActivityFieldsConfig{
		Model:    true,
		Tokens:   true,
		Speeds:   true,
		Duration: true,
	}, logger)
}

func newMetricsStoreWithOptions(
	path string,
	retentionDays int,
	defaultQueryRows int,
	usageMetricsPersistence bool,
	activityPersistence bool,
	activityCapturePersistence bool,
	activityFields config.ActivityFieldsConfig,
	logger *LogMonitor,
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
		`CREATE TABLE IF NOT EXISTS token_metrics (
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
		);`,
		`CREATE TABLE IF NOT EXISTS activity_metrics (
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
		"CREATE INDEX IF NOT EXISTS idx_token_metrics_timestamp ON token_metrics(timestamp_ms);",
		"CREATE INDEX IF NOT EXISTS idx_token_metrics_model_timestamp ON token_metrics(model, timestamp_ms);",
		"CREATE INDEX IF NOT EXISTS idx_activity_metrics_timestamp ON activity_metrics(timestamp_ms);",
		"CREATE INDEX IF NOT EXISTS idx_activity_metrics_model_timestamp ON activity_metrics(model, timestamp_ms);",
	}

	for _, command := range commands {
		if _, err := s.db.Exec(command); err != nil {
			return fmt.Errorf("initialize metrics database: %w", err)
		}
	}
	return s.migrateLegacyMetrics()
}

func (s *metricsStore) migrateLegacyMetrics() error {
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO activity_metrics (
			id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms, has_capture
		)
		SELECT id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms, has_capture
		FROM token_metrics`); err != nil {
		return fmt.Errorf("migrate activity metrics: %w", err)
	}

	var legacyCaptureTable string
	if err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'request_captures'").Scan(&legacyCaptureTable); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("find legacy captures table: %w", err)
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO activity_request_captures (id, created_ms, capture_zstd)
		SELECT request_captures.id, request_captures.created_ms, request_captures.capture_zstd
		FROM request_captures
		INNER JOIN activity_metrics ON activity_metrics.id = request_captures.id`); err != nil {
		return fmt.Errorf("migrate activity captures: %w", err)
	}
	return nil
}

func (s *metricsStore) close() {
	if s == nil || s.db == nil {
		return
	}
	if err := s.db.Close(); err != nil && s.logger != nil {
		s.logger.Warnf("failed to close metrics database %s: %v", s.path, err)
	}
}

func (s *metricsStore) insert(metric TokenMetrics) error {
	if s == nil || s.db == nil {
		return nil
	}
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	settings := s.settings()
	if settings.UsageMetricsPersistence {
		if err := s.insertIntoTable("token_metrics", metric); err != nil {
			return err
		}
	}
	if settings.ActivityPersistence {
		if err := s.insertIntoTable("activity_metrics", applyActivityFields(metric, settings.ActivityFields)); err != nil {
			return err
		}
	}
	return nil
}

func (s *metricsStore) insertIntoTable(table string, metric TokenMetrics) error {
	_, err := s.db.Exec(
		fmt.Sprintf(`INSERT OR REPLACE INTO %s (
			id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms, has_capture
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, table),
		metric.ID,
		metric.Timestamp.UnixMilli(),
		metric.Model,
		metric.CachedTokens,
		metric.NewInputTokens,
		metric.OutputTokens,
		metric.PromptPerSecond,
		metric.TokensPerSecond,
		metric.DurationMs,
		boolToInt(metric.HasCapture),
	)
	if err != nil {
		return fmt.Errorf("insert %s: %w", table, err)
	}
	return nil
}

func (s *metricsStore) insertCapture(id int, compressed []byte) error {
	if s == nil || s.db == nil {
		return nil
	}
	settings := s.settings()
	if !settings.ActivityPersistence || !settings.ActivityCapturePersistence {
		return nil
	}
	if len(compressed) == 0 {
		return nil
	}

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO activity_request_captures (
			id, created_ms, capture_zstd
		) VALUES (?, ?, ?)`,
		id,
		time.Now().UnixMilli(),
		compressed,
	)
	if err != nil {
		return fmt.Errorf("insert activity capture: %w", err)
	}
	return nil
}

func (s *metricsStore) getCapture(id int) ([]byte, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, nil
	}

	var capture []byte
	if err := s.db.QueryRow("SELECT capture_zstd FROM activity_request_captures WHERE id = ?", id).Scan(&capture); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read activity capture: %w", err)
	}
	return capture, true, nil
}

func (s *metricsStore) maxID() (int, error) {
	if s == nil || s.db == nil {
		return -1, nil
	}

	var id sql.NullInt64
	if err := s.db.QueryRow("SELECT MAX(id) FROM (SELECT id FROM token_metrics UNION ALL SELECT id FROM activity_metrics)").Scan(&id); err != nil {
		return -1, fmt.Errorf("read max metric id: %w", err)
	}
	if !id.Valid {
		return -1, nil
	}
	return int(id.Int64), nil
}

func (s *metricsStore) latest(limit int) ([]TokenMetrics, error) {
	if s == nil || s.db == nil {
		return []TokenMetrics{}, nil
	}
	if limit <= 0 {
		limit = s.defaultQueryRows
	}

	rows, err := s.db.Query(`SELECT
			token_metrics.id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms,
			EXISTS(SELECT 1 FROM activity_request_captures WHERE activity_request_captures.id = token_metrics.id)
		FROM token_metrics
		ORDER BY token_metrics.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest metrics: %w", err)
	}
	defer rows.Close()

	metrics, err := scanTokenMetrics(rows)
	if err != nil {
		return nil, err
	}
	slices.Reverse(metrics)
	return metrics, nil
}

func (s *metricsStore) query(q metricsQuery) ([]TokenMetrics, bool, error) {
	if s == nil || s.db == nil {
		return []TokenMetrics{}, false, nil
	}

	limit := q.Limit
	if limit <= 0 {
		limit = s.defaultQueryRows
	}

	table := "token_metrics"
	if q.Scope == "activity" {
		table = "activity_metrics"
	}

	query := fmt.Sprintf(`SELECT
			%s.id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms,
			EXISTS(SELECT 1 FROM activity_request_captures WHERE activity_request_captures.id = %s.id)
		FROM %s`, table, table, table)
	args := []any{}

	if q.From != nil || q.To != nil {
		query += " WHERE"
		if q.From != nil {
			query += " timestamp_ms >= ?"
			args = append(args, q.From.UnixMilli())
		}
		if q.To != nil {
			if q.From != nil {
				query += " AND"
			}
			query += " timestamp_ms <= ?"
			args = append(args, q.To.UnixMilli())
		}
	}

	query += " ORDER BY timestamp_ms ASC, id ASC LIMIT ?"
	args = append(args, limit+1)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	metrics, err := scanTokenMetrics(rows)
	if err != nil {
		return nil, false, err
	}

	truncated := len(metrics) > limit
	if truncated {
		metrics = metrics[:limit]
	}

	return metrics, truncated, nil
}

func (s *metricsStore) cleanup() error {
	if s == nil || s.db == nil || s.retentionDays <= 0 {
		return nil
	}

	cutoff := time.Now().Add(-time.Duration(s.retentionDays) * 24 * time.Hour).UnixMilli()
	if _, err := s.db.Exec("DELETE FROM token_metrics WHERE timestamp_ms < ?", cutoff); err != nil {
		return fmt.Errorf("cleanup metrics: %w", err)
	}
	if _, err := s.db.Exec("DELETE FROM activity_metrics WHERE timestamp_ms < ?", cutoff); err != nil {
		return fmt.Errorf("cleanup activity metrics: %w", err)
	}
	if _, err := s.db.Exec("DELETE FROM activity_request_captures WHERE id NOT IN (SELECT id FROM activity_metrics)"); err != nil {
		return fmt.Errorf("cleanup orphaned activity captures: %w", err)
	}
	return nil
}

func (s *metricsStore) settings() persistenceSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return persistenceSettings{
		SQLiteAvailable:            true,
		DBPath:                     s.path,
		RetentionDays:              s.retentionDays,
		LoggingEnabled:             s.loggingEnabled,
		UsageMetricsPersistence:    s.usageMetricsPersistence,
		ActivityPersistence:        s.activityPersistence,
		ActivityCapturePersistence: s.activityCapturePersistence,
		CaptureRedactHeaders:       s.captureRedactHeaders,
		ActivityFields:             s.activityFields,
	}
}

func (s *metricsStore) updateSettings(settings persistenceSettings) error {
	if !settings.ActivityPersistence {
		settings.ActivityCapturePersistence = false
	}

	s.mu.Lock()
	if settings.DBPath == "" {
		settings.DBPath = s.path
	}
	s.selectedPath = settings.DBPath
	s.loggingEnabled = settings.LoggingEnabled
	s.usageMetricsPersistence = settings.UsageMetricsPersistence
	s.activityPersistence = settings.ActivityPersistence
	s.activityCapturePersistence = settings.ActivityCapturePersistence
	s.captureRedactHeaders = settings.CaptureRedactHeaders
	s.activityFields = settings.ActivityFields
	current := persistenceSettings{
		UsageMetricsPersistence:    s.usageMetricsPersistence,
		DBPath:                     s.selectedPath,
		LoggingEnabled:             s.loggingEnabled,
		ActivityPersistence:        s.activityPersistence,
		ActivityCapturePersistence: s.activityCapturePersistence,
		CaptureRedactHeaders:       s.captureRedactHeaders,
		ActivityFields:             s.activityFields,
	}
	s.mu.Unlock()

	return s.saveSettings(current)
}

func (s *metricsStore) loadSettings() error {
	rows, err := s.db.Query("SELECT key, value FROM persistence_settings")
	if err != nil {
		return err
	}
	defer rows.Close()

	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return err
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(values) == 0 {
		return s.saveSettings(s.settings())
	}

	s.mu.Lock()
	if dbPath := values["db_path"]; dbPath != "" {
		s.selectedPath = dbPath
	}
	s.loggingEnabled = parseBoolSetting(values, "logging_enabled", s.loggingEnabled)
	s.usageMetricsPersistence = parseBoolSetting(values, "usage_metrics_persistence", s.usageMetricsPersistence)
	s.activityPersistence = parseBoolSetting(values, "activity_persistence", s.activityPersistence)
	s.activityCapturePersistence = parseBoolSetting(values, "activity_capture_persistence", s.activityCapturePersistence)
	s.captureRedactHeaders = parseBoolSetting(values, "capture_redact_headers", s.captureRedactHeaders)
	s.activityFields.Model = parseBoolSetting(values, "activity_field_model", s.activityFields.Model)
	s.activityFields.Tokens = parseBoolSetting(values, "activity_field_tokens", s.activityFields.Tokens)
	s.activityFields.Speeds = parseBoolSetting(values, "activity_field_speeds", s.activityFields.Speeds)
	s.activityFields.Duration = parseBoolSetting(values, "activity_field_duration", s.activityFields.Duration)
	s.mu.Unlock()

	return nil
}

func (s *metricsStore) preferredPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedPath
}

func (s *metricsStore) saveSettings(settings persistenceSettings) error {
	values := map[string]bool{
		"logging_enabled":              settings.LoggingEnabled,
		"usage_metrics_persistence":    settings.UsageMetricsPersistence,
		"activity_persistence":         settings.ActivityPersistence,
		"activity_capture_persistence": settings.ActivityCapturePersistence,
		"capture_redact_headers":       settings.CaptureRedactHeaders,
		"activity_field_model":         settings.ActivityFields.Model,
		"activity_field_tokens":        settings.ActivityFields.Tokens,
		"activity_field_speeds":        settings.ActivityFields.Speeds,
		"activity_field_duration":      settings.ActivityFields.Duration,
	}

	now := time.Now().UnixMilli()
	dbPath := settings.DBPath
	if dbPath == "" {
		dbPath = s.path
	}
	if _, err := s.db.Exec(
		"INSERT OR REPLACE INTO persistence_settings (key, value, updated_ms) VALUES (?, ?, ?)",
		"db_path",
		dbPath,
		now,
	); err != nil {
		return fmt.Errorf("save persistence setting db_path: %w", err)
	}
	for key, value := range values {
		if _, err := s.db.Exec(
			"INSERT OR REPLACE INTO persistence_settings (key, value, updated_ms) VALUES (?, ?, ?)",
			key,
			boolSetting(value),
			now,
		); err != nil {
			return fmt.Errorf("save persistence setting %s: %w", key, err)
		}
	}
	return nil
}

func parseBoolSetting(values map[string]string, key string, fallback bool) bool {
	switch values[key] {
	case "true":
		return true
	case "false":
		return false
	default:
		return fallback
	}
}

func boolSetting(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func applyActivityFields(metric TokenMetrics, fields activityFieldsSettings) TokenMetrics {
	activity := metric
	if !fields.Model {
		activity.Model = ""
	}
	if !fields.Tokens {
		activity.CachedTokens = 0
		activity.NewInputTokens = 0
		activity.OutputTokens = 0
	}
	if !fields.Speeds {
		activity.PromptPerSecond = -1
		activity.TokensPerSecond = -1
	}
	if !fields.Duration {
		activity.DurationMs = 0
	}
	return activity
}

func activityFieldsConfig(fields activityFieldsSettings) config.ActivityFieldsConfig {
	return config.ActivityFieldsConfig{
		Model:    fields.Model,
		Tokens:   fields.Tokens,
		Speeds:   fields.Speeds,
		Duration: fields.Duration,
	}
}

func scanTokenMetrics(rows *sql.Rows) ([]TokenMetrics, error) {
	metrics := []TokenMetrics{}
	for rows.Next() {
		var metric TokenMetrics
		var timestampMs int64
		var hasCapture int
		if err := rows.Scan(
			&metric.ID,
			&timestampMs,
			&metric.Model,
			&metric.CachedTokens,
			&metric.NewInputTokens,
			&metric.OutputTokens,
			&metric.PromptPerSecond,
			&metric.TokensPerSecond,
			&metric.DurationMs,
			&hasCapture,
		); err != nil {
			return nil, fmt.Errorf("scan metrics: %w", err)
		}
		metric.Timestamp = time.UnixMilli(timestampMs)
		metric.HasCapture = hasCapture != 0
		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metrics: %w", err)
	}
	return metrics, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
