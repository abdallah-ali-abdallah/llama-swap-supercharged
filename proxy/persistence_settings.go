package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func (pm *ProxyManager) persistenceSettingsWithYAMLPriority() persistenceSettings {
	current := pm.metricsMonitor.persistenceSettings()
	yamlSettings, ok := pm.persistenceSettingsFromConfig(current)
	if !ok {
		return current
	}

	conflicts := persistenceConflicts(current, yamlSettings)
	if len(conflicts) > 0 && current.SQLiteAvailable {
		if updated, err := pm.metricsMonitor.updatePersistenceSettings(yamlSettings); err == nil {
			current = updated
			pm.applyLoggingEnabled(current.LoggingEnabled)
			pm.metricsMonitor.setYAMLConflicts(conflicts)
		}
	} else if len(conflicts) > 0 {
		current = yamlSettings
	}

	current.YAMLAvailable = true
	current.YAMLPath = pm.config.ConfigPath
	current.YAMLConflicts = mergePersistenceConflicts(pm.metricsMonitor.yamlConflicts(), conflicts)
	return current
}

func (pm *ProxyManager) persistenceSettingsFromConfig(base persistenceSettings) (persistenceSettings, bool) {
	if strings.TrimSpace(pm.config.ConfigPath) == "" {
		return persistenceSettings{}, false
	}
	if _, err := os.Stat(pm.config.ConfigPath); err != nil {
		return persistenceSettings{}, false
	}

	dbPath, ok := defaultMetricsDBPath(pm.config)
	if !ok {
		dbPath = base.DBPath
	}

	return normalizePersistenceSettings(persistenceSettings{
		SQLiteAvailable:            base.SQLiteAvailable,
		YAMLAvailable:              true,
		YAMLPath:                   pm.config.ConfigPath,
		DBPath:                     dbPath,
		RetentionDays:              pm.config.MetricsRetentionDays,
		LoggingEnabled:             pm.config.LoggingEnabled,
		UsageMetricsPersistence:    pm.config.UsageMetricsPersistence,
		ActivityPersistence:        pm.config.ActivityPersistence,
		ActivityCapturePersistence: pm.config.ActivityCapturePersistence,
		CaptureRedactHeaders:       pm.config.CaptureRedactHeaders,
		ActivityFields: activityFieldsSettings{
			Model:    pm.config.ActivityFields.Model,
			Tokens:   pm.config.ActivityFields.Tokens,
			Speeds:   pm.config.ActivityFields.Speeds,
			Duration: pm.config.ActivityFields.Duration,
		},
	}), true
}

func normalizePersistenceSettings(settings persistenceSettings) persistenceSettings {
	if !settings.ActivityPersistence {
		settings.ActivityCapturePersistence = false
	}
	return settings
}

func persistenceConflicts(sqliteSettings, yamlSettings persistenceSettings) []persistenceConflict {
	conflicts := []persistenceConflict{}
	addString := func(field, sqliteValue, yamlValue string) {
		if sqliteValue != yamlValue {
			conflicts = append(conflicts, persistenceConflict{
				Field:       field,
				SQLiteValue: sqliteValue,
				YAMLValue:   yamlValue,
			})
		}
	}
	addBool := func(field string, sqliteValue, yamlValue bool) {
		if sqliteValue != yamlValue {
			conflicts = append(conflicts, persistenceConflict{
				Field:       field,
				SQLiteValue: strconv.FormatBool(sqliteValue),
				YAMLValue:   strconv.FormatBool(yamlValue),
			})
		}
	}

	addString("metricsDBPath", filepath.Clean(sqliteSettings.DBPath), filepath.Clean(yamlSettings.DBPath))
	addBool("loggingEnabled", sqliteSettings.LoggingEnabled, yamlSettings.LoggingEnabled)
	addBool("usageMetricsPersistence", sqliteSettings.UsageMetricsPersistence, yamlSettings.UsageMetricsPersistence)
	addBool("activityPersistence", sqliteSettings.ActivityPersistence, yamlSettings.ActivityPersistence)
	addBool("activityCapturePersistence", sqliteSettings.ActivityCapturePersistence, yamlSettings.ActivityCapturePersistence)
	addBool("captureRedactHeaders", sqliteSettings.CaptureRedactHeaders, yamlSettings.CaptureRedactHeaders)
	addBool("activityFields.model", sqliteSettings.ActivityFields.Model, yamlSettings.ActivityFields.Model)
	addBool("activityFields.tokens", sqliteSettings.ActivityFields.Tokens, yamlSettings.ActivityFields.Tokens)
	addBool("activityFields.speeds", sqliteSettings.ActivityFields.Speeds, yamlSettings.ActivityFields.Speeds)
	addBool("activityFields.duration", sqliteSettings.ActivityFields.Duration, yamlSettings.ActivityFields.Duration)
	return conflicts
}

func mergePersistenceConflicts(groups ...[]persistenceConflict) []persistenceConflict {
	merged := []persistenceConflict{}
	seen := map[string]struct{}{}
	for _, group := range groups {
		for _, conflict := range group {
			key := conflict.Field + "\x00" + conflict.YAMLValue + "\x00" + conflict.SQLiteValue
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, conflict)
		}
	}
	return merged
}

func (pm *ProxyManager) writePersistenceSettingsToYAML(settings persistenceSettings) error {
	if strings.TrimSpace(pm.config.ConfigPath) == "" {
		return fmt.Errorf("config file path is unavailable")
	}
	settings = normalizePersistenceSettings(settings)

	if err := writePersistenceSettingsYAML(pm.config.ConfigPath, settings); err != nil {
		return err
	}

	pm.config.MetricsDBPath = settings.DBPath
	pm.config.LoggingEnabled = settings.LoggingEnabled
	pm.config.UsageMetricsPersistence = settings.UsageMetricsPersistence
	pm.config.ActivityPersistence = settings.ActivityPersistence
	pm.config.ActivityCapturePersistence = settings.ActivityCapturePersistence
	pm.config.CaptureRedactHeaders = settings.CaptureRedactHeaders
	pm.config.ActivityFields = activityFieldsConfig(settings.ActivityFields)
	return nil
}

func writePersistenceSettingsYAML(path string, settings persistenceSettings) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var document yaml.Node
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &document); err != nil {
			return fmt.Errorf("parse config file: %w", err)
		}
	}
	root := yamlRootMapping(&document)

	setYAMLString(root, "metricsDBPath", settings.DBPath)
	setYAMLBool(root, "loggingEnabled", settings.LoggingEnabled)
	setYAMLBool(root, "usageMetricsPersistence", settings.UsageMetricsPersistence)
	setYAMLBool(root, "activityPersistence", settings.ActivityPersistence)
	setYAMLBool(root, "activityCapturePersistence", settings.ActivityCapturePersistence)
	setYAMLBool(root, "captureRedactHeaders", settings.CaptureRedactHeaders)
	setYAMLActivityFields(root, settings.ActivityFields)

	out, err := yaml.Marshal(&document)
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".llama-swap-config-*.yaml")
	if err != nil {
		return fmt.Errorf("create config temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write config temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close config temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}

func yamlRootMapping(document *yaml.Node) *yaml.Node {
	if document.Kind != yaml.DocumentNode {
		document.Kind = yaml.DocumentNode
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		root.Kind = yaml.MappingNode
		root.Tag = "!!map"
		root.Content = nil
	}
	return root
}

func setYAMLActivityFields(root *yaml.Node, fields activityFieldsSettings) {
	node := mappingValue(root, "activityFields")
	node.Kind = yaml.MappingNode
	node.Tag = "!!map"
	node.Content = nil
	setYAMLBool(node, "model", fields.Model)
	setYAMLBool(node, "tokens", fields.Tokens)
	setYAMLBool(node, "speeds", fields.Speeds)
	setYAMLBool(node, "duration", fields.Duration)
}

func setYAMLString(root *yaml.Node, key, value string) {
	node := mappingValue(root, key)
	node.Kind = yaml.ScalarNode
	node.Tag = "!!str"
	node.Value = value
	node.Content = nil
}

func setYAMLBool(root *yaml.Node, key string, value bool) {
	node := mappingValue(root, key)
	node.Kind = yaml.ScalarNode
	node.Tag = "!!bool"
	node.Value = strconv.FormatBool(value)
	node.Content = nil
}

func mappingValue(root *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return root.Content[i+1]
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{}
	root.Content = append(root.Content, keyNode, valueNode)
	return valueNode
}
