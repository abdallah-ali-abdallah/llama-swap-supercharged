package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mostlygeek/llama-swap/event"
	"github.com/mostlygeek/llama-swap/proxy/config"
	"gopkg.in/yaml.v3"
)

type Model struct {
	Id          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	State       string                  `json:"state"`
	Unlisted    bool                    `json:"unlisted"`
	PeerID      string                  `json:"peerID"`
	Aliases     []string                `json:"aliases,omitempty"`
	Memory      *LlamaCppMemorySnapshot `json:"memory,omitempty"`
}

type ModelConfiguration struct {
	ModelID       string   `json:"modelID"`
	Cmd           string   `json:"cmd"`
	Proxy         string   `json:"proxy"`
	Env           []string `json:"env,omitempty"`
	CheckEndpoint string   `json:"checkEndpoint"`
	TTL           int      `json:"ttl"`
	YAML          string   `json:"yaml"`
}

func addApiHandlers(pm *ProxyManager) {
	// Add API endpoints for React to consume
	// Protected with API key authentication
	apiGroup := pm.ginEngine.Group("/api", pm.apiKeyAuth())
	{
		apiGroup.GET("/models/config/*model", pm.apiGetModelConfig)
		apiGroup.POST("/models/unload", pm.apiUnloadAllModels)
		apiGroup.POST("/models/unload/*model", pm.apiUnloadSingleModelHandler)
		apiGroup.GET("/events", pm.apiSendEvents)
		apiGroup.GET("/metrics", pm.apiGetMetrics)
		apiGroup.GET("/settings/persistence", pm.apiGetPersistenceSettings)
		apiGroup.PUT("/settings/persistence", pm.apiUpdatePersistenceSettings)
		apiGroup.GET("/version", pm.apiGetVersion)
		apiGroup.GET("/captures/:id", pm.apiGetCapture)
	}
}

func (pm *ProxyManager) apiUnloadAllModels(c *gin.Context) {
	pm.StopProcesses(StopImmediately)
	c.JSON(http.StatusOK, gin.H{"msg": "ok"})
}

func (pm *ProxyManager) getModelStatus() []Model {
	// Extract keys and sort them
	models := []Model{}

	modelIDs := make([]string, 0, len(pm.config.Models))
	for modelID := range pm.config.Models {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)

	// Iterate over sorted keys
	for _, modelID := range modelIDs {
		// Get process state
		state := "unknown"
		var process *Process
		if pm.matrix != nil {
			process, _ = pm.matrix.GetProcess(modelID)
		} else {
			processGroup := pm.findGroupByModelName(modelID)
			if processGroup != nil {
				process = processGroup.processes[modelID]
			}
		}
		var memory *LlamaCppMemorySnapshot
		if process != nil {
			switch process.CurrentState() {
			case StateReady:
				state = "ready"
				memory = process.MemorySnapshot()
			case StateStarting:
				state = "starting"
				memory = process.MemorySnapshot()
			case StateStopping:
				state = "stopping"
				memory = process.MemorySnapshot()
			case StateShutdown:
				state = "shutdown"
			case StateStopped:
				state = "stopped"
			}
		}
		models = append(models, Model{
			Id:          modelID,
			Name:        pm.config.Models[modelID].Name,
			Description: pm.config.Models[modelID].Description,
			State:       state,
			Unlisted:    pm.config.Models[modelID].Unlisted,
			Aliases:     pm.config.Models[modelID].Aliases,
			Memory:      memory,
		})
	}

	// Iterate over the peer models
	if pm.peerProxy != nil {
		for peerID, peer := range pm.peerProxy.ListPeers() {
			for _, modelID := range peer.Models {
				models = append(models, Model{
					Id:     modelID,
					PeerID: peerID,
				})
			}
		}
	}

	return models
}

type messageType string

const (
	msgTypeModelStatus messageType = "modelStatus"
	msgTypeLogData     messageType = "logData"
	msgTypeMetrics     messageType = "metrics"
	msgTypeInFlight    messageType = "inflight"
)

type messageEnvelope struct {
	Type messageType `json:"type"`
	Data string      `json:"data"`
}

// sends a stream of different message types that happen on the server
func (pm *ProxyManager) apiSendEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Content-Type-Options", "nosniff")
	// prevent nginx from buffering SSE
	c.Header("X-Accel-Buffering", "no")

	sendBuffer := make(chan messageEnvelope, 25)
	ctx, cancel := context.WithCancel(c.Request.Context())
	sendModels := func() {
		data, err := json.Marshal(pm.getModelStatus())
		if err == nil {
			msg := messageEnvelope{Type: msgTypeModelStatus, Data: string(data)}
			select {
			case sendBuffer <- msg:
			case <-ctx.Done():
				return
			default:
			}

		}
	}

	sendLogData := func(source string, data []byte) {
		data, err := json.Marshal(gin.H{
			"source": source,
			"data":   string(data),
		})
		if err == nil {
			select {
			case sendBuffer <- messageEnvelope{Type: msgTypeLogData, Data: string(data)}:
			case <-ctx.Done():
				return
			default:
			}
		}
	}

	sendMetrics := func(metrics []TokenMetrics) {
		filtered := pm.filterExcludedMetrics(metrics)
		if len(filtered) == 0 {
			return
		}
		jsonData, err := json.Marshal(filtered)
		if err == nil {
			select {
			case sendBuffer <- messageEnvelope{Type: msgTypeMetrics, Data: string(jsonData)}:
			case <-ctx.Done():
				return
			default:
			}
		}
	}

	sendInFlight := func(total int) {
		jsonData, err := json.Marshal(gin.H{"total": total})
		if err == nil {
			select {
			case sendBuffer <- messageEnvelope{Type: msgTypeInFlight, Data: string(jsonData)}:
			case <-ctx.Done():
				return
			default:
			}
		}
	}

	/**
	 * Send updated models list
	 */
	defer event.On(func(e ProcessStateChangeEvent) {
		sendModels()
	})()
	defer event.On(func(e ConfigFileChangedEvent) {
		sendModels()
	})()

	/**
	 * Send Log data
	 */
	defer pm.proxyLogger.OnLogData(func(data []byte) {
		sendLogData("proxy", data)
	})()
	defer pm.upstreamLogger.OnLogData(func(data []byte) {
		sendLogData("upstream", data)
	})()

	/**
	 * Send Metrics data
	 */
	defer event.On(func(e TokenMetricsEvent) {
		sendMetrics([]TokenMetrics{e.Metrics})
	})()

	/**
	 * Send in-flight request stats related to token stats "Waiting: N" count.
	 */
	defer event.On(func(e InFlightRequestsEvent) {
		sendInFlight(e.Total)
	})()

	// send initial batch of data
	sendLogData("proxy", pm.proxyLogger.GetHistory())
	sendLogData("upstream", pm.upstreamLogger.GetHistory())
	sendModels()
	sendMetrics(pm.metricsMonitor.getMetrics())
	sendInFlight(pm.inFlightCounter.Current())

	for {
		select {
		case <-c.Request.Context().Done():
			cancel()
			return
		case <-pm.shutdownCtx.Done():
			cancel()
			return
		case msg := <-sendBuffer:
			c.SSEvent("message", msg)
			c.Writer.Flush()
		}
	}
}

func (pm *ProxyManager) apiGetMetrics(c *gin.Context) {
	rangeName := strings.TrimSpace(c.Query("range"))
	if rangeName == "" || rangeName == "realtime" {
		metrics := pm.filterExcludedMetrics(pm.metricsMonitor.getMetrics())
		jsonData, err := json.Marshal(metrics)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metrics"})
			return
		}
		c.Header("X-Metrics-Range", "realtime")
		c.Header("X-Metrics-Truncated", "false")
		c.Data(http.StatusOK, "application/json", jsonData)
		return
	}

	query, normalizedRange, err := pm.parseMetricsRangeQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	metrics, truncated, err := pm.metricsMonitor.getMetricsForRange(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metrics"})
		return
	}
	jsonData, err := json.Marshal(pm.filterExcludedMetrics(metrics))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get metrics"})
		return
	}
	c.Header("X-Metrics-Range", normalizedRange)
	c.Header("X-Metrics-Truncated", strconv.FormatBool(truncated))
	c.Data(http.StatusOK, "application/json", jsonData)
}

func (pm *ProxyManager) excludeModelFromMetrics(modelID string) bool {
	modelConfig, found := pm.config.Models[modelID]
	return found && modelConfig.ExcludeFromMetrics
}

func (pm *ProxyManager) filterExcludedMetrics(metrics []TokenMetrics) []TokenMetrics {
	if len(metrics) == 0 {
		return metrics
	}

	filtered := make([]TokenMetrics, 0, len(metrics))
	excluded := false
	for _, metric := range metrics {
		if pm.excludeModelFromMetrics(metric.Model) {
			excluded = true
			continue
		}
		filtered = append(filtered, metric)
	}
	if !excluded {
		return metrics
	}
	return filtered
}

func (pm *ProxyManager) parseMetricsRangeQuery(c *gin.Context) (metricsQuery, string, error) {
	limit := metricsLimit(c.Query("limit"), pm.config.MetricsQueryMaxRows)
	now := time.Now()
	rangeName := strings.ToLower(strings.TrimSpace(c.Query("range")))
	scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	if scope != "" && scope != "usage" && scope != "activity" {
		return metricsQuery{}, "", fmt.Errorf("unsupported metrics scope %q", scope)
	}
	query := metricsQuery{Limit: limit, Scope: scope}

	setFrom := func(duration time.Duration) {
		from := now.Add(-duration)
		query.From = &from
	}

	switch rangeName {
	case "5m", "past_5m", "past-5m", "past_5min", "past-5min":
		setFrom(5 * time.Minute)
		return query, "5m", nil
	case "10m", "past_10m", "past-10m", "past_10min", "past-10min":
		setFrom(10 * time.Minute)
		return query, "10m", nil
	case "1h", "past_1h", "past-1h", "past_hour":
		setFrom(time.Hour)
		return query, "1h", nil
	case "8h", "past_8h", "past-8h":
		setFrom(8 * time.Hour)
		return query, "8h", nil
	case "1d", "24h", "day", "past_day":
		setFrom(24 * time.Hour)
		return query, "1d", nil
	case "1w", "week", "past_week":
		setFrom(7 * 24 * time.Hour)
		return query, "1w", nil
	case "1mo", "month", "past_month":
		setFrom(30 * 24 * time.Hour)
		return query, "1mo", nil
	case "all":
		return query, "all", nil
	case "custom":
		from, hasFrom, err := parseMetricRangeTime(c.Query("from"))
		if err != nil {
			return metricsQuery{}, "", fmt.Errorf("invalid from: %w", err)
		}
		to, hasTo, err := parseMetricRangeTime(c.Query("to"))
		if err != nil {
			return metricsQuery{}, "", fmt.Errorf("invalid to: %w", err)
		}
		if !hasFrom && !hasTo {
			return metricsQuery{}, "", fmt.Errorf("custom range requires from or to")
		}
		if hasFrom {
			query.From = &from
		}
		if hasTo {
			query.To = &to
		}
		if hasFrom && hasTo && from.After(to) {
			return metricsQuery{}, "", fmt.Errorf("from must be before to")
		}
		return query, "custom", nil
	default:
		return metricsQuery{}, "", fmt.Errorf("unsupported metrics range %q", rangeName)
	}
}

func metricsLimit(value string, configuredMax int) int {
	if configuredMax <= 0 {
		configuredMax = defaultMetricsQueryMaxRows
	}
	if strings.TrimSpace(value) == "" {
		return configuredMax
	}

	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return configuredMax
	}
	if limit > configuredMax {
		return configuredMax
	}
	return limit
}

func parseMetricRangeTime(value string) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, nil
	}

	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true, nil
	}

	unixValue, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, false, err
	}
	if unixValue > 1_000_000_000_000 {
		return time.UnixMilli(unixValue), true, nil
	}
	return time.Unix(unixValue, 0), true, nil
}

func (pm *ProxyManager) apiGetPersistenceSettings(c *gin.Context) {
	settings := pm.persistenceSettingsWithYAMLPriority()
	c.JSON(http.StatusOK, settings)
}

func (pm *ProxyManager) apiUpdatePersistenceSettings(c *gin.Context) {
	var settings persistenceSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid persistence settings"})
		return
	}
	if strings.TrimSpace(settings.DBPath) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "db_path is required"})
		return
	}
	settings.DBPath = resolveMetricsDBPath(pm.config, settings.DBPath)
	settings = normalizePersistenceSettings(settings)

	update, _, err := pm.metricsMonitor.stagePersistenceSettings(settings)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	defer update.close()

	if err := pm.writePersistenceSettingsToYAML(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated, err := update.commit()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	pm.applyPersistenceSettingsToConfig(updated)
	pm.applyLoggingEnabled(updated.LoggingEnabled)
	pm.metricsMonitor.setYAMLConflicts(nil)
	updated.YAMLAvailable = true
	updated.YAMLPath = pm.config.ConfigPath
	updated.Stats = pm.metricsMonitor.persistenceSettings().Stats
	c.JSON(http.StatusOK, updated)
}

func (pm *ProxyManager) applyLoggingEnabled(enabled bool) {
	if pm.proxyLogger != nil {
		pm.proxyLogger.SetEnabled(enabled)
	}
	if pm.upstreamLogger != nil {
		pm.upstreamLogger.SetEnabled(enabled)
	}
	if pm.muxLogger != nil {
		pm.muxLogger.SetEnabled(enabled)
	}
}

func (pm *ProxyManager) apiUnloadSingleModelHandler(c *gin.Context) {
	requestedModel := strings.TrimPrefix(c.Param("model"), "/")
	realModelName, found := pm.config.RealModelName(requestedModel)
	if !found {
		pm.sendErrorResponse(c, http.StatusNotFound, "Model not found")
		return
	}

	var stopErr error
	if pm.matrix != nil {
		stopErr = pm.matrix.StopProcess(realModelName, StopImmediately)
	} else {
		processGroup := pm.findGroupByModelName(realModelName)
		if processGroup == nil {
			pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("process group not found for model %s", requestedModel))
			return
		}
		stopErr = processGroup.StopProcess(realModelName, StopImmediately)
	}

	if stopErr != nil {
		pm.sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("error stopping process: %s", stopErr.Error()))
		return
	}
	c.String(http.StatusOK, "OK")
}

func (pm *ProxyManager) apiGetModelConfig(c *gin.Context) {
	requestedModel := strings.TrimPrefix(c.Param("model"), "/")
	realModelName, found := pm.config.RealModelName(requestedModel)
	if !found {
		pm.sendErrorResponse(c, http.StatusNotFound, "Model not found")
		return
	}

	modelConfig := pm.config.Models[realModelName]
	c.JSON(http.StatusOK, ModelConfiguration{
		ModelID:       realModelName,
		Cmd:           modelConfig.Cmd,
		Proxy:         modelConfig.Proxy,
		Env:           modelConfig.Env,
		CheckEndpoint: modelConfig.CheckEndpoint,
		TTL:           modelConfig.UnloadAfter,
		YAML:          pm.modelConfigYAML(realModelName, modelConfig),
	})
}

func (pm *ProxyManager) modelConfigYAML(modelID string, modelConfig config.ModelConfig) string {
	if rawYAML, err := pm.rawModelConfigYAML(modelID); err == nil && rawYAML != "" {
		return rawYAML
	}

	data, err := yaml.Marshal(modelConfig)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (pm *ProxyManager) rawModelConfigYAML(modelID string) (string, error) {
	if strings.TrimSpace(pm.config.ConfigPath) == "" {
		return "", nil
	}

	data, err := os.ReadFile(pm.config.ConfigPath)
	if err != nil {
		return "", err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", err
	}
	if len(root.Content) == 0 {
		return "", nil
	}

	document := root.Content[0]
	if document.Kind != yaml.MappingNode {
		return "", nil
	}

	for i := 0; i+1 < len(document.Content); i += 2 {
		if document.Content[i].Value != "models" {
			continue
		}

		modelsNode := document.Content[i+1]
		if modelsNode.Kind != yaml.MappingNode {
			return "", nil
		}

		for j := 0; j+1 < len(modelsNode.Content); j += 2 {
			if modelsNode.Content[j].Value != modelID {
				continue
			}

			data, err := yaml.Marshal(modelsNode.Content[j+1])
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(data)), nil
		}
	}

	return "", nil
}

func (pm *ProxyManager) apiGetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"version":    pm.version,
		"commit":     pm.commit,
		"build_date": pm.buildDate,
	})
}

func (pm *ProxyManager) apiGetCapture(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capture ID"})
		return
	}

	data, exists := pm.metricsMonitor.getCompressedBytes(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "capture not found"})
		return
	}

	c.Header("Vary", "Accept-Encoding")

	// ¯\_(ツ)_/¯ quality weights are too fancy for us anyway
	hasZstd := strings.Contains(c.GetHeader("Accept-Encoding"), "zstd")

	if hasZstd {
		c.Header("Content-Encoding", "zstd")
		c.Data(http.StatusOK, "application/json", data)
	} else {
		decompressed, err := decompressCapture(data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decompress capture"})
			return
		}
		c.Data(http.StatusOK, "application/json", decompressed)
	}
}
