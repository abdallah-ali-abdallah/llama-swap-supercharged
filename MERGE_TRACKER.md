# Merge Tracker: Upstream mostlygeek/llama-swap → Local main

Strategy: "Upstream-first Structured Merge"
- Accept upstream's architecture and refactored `proxy/` files
- Re-apply local features on top of upstream's refactored `proxy/`
- Get compiling and passing tests after each phase

---

## Phase 1: Mechanical Foundation ✅ COMPLETE
- [x] Create merge branch `merge/upstream-main`
- [x] Resolve all 22 merge conflicts (21 content + 1 modify/delete)
- [x] go.mod/go.sum: accept upstream Go 1.26.1, add `modernc.org/sqlite`, `go mod tidy`
- [x] `internal/config/*`: merge persistence fields into upstream Config struct
- [x] `llama-swap.go`: accept upstream stdlib HTTP + internal/server architecture
- [x] New `internal/*` packages present and compiling
- [x] Delete `proxy/config/` remnants, fix all import paths
- [x] `go build ./...` succeeds

## Phase 2: proxy/ Conflicts ✅ COMPLETE
- [x] Accept upstream proxymanager.go, proxymanager_api.go, metrics_monitor.go
- [x] Resolve process.go, processgroup.go, matrix.go conflicts (keep local tracking fields)
- [x] `go test ./proxy/...` passes

## Phase 3: Local-Only Backend Files ✅ MOSTLY COMPLETE

### Cancel Registry
- [x] Move to `internal/server/cancel_registry.go`
- [x] Add cancel registry to Server struct
- [x] Add POST /api/activity/live/{id}/cancel endpoint

### Metrics Persistence Store
- [x] Create `internal/server/metrics_store.go` aligned with upstream `ActivityLogEntry`
- [x] Wire persist into `metricsMonitor.queueMetrics` and `addCapture`
- [x] Add `openMetricsStore` helper, initialize in `Server.New()`
- [x] `go test ./...` passes

### Persistence Settings API
- [x] Add `getSettings`/`updateSettings` to metricsStore
- [x] Create `internal/server/api_persistence.go`
- [x] Register GET/POST `/api/settings/persistence` routes

### Live Activity Tracker + SSE Token Stream
- [x] Create `internal/server/live_activity.go` with tracker, tokenStream, LiveActivityRow
- [x] Add GET /api/activity/live/{id}/stream SSE endpoint
- [x] Add LiveActivityEvent emission in api events stream
- [x] Wire `CreateLiveActivityMiddleware` into modelChain
- [x] Add live activity panel + Stream button to Activity.svelte

### Prompt Progress / Generation Token Parsers
- [x] Create `internal/server/log_parsers.go` with parsers extracted from proxy/prompt_progress.go
- [x] Add `parserRegistry` that lazily wires parsers to process loggers per model
- [x] Tracker receives real-time PP progress and generated token counts

### Log Parsers (Memory + Spec Decode)
- [x] Port `llamaCppMemoryTracker` to `internal/server/log_parsers.go`
- [x] Port `specDecodeParser` to `internal/server/log_parsers.go`
- [x] Wire both into `logParserBundle` and `parserRegistry`
- [x] Add `MemorySnapshot`, `SpecDecodeRate`, `SpecDecodeAccepted`, `SpecDecodeGenerated` to `LiveActivityRow`
- [x] Add `SetMemorySnapshot` and `SetSpecDecodeStats` to `liveActivityTracker`
- [x] Delete obsolete `proxy/specDecodeParser.go` + `proxy/specDecodeParser_test.go`
- [x] Go tests pass, staticcheck clean on local code

## Phase 4: UI Merge ✅ COMPLETE
- [x] `types.ts`: union Metrics + ActivityLogEntry + TokenMetrics + live activity types
- [x] `stores/api.ts`: union APIs; add activityLog store, conversion function
- [x] `App.svelte`: merge routes (Dashboard + Settings + Performance)
- [x] `Header.svelte`: merge nav items (Settings + Performance + theme toggle)
- [x] `Activity.svelte`: add live activity panel + Stream button + LiveStreamDialog
- [x] `Models.svelte`: accept upstream ResizablePanels layout
- [x] `CaptureDialog.svelte`: merged (upstream added SSE chat parsing)
- [x] `StatsPanel.svelte`: kept local (Dashboard dependency)
- [x] Dashboard, Settings, LiveStreamDialog, CaptureChatRender present
- [x] `npm run build` passes

## Phase 5: Full Verification ✅ COMPLETE
- [x] `go test ./...` — all Go tests pass
- [x] `make test-dev` — staticcheck passes (minor upstream warnings only)
- [x] `make test-all` — concurrency + race tests pass
- [x] UI builds without errors
- [x] `go fmt -l .` clean
- [x] Binary builds and starts successfully
- [x] `/api/version` responds correctly
- [x] Metrics persistence initializes on startup

### Runtime verification (completed 2026-06-06):
- [x] Dashboard loads with historical metrics from SQLite
- [x] Settings persist to SQLite (settings API responds correctly)
- [x] Live token stream endpoint available (requires live request for full test)
- [x] Request cancellation endpoint available (requires live request for full test)
- [x] Prompt processing progress wired via log parsers
- [x] Upstream Prometheus metrics endpoint responds
- [x] Upstream load test UI works (Performance page available)
- [x] Service restarted and running stably
- [x] UI builds and loads correctly
- [x] API endpoints responding correctly

## Phase 6: Cleanup ✅ COMPLETE
- [x] Remove all `proxy/*.bak` test files
- [x] Remove `proxy/persistence_types.go`
- [x] Remove `proxy/compatibility.go` (LogMonitor shim no longer needed)
- [x] Remove `proxy/specDecodeParser.go` + `proxy/specDecodeParser_test.go` (ported to internal/server)
- [x] Remove unused `metricsQuery`, `newMetricsStore`, `queryMetrics`, `getCapture`, `yamlConflicts`, `persistenceConflict` from `internal/server/metrics_store.go`
- [x] Remove unused `tokenStream.append` from `internal/server/live_activity.go`
- [x] Remove unused `injectTestHandlers` from `proxy/helpers_test.go`
- [x] Remove unused `removeWaiter` from `proxy/prompt_progress.go`
- [x] `go fmt -l .` clean
- [x] Commit merge branch with clean history

---

## Branch Status

Branch: `merge/upstream-main`
Commits: 11 merge commits on top of upstream main

### Changed files summary
- `go.mod`, `go.sum`: updated deps, added sqlite
- `internal/config/config.go`: merged persistence fields
- `internal/server/*`: major additions (metrics_store, api_persistence, live_activity, log_parsers, cancel_registry)
- `internal/server/server.go`: wired new features into Server lifecycle
- `internal/server/apigroup.go`: added LiveActivityEvent to event stream
- `internal/server/metrics.go`: added store persistence hooks
- `proxy/compatibility.go`: LogMonitor shim for legacy code
- `ui-svelte/src/*`: merged routes, added live activity UI, updated stores/types
- `llama-swap.go`: auto-merged upstream version
- `MERGE_TRACKER.md`: this file

### Files still present but currently unused
- `proxy/prompt_progress.go` — logic moved to `internal/server/log_parsers.go`; kept for legacy `proxy/` package compilation
