package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/mostlygeek/llama-swap/event"
	"github.com/tidwall/gjson"
)

// zstdEncOptions are the shared zstd encoder options for maximum compression.
var zstdEncOptions = []zstd.EOption{
	zstd.WithEncoderLevel(zstd.SpeedBetterCompression),
}

// zstdDecOptions are the shared zstd decoder options.
var zstdDecOptions = []zstd.DOption{}

// zstdEncPool pools zstd.Encoder instances to reduce allocations.
var zstdEncPool = &sync.Pool{
	New: func() interface{} {
		enc, _ := zstd.NewWriter(nil, zstdEncOptions...)
		return enc
	},
}

// zstdDecPool pools zstd.Decoder instances to reduce allocations.
var zstdDecPool = &sync.Pool{
	New: func() interface{} {
		dec, _ := zstd.NewReader(nil, zstdDecOptions...)
		return dec
	},
}

// compressCapture marshals a ReqRespCapture to JSON and compresses it with zstd.
// Returns compressed bytes and the original JSON byte count for logging.
func compressCapture(c *ReqRespCapture) ([]byte, int, error) {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal capture: %w", err)
	}
	enc := zstdEncPool.Get().(*zstd.Encoder)
	defer zstdEncPool.Put(enc)
	return enc.EncodeAll(jsonBytes, nil), len(jsonBytes), nil
}

// decompressCapture decompresses zstd-compressed JSON and returns it.
func decompressCapture(data []byte) ([]byte, error) {
	dec := zstdDecPool.Get().(*zstd.Decoder)
	defer zstdDecPool.Put(dec)
	return dec.DecodeAll(data, nil)
}

// TokenMetrics represents parsed token statistics from llama-server logs
type TokenMetrics struct {
	ID              int       `json:"id"`
	Timestamp       time.Time `json:"timestamp"`
	Model           string    `json:"model"`
	CachedTokens    int       `json:"cache_tokens"`
	NewInputTokens  int       `json:"new_input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	PromptPerSecond float64   `json:"prompt_per_second"`
	TokensPerSecond float64   `json:"tokens_per_second"`
	DurationMs      int       `json:"duration_ms"`
	PromptMs        int       `json:"prompt_ms"`
	PredictedMs     int       `json:"predicted_ms"`
	HasCapture      bool      `json:"has_capture"`

	// Speculative decoding stats (0 = not using spec decode)
	DraftAcceptanceRate float64 `json:"draft_acceptance_rate"`
	AcceptedDrafts      int     `json:"accepted_drafts"`
	GeneratedDrafts     int     `json:"generated_drafts"`
}

type ReqRespCapture struct {
	ID          int               `json:"id"`
	ReqPath     string            `json:"req_path"`
	ReqHeaders  map[string]string `json:"req_headers"`
	ReqBody     []byte            `json:"req_body"`
	RespHeaders map[string]string `json:"resp_headers"`
	RespBody    []byte            `json:"resp_body"`
}

// TokenMetricsEvent represents a token metrics event
type TokenMetricsEvent struct {
	Metrics TokenMetrics
}

func (e TokenMetricsEvent) Type() uint32 {
	return TokenMetricsEventID // defined in events.go
}

// metricsMonitor parses llama-server output for token statistics
type metricsMonitor struct {
	mu         sync.RWMutex
	metrics    []TokenMetrics
	maxMetrics int
	nextID     int
	logger     *LogMonitor
	store      *metricsStore

	// capture fields
	enableCaptures bool
	captures       map[int][]byte // zstd-compressed JSON of ReqRespCapture
	captureOrder   []int          // track insertion order for FIFO eviction
	captureSize    int            // current total compressed size in bytes
	maxCaptureSize int            // max bytes for captures (uncompressed)

	// spec decode parser for tracking draft acceptance rates from upstream llama-server logs
	specParser     *specDecodeParser
	specParserStop func()

	liveActivity *liveActivityTracker
}

// newMetricsMonitor creates a new metricsMonitor. captureBufferMB is the
// capture buffer size in megabytes; 0 disables captures.
// upstreamLogger is the optional upstream logger that captures llama-server stdout,
// used for parsing speculative decoding stats (draft acceptance rate lines).
func newMetricsMonitor(logger *LogMonitor, maxMetrics int, captureBufferMB int, upstreamLogger *LogMonitor, stores ...*metricsStore) *metricsMonitor {
	var store *metricsStore
	if len(stores) > 0 {
		store = stores[0]
	}

	mp := &metricsMonitor{
		logger:         logger,
		maxMetrics:     maxMetrics,
		store:          store,
		enableCaptures: captureBufferMB > 0,
		captures:       make(map[int][]byte),
		captureOrder:   make([]int, 0),
		captureSize:    0,
		maxCaptureSize: captureBufferMB * 1024 * 1024,
	}

	// Start spec decode parser to track draft acceptance rates from upstream llama-server logs.
	// The upstream logger captures the llama-server stdout which includes "draft acceptance rate" lines.
	// If no upstream logger is provided, fall back to the proxy logger (for tests).
	var eventBus *event.Dispatcher
	if upstreamLogger != nil {
		eventBus = upstreamLogger.eventbus
	} else {
		eventBus = logger.eventbus
	}
	mp.specParser = newSpecDecodeParser(logger)
	mp.specParserStop = event.Subscribe(eventBus, func(e LogDataEvent) {
		mp.specParser.parseChunk(e.Data)
	})

	// Load persisted metrics if store is available
	if store != nil {
		if metrics, err := store.latest(maxMetrics); err != nil {
			if logger != nil {
				logger.Warnf("failed to load metrics from %s: %v", store.path, err)
			}
		} else {
			mp.metrics = metrics
		}

		if maxID, err := store.maxID(); err != nil {
			if logger != nil {
				logger.Warnf("failed to read max metric id from %s: %v", store.path, err)
			}
		} else {
			mp.nextID = maxID + 1
		}
	}

	return mp
}

// addMetrics adds a new metric to the collection and publishes an event.
// Returns the assigned metric ID.
func (mp *metricsMonitor) addMetrics(metric TokenMetrics) int {
	mp.mu.Lock()
	metric.ID = mp.nextID
	mp.nextID++
	mp.metrics = append(mp.metrics, metric)
	if len(mp.metrics) > mp.maxMetrics {
		mp.metrics = mp.metrics[len(mp.metrics)-mp.maxMetrics:]
	}
	store := mp.store
	mp.mu.Unlock()

	if store != nil {
		if err := store.insert(metric); err != nil && mp.logger != nil {
			mp.logger.Warnf("failed to persist metric %d: %v", metric.ID, err)
		}
	}

	event.Emit(TokenMetricsEvent{Metrics: metric})
	return metric.ID
}

func (mp *metricsMonitor) enrichWithDraftStats(metric *TokenMetrics) {
	if mp.specParser == nil || metric.GeneratedDrafts > 0 {
		return
	}

	requestEnd := metric.Timestamp
	if requestEnd.IsZero() {
		requestEnd = time.Now()
		metric.Timestamp = requestEnd
	}

	stats, ok := mp.specParser.consumeClosestTo(requestEnd, specDecodeWaitTimeout)
	if !ok {
		return
	}

	metric.DraftAcceptanceRate = stats.AcceptanceRate
	metric.AcceptedDrafts = stats.AcceptedDrafts
	metric.GeneratedDrafts = stats.GeneratedDrafts
}

func (mp *metricsMonitor) close() {
	// Stop spec decode parser
	if mp.specParserStop != nil {
		mp.specParserStop()
		mp.specParserStop = nil
	}

	mp.mu.RLock()
	store := mp.store
	mp.mu.RUnlock()
	if store != nil {
		store.close()
	}
}

// addCapture adds a new capture to the buffer with size-based eviction.
// Captures are skipped if enableCaptures is false or if compressed data exceeds maxCaptureSize.
func (mp *metricsMonitor) addCapture(capture ReqRespCapture) {
	if !mp.enableCaptures {
		return
	}

	compressed, uncompressedBytes, err := compressCapture(&capture)
	if err != nil {
		mp.logger.Warnf("failed to compress capture: %v, skipping", err)
		return
	}

	captureSize := len(compressed)
	if captureSize > mp.maxCaptureSize {
		mp.logger.Warnf("compressed capture size %d exceeds max %d, skipping", captureSize, mp.maxCaptureSize)
		return
	}

	compressionRatio := (1 - float64(captureSize)/float64(uncompressedBytes)) * 100

	mp.mu.Lock()

	// Evict oldest (FIFO) until room available for the compressed data
	for mp.captureSize+captureSize > mp.maxCaptureSize && len(mp.captureOrder) > 0 {
		oldestID := mp.captureOrder[0]
		mp.captureOrder = mp.captureOrder[1:]
		if evicted, exists := mp.captures[oldestID]; exists {
			l := len(evicted)
			mp.captureSize -= l
			delete(mp.captures, oldestID)
			mp.logger.Debugf("Capture %d evicted to make space: %d bytes", oldestID, l)
		}
	}

	mp.captures[capture.ID] = compressed
	mp.captureOrder = append(mp.captureOrder, capture.ID)
	mp.captureSize += captureSize

	mp.logger.Debugf("Capture %d compressed and saved: %d bytes -> %d bytes (%.1f%% compression)", capture.ID, uncompressedBytes, len(compressed), compressionRatio)

	store := mp.store
	mp.mu.Unlock()

	if store != nil {
		if err := store.insertCapture(capture.ID, compressed); err != nil && mp.logger != nil {
			mp.logger.Warnf("failed to persist capture %d: %v", capture.ID, err)
		}
	}
}

// getCompressedBytes returns the raw compressed bytes for a capture by ID.
func (mp *metricsMonitor) getCompressedBytes(id int) ([]byte, bool) {
	mp.mu.RLock()
	data, exists := mp.captures[id]
	store := mp.store
	mp.mu.RUnlock()
	if exists {
		return data, true
	}

	if store == nil {
		return nil, false
	}

	data, exists, err := store.getCapture(id)
	if err != nil {
		if mp.logger != nil {
			mp.logger.Warnf("failed to read persisted capture %d: %v", id, err)
		}
		return nil, false
	}
	return data, exists
}

// getCaptureByID returns decompressed capture bytes if found and decompress=true.
// If decompress=false, returns the raw zstd-compressed bytes.
// Returns nil if the capture is not found.
func (mp *metricsMonitor) getCaptureByID(id int, decompress bool) []byte {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	data, exists := mp.captures[id]
	if !exists {
		return nil
	}

	if !decompress {
		return data
	}

	decompressed, err := decompressCapture(data)
	if err != nil {
		mp.logger.Warnf("failed to decompress capture %d: %v", id, err)
		return nil
	}

	return decompressed
}

// getMetrics returns a copy of the current metrics
func (mp *metricsMonitor) getMetrics() []TokenMetrics {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	result := make([]TokenMetrics, len(mp.metrics))
	copy(result, mp.metrics)
	return result
}

// getMetricsJSON returns metrics as JSON
func (mp *metricsMonitor) getMetricsJSON() ([]byte, error) {
	return json.Marshal(mp.getMetrics())
}

func (mp *metricsMonitor) getMetricsForRange(q metricsQuery) ([]TokenMetrics, bool, error) {
	mp.mu.RLock()
	store := mp.store
	mp.mu.RUnlock()

	if store != nil {
		metrics, truncated, err := store.query(q)
		if err != nil {
			return nil, false, err
		}
		mp.markMemoryCapturesAvailable(metrics)
		return metrics, truncated, nil
	}

	metrics := mp.getMetrics()
	filtered := make([]TokenMetrics, 0, len(metrics))
	for _, metric := range metrics {
		if q.From != nil && metric.Timestamp.Before(*q.From) {
			continue
		}
		if q.To != nil && metric.Timestamp.After(*q.To) {
			continue
		}
		filtered = append(filtered, metric)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = defaultMetricsQueryMaxRows
	}
	truncated := len(filtered) > limit
	if truncated {
		filtered = filtered[:limit]
	}
	return filtered, truncated, nil
}

func (mp *metricsMonitor) markMemoryCapturesAvailable(metrics []TokenMetrics) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	for i := range metrics {
		if _, exists := mp.captures[metrics[i].ID]; exists {
			metrics[i].HasCapture = true
		}
	}
}

//lint:ignore U1000 retained for API paths that serialize monitor range results directly.
func (mp *metricsMonitor) getMetricsForRangeJSON(q metricsQuery) ([]byte, bool, error) {
	metrics, truncated, err := mp.getMetricsForRange(q)
	if err != nil {
		return nil, false, err
	}
	jsonData, err := json.Marshal(metrics)
	return jsonData, truncated, err
}

func (mp *metricsMonitor) persistenceSettings() persistenceSettings {
	mp.mu.RLock()
	store := mp.store
	mp.mu.RUnlock()
	if store == nil {
		return persistenceSettings{
			SQLiteAvailable:      false,
			RetentionDays:        0,
			LoggingEnabled:       true,
			CaptureRedactHeaders: true,
			ActivityFields: activityFieldsSettings{
				Model:    true,
				Tokens:   true,
				Speeds:   true,
				Duration: true,
			},
		}
	}
	settings := store.settings()
	if stats, err := store.stats(); err == nil {
		settings.Stats = &stats
	} else if mp.logger != nil {
		mp.logger.Warnf("failed to read metrics persistence stats from %s: %v", store.path, err)
	}
	return settings
}

func (mp *metricsMonitor) yamlConflicts() []persistenceConflict {
	mp.mu.RLock()
	store := mp.store
	mp.mu.RUnlock()
	if store == nil {
		return nil
	}
	return store.yamlConflictsSnapshot()
}

func (mp *metricsMonitor) setYAMLConflicts(conflicts []persistenceConflict) {
	mp.mu.RLock()
	store := mp.store
	mp.mu.RUnlock()
	if store != nil {
		store.setYAMLConflicts(conflicts)
	}
}

type stagedPersistenceSettingsUpdate struct {
	monitor          *metricsMonitor
	currentStore     *metricsStore
	newStore         *metricsStore
	settings         persistenceSettings
	rollbackSettings *persistenceSettings
	closed           bool
}

func (mp *metricsMonitor) stagePersistenceSettings(settings persistenceSettings) (*stagedPersistenceSettingsUpdate, persistenceSettings, error) {
	mp.mu.RLock()
	store := mp.store
	mp.mu.RUnlock()
	if store == nil {
		return nil, mp.persistenceSettings(), fmt.Errorf("metrics persistence is unavailable")
	}

	current := store.settings()
	if settings.DBPath == "" {
		settings.DBPath = current.DBPath
	}
	settings = normalizePersistenceSettings(settings)
	if settings.DBPath != current.DBPath {
		newStore, err := newMetricsStoreWithOptions(
			settings.DBPath,
			store.retentionDays,
			store.defaultQueryRows,
			settings.UsageMetricsPersistence,
			settings.ActivityPersistence,
			settings.ActivityCapturePersistence,
			activityFieldsConfig(settings.ActivityFields),
			store.logger,
		)
		if err != nil {
			return nil, current, err
		}
		if err := newStore.updateSettings(settings); err != nil {
			newStore.close()
			return nil, current, err
		}
		if err := store.saveSettings(settings); err != nil {
			newStore.close()
			return nil, current, err
		}
		rollbackSettings := current

		return &stagedPersistenceSettingsUpdate{
			monitor:          mp,
			currentStore:     store,
			newStore:         newStore,
			settings:         settings,
			rollbackSettings: &rollbackSettings,
		}, newStore.settings(), nil
	}

	if err := store.validateSettings(settings); err != nil {
		return nil, current, err
	}
	return &stagedPersistenceSettingsUpdate{
		monitor:      mp,
		currentStore: store,
		settings:     settings,
	}, current, nil
}

func (update *stagedPersistenceSettingsUpdate) commit() (persistenceSettings, error) {
	if update == nil || update.closed {
		return persistenceSettings{}, fmt.Errorf("persistence settings update is unavailable")
	}

	if update.newStore == nil {
		if err := update.currentStore.updateSettings(update.settings); err != nil {
			return update.currentStore.settings(), err
		}
		update.closed = true
		return update.currentStore.settings(), nil
	}

	mp := update.monitor
	mp.mu.Lock()
	if mp.store != update.currentStore {
		mp.mu.Unlock()
		update.close()
		return mp.persistenceSettings(), fmt.Errorf("metrics persistence store changed while updating settings")
	}
	if maxID, err := update.newStore.maxID(); err == nil && maxID >= mp.nextID {
		mp.nextID = maxID + 1
	}
	oldStore := update.currentStore
	activeStore := update.newStore
	mp.store = activeStore
	update.newStore = nil
	update.rollbackSettings = nil
	update.closed = true
	mp.mu.Unlock()

	oldStore.close()
	return activeStore.settings(), nil
}

func (update *stagedPersistenceSettingsUpdate) close() {
	if update == nil || update.closed {
		return
	}
	update.closed = true
	if update.newStore != nil {
		update.newStore.close()
		update.newStore = nil
	}
	if update.rollbackSettings != nil {
		if err := update.currentStore.saveSettings(*update.rollbackSettings); err != nil && update.currentStore.logger != nil {
			update.currentStore.logger.Warnf("failed to roll back staged persistence settings: %v", err)
		}
		update.rollbackSettings = nil
	}
}

func (mp *metricsMonitor) updatePersistenceSettings(settings persistenceSettings) (persistenceSettings, error) {
	update, current, err := mp.stagePersistenceSettings(settings)
	if err != nil {
		return current, err
	}
	defer update.close()
	return update.commit()
}

// wrapHandler wraps the proxy handler to extract token metrics
// if wrapHandler returns an error it is safe to assume that no
// data was sent to the client
func (mp *metricsMonitor) wrapHandler(
	modelID string,
	writer gin.ResponseWriter,
	request *http.Request,
	next func(modelID string, w http.ResponseWriter, r *http.Request) error,
) error {
	liveActivityID := ""
	if mp.liveActivity != nil {
		liveActivityID = mp.liveActivity.Start(modelID)
		defer mp.liveActivity.Finish(liveActivityID)
	}

	// Capture request body and headers if captures enabled
	var reqBody []byte
	var reqHeaders map[string]string
	if mp.enableCaptures {
		if request.Body != nil {
			var err error
			reqBody, err = io.ReadAll(request.Body)
			if err != nil {
				return fmt.Errorf("failed to read request body for capture: %w", err)
			}
			request.Body.Close()
			request.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		}
		reqHeaders = make(map[string]string)
		for key, values := range request.Header {
			if len(values) > 0 {
				reqHeaders[key] = values[0]
			}
		}
		if mp.captureRedactHeaders() {
			redactHeaders(reqHeaders)
		}
	}

	recorder := newBodyCopier(writer)

	// Filter Accept-Encoding to only include encodings we can decompress for metrics
	if ae := request.Header.Get("Accept-Encoding"); ae != "" {
		request.Header.Set("Accept-Encoding", filterAcceptEncoding(ae))
	}

	if err := next(modelID, recorder, request); err != nil {
		return err
	}

	// after this point we have to assume that data was sent to the client
	// and we can only log errors but not send them to clients

	if recorder.Status() != http.StatusOK {
		mp.logger.Warnf("metrics skipped, HTTP status=%d, path=%s", recorder.Status(), request.URL.Path)
		return nil
	}

	// Initialize default metrics - these will always be recorded
	tm := TokenMetrics{
		Timestamp:  time.Now(),
		Model:      modelID,
		DurationMs: int(time.Since(recorder.StartTime()).Milliseconds()),
	}

	body := recorder.body.Bytes()
	if len(body) == 0 {
		mp.logger.Warn("metrics: empty body, recording minimal metrics")
	} else {
		// Decompress if needed
		if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
			var err error
			body, err = decompressBody(body, encoding)
			if err != nil {
				mp.logger.Warnf("metrics: decompression failed: %v, path=%s, recording minimal metrics", err, request.URL.Path)
				body = nil
			}
		}

		if len(body) > 0 && strings.Contains(recorder.Header().Get("Content-Type"), "text/event-stream") {
			if parsed, err := processStreamingResponse(modelID, recorder.StartTime(), body); err != nil {
				mp.logger.Warnf("error processing streaming response: %v, path=%s, recording minimal metrics", err, request.URL.Path)
			} else {
				tm = parsed
			}
		} else if len(body) > 0 {
			if gjson.ValidBytes(body) {
				parsed := gjson.ParseBytes(body)
				usage := parsed.Get("usage")
				timings := parsed.Get("timings")

				// extract timings for infill - response is an array, timings are in the last element
				// see #463
				if strings.HasPrefix(request.URL.Path, "/infill") {
					if arr := parsed.Array(); len(arr) > 0 {
						timings = arr[len(arr)-1].Get("timings")
					}
				}

				if usage.Exists() || timings.Exists() {
					if parsedMetrics, err := parseMetrics(modelID, recorder.StartTime(), usage, timings); err != nil {
						mp.logger.Warnf("error parsing metrics: %v, path=%s, recording minimal metrics", err, request.URL.Path)
					} else {
						tm = parsedMetrics
					}
				}
			} else {
				mp.logger.Warnf("metrics: invalid JSON in response body path=%s, recording minimal metrics", request.URL.Path)
			}
		}
	}

	// Build capture if enabled and determine if it will be stored
	var capture *ReqRespCapture
	if mp.enableCaptures {
		respHeaders := make(map[string]string)
		for key, values := range recorder.Header() {
			if len(values) > 0 {
				respHeaders[key] = values[0]
			}
		}
		if mp.captureRedactHeaders() {
			redactHeaders(respHeaders)
		}
		delete(respHeaders, "Content-Encoding")
		capture = &ReqRespCapture{
			ReqPath:     request.URL.Path,
			ReqHeaders:  reqHeaders,
			ReqBody:     reqBody,
			RespHeaders: respHeaders,
			RespBody:    body,
		}
		compressed, _, err := compressCapture(capture)
		if err == nil && len(compressed) <= mp.maxCaptureSize {
			tm.HasCapture = true
		}
	}

	mp.enrichWithDraftStats(&tm)
	metricID := mp.addMetrics(tm)

	// Store capture if enabled
	if capture != nil {
		capture.ID = metricID
		mp.addCapture(*capture)
	}

	return nil
}

func (mp *metricsMonitor) captureRedactHeaders() bool {
	mp.mu.RLock()
	store := mp.store
	mp.mu.RUnlock()
	if store == nil {
		return true
	}
	return store.settings().CaptureRedactHeaders
}

func processStreamingResponse(modelID string, start time.Time, body []byte) (TokenMetrics, error) {
	// Iterate **backwards** through the body looking for the data payload with
	// usage data. This avoids allocating a slice of all lines via bytes.Split.

	// Start from the end of the body and scan backwards for newlines
	pos := len(body)
	for pos > 0 {
		// Find the previous newline (or start of body)
		lineStart := bytes.LastIndexByte(body[:pos], '\n')
		if lineStart == -1 {
			lineStart = 0
		} else {
			lineStart++ // Move past the newline
		}

		line := bytes.TrimSpace(body[lineStart:pos])
		pos = lineStart - 1 // Move position before the newline for next iteration

		if len(line) == 0 {
			continue
		}

		// SSE payload always follows "data:"
		prefix := []byte("data:")
		if !bytes.HasPrefix(line, prefix) {
			continue
		}
		data := bytes.TrimSpace(line[len(prefix):])

		if len(data) == 0 {
			continue
		}

		if bytes.Equal(data, []byte("[DONE]")) {
			// [DONE] line itself contains nothing of interest.
			continue
		}

		if gjson.ValidBytes(data) {
			parsed := gjson.ParseBytes(data)
			usage := parsed.Get("usage")
			timings := parsed.Get("timings")

			// v1/responses format nests usage under response.usage
			if !usage.Exists() {
				usage = parsed.Get("response.usage")
			}

			if usage.Exists() || timings.Exists() {
				return parseMetrics(modelID, start, usage, timings)
			}
		}
	}

	return TokenMetrics{}, fmt.Errorf("no valid JSON data found in stream")
}

func parseMetrics(modelID string, start time.Time, usage, timings gjson.Result) (TokenMetrics, error) {
	wallDurationMs := int(time.Since(start).Milliseconds())

	// default values
	cachedTokens := -1 // unknown or missing data
	newInputTokens := 0
	outputTokens := 0

	// timings data
	tokensPerSecond := -1.0
	promptPerSecond := -1.0
	durationMs := wallDurationMs
	promptMs := 0
	predictedMs := 0
	generatedDrafts := 0
	acceptedDrafts := 0

	if usage.Exists() {
		if ct := usage.Get("completion_tokens"); ct.Exists() {
			outputTokens = int(ct.Int())
		} else if ot := usage.Get("output_tokens"); ot.Exists() {
			outputTokens = int(ot.Int())
		}

		// Read per-request cached tokens from llama.cpp's prompt_tokens_details.cached_tokens
		if details := usage.Get("prompt_tokens_details"); details.Exists() && details.IsObject() {
			if ct := details.Get("cached_tokens"); ct.Exists() {
				cachedTokens = int(ct.Int())
			}
		} else if ct := usage.Get("cache_read_input_tokens"); ct.Exists() {
			// Anthropic v1/messages format fallback
			cachedTokens = int(ct.Int())
		}

		// Fallback: if no timings data, use prompt_tokens as new input tokens
		if !timings.Exists() || timings.Get("prompt_n").Int() == 0 {
			if pt := usage.Get("prompt_tokens"); pt.Exists() {
				newInputTokens = int(pt.Int())
			} else if it := usage.Get("input_tokens"); it.Exists() {
				newInputTokens = int(it.Int())
			}
		}
	}

	// use llama-server's timing data for tok/sec, new tokens, and duration
	if timings.Exists() {
		newInputTokens = int(timings.Get("prompt_n").Int())
		outputTokens = int(timings.Get("predicted_n").Int())
		promptPerSecond = timings.Get("prompt_per_second").Float()
		tokensPerSecond = predictedTokensPerSecond(outputTokens, timings)
		generatedDrafts = int(timings.Get("draft_n").Int())
		acceptedDrafts = int(timings.Get("draft_n_accepted").Int())
		promptMs = int(timings.Get("prompt_ms").Float())
		predictedMs = int(timings.Get("predicted_ms").Float())
		timingsDurationMs := promptMs + predictedMs
		if timingsDurationMs > durationMs {
			durationMs = timingsDurationMs
		}

		// Fallback: use cache_n from timings if cached_tokens not found in usage
		if cachedTokens == -1 {
			if cn := timings.Get("cache_n"); cn.Exists() {
				cachedTokens = int(cn.Int())
			}
		}

	} else {
		// No timings — try usage as final fallback
		if details := usage.Get("prompt_tokens_details"); details.Exists() && details.IsObject() {
			if ct := details.Get("cached_tokens"); ct.Exists() {
				cachedTokens = int(ct.Int())
			}
		} else if ct := usage.Get("cache_read_input_tokens"); ct.Exists() {
			cachedTokens = int(ct.Int())
		}
	}

	result := TokenMetrics{
		Timestamp:      time.Now(),
		Model:          modelID,
		CachedTokens:   cachedTokens,
		NewInputTokens: newInputTokens,
		OutputTokens:   outputTokens,
		// Total input = new tokens processed + cached tokens served from KV cache
		PromptPerSecond: promptPerSecond,
		TokensPerSecond: tokensPerSecond,
		DurationMs:      durationMs,
		PromptMs:        promptMs,
		PredictedMs:     predictedMs,
	}
	if generatedDrafts > 0 {
		result.GeneratedDrafts = generatedDrafts
		result.AcceptedDrafts = acceptedDrafts
		result.DraftAcceptanceRate = float64(acceptedDrafts) / float64(generatedDrafts)
	}

	return result, nil
}

func predictedTokensPerSecond(outputTokens int, timings gjson.Result) float64 {
	if outputTokens <= 1 {
		return -1
	}
	speed := timings.Get("predicted_per_second").Float()
	if speed <= 0 {
		return -1
	}
	return speed
}

// decompressBody decompresses the body based on Content-Encoding header
func decompressBody(body []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	case "deflate":
		reader := flate.NewReader(bytes.NewReader(body))
		defer reader.Close()
		return io.ReadAll(reader)
	default:
		return body, nil // Return as-is for unknown/no encoding
	}
}

// responseBodyCopier records the response body and writes to the original response writer
// while also capturing it in a buffer for later processing
type responseBodyCopier struct {
	gin.ResponseWriter
	body  *bytes.Buffer
	tee   io.Writer
	start time.Time
}

func newBodyCopier(w gin.ResponseWriter) *responseBodyCopier {
	bodyBuffer := &bytes.Buffer{}
	return &responseBodyCopier{
		ResponseWriter: w,
		body:           bodyBuffer,
		tee:            io.MultiWriter(w, bodyBuffer),
	}
}

func (w *responseBodyCopier) Write(b []byte) (int, error) {
	if w.start.IsZero() {
		w.start = time.Now()
	}

	// Single write operation that writes to both the response and buffer
	return w.tee.Write(b)
}

func (w *responseBodyCopier) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseBodyCopier) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *responseBodyCopier) StartTime() time.Time {
	return w.start
}

// sensitiveHeaders lists headers that should be redacted in captures
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
}

// redactHeaders replaces sensitive header values in-place with "[REDACTED]"
func redactHeaders(headers map[string]string) {
	for key := range headers {
		if sensitiveHeaders[strings.ToLower(key)] {
			headers[key] = "[REDACTED]"
		}
	}
}

// filterAcceptEncoding filters the Accept-Encoding header to only include
// encodings we can decompress (gzip, deflate). This respects the client's
// preferences while ensuring we can parse response bodies for metrics.
func filterAcceptEncoding(acceptEncoding string) string {
	if acceptEncoding == "" {
		return ""
	}

	supported := map[string]bool{"gzip": true, "deflate": true}
	var filtered []string

	for part := range strings.SplitSeq(acceptEncoding, ",") {
		// Parse encoding and optional quality value (e.g., "gzip;q=1.0")
		encoding, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		if supported[strings.ToLower(encoding)] {
			filtered = append(filtered, strings.TrimSpace(part))
		}
	}

	return strings.Join(filtered, ", ")
}
