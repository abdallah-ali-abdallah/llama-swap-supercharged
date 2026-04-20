package proxy

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	_ "modernc.org/sqlite"
)

const defaultMetricsQueryMaxRows = 100000

type metricsQuery struct {
	From  *time.Time
	To    *time.Time
	Limit int
}

type metricsStore struct {
	db               *sql.DB
	path             string
	retentionDays    int
	defaultQueryRows int
	logger           *LogMonitor
}

func newMetricsStore(path string, retentionDays int, defaultQueryRows int, logger *LogMonitor) (*metricsStore, error) {
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
		db:               db,
		path:             path,
		retentionDays:    retentionDays,
		defaultQueryRows: defaultQueryRows,
		logger:           logger,
	}

	if err := store.init(); err != nil {
		db.Close()
		return nil, err
	}

	if err := store.cleanup(); err != nil && logger != nil {
		logger.Warnf("failed to clean old metrics from %s: %v", path, err)
	}

	return store, nil
}

func (s *metricsStore) init() error {
	commands := []string{
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
		"CREATE INDEX IF NOT EXISTS idx_token_metrics_timestamp ON token_metrics(timestamp_ms);",
		"CREATE INDEX IF NOT EXISTS idx_token_metrics_model_timestamp ON token_metrics(model, timestamp_ms);",
	}

	for _, command := range commands {
		if _, err := s.db.Exec(command); err != nil {
			return fmt.Errorf("initialize metrics database: %w", err)
		}
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

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO token_metrics (
			id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms, has_capture
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		return fmt.Errorf("insert metrics: %w", err)
	}
	return nil
}

func (s *metricsStore) maxID() (int, error) {
	if s == nil || s.db == nil {
		return -1, nil
	}

	var id sql.NullInt64
	if err := s.db.QueryRow("SELECT MAX(id) FROM token_metrics").Scan(&id); err != nil {
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
			id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms, has_capture
		FROM token_metrics
		ORDER BY id DESC
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

	query := `SELECT
			id, timestamp_ms, model, cache_tokens, new_input_tokens, output_tokens,
			prompt_per_second, tokens_per_second, duration_ms, has_capture
		FROM token_metrics`
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
	return nil
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
		// Request/response captures are stored only in memory, so persisted
		// metrics must not advertise unavailable captures after reload.
		metric.HasCapture = false
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
