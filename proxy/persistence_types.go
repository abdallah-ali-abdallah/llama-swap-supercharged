package proxy

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
	YAMLConflicts              []persistenceConflict  `json:"yaml_conflicts,omitempty"`
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

type persistenceConflict struct {
	Field       string `json:"field"`
	YAMLValue   string `json:"yaml_value"`
	SQLiteValue string `json:"sqlite_value"`
}
