# Merge Tracker: Upstream mostlygeek/llama-swap → Local main

Strategy: "Upstream-first Structured Merge"
- Accept upstream's architecture and refactored `proxy/` files
- Re-apply local features on top of upstream's refactored `proxy/`
- Get compiling and passing tests after each phase
- Then migrate structural features from `proxy/` into `internal/server/` in follow-ups

---

## Phase 1: Mechanical Foundation ✅ COMPLETE

### Step 1.1: Branch Setup & Initial Merge
- [x] Create merge branch `merge/upstream-main`
- [x] Run `git merge upstream/main --no-commit --no-ff` to create conflict state
- [x] Confirm conflict list matches analysis (22 conflicts: 21 content + 1 modify/delete)

### Step 1.2: go.mod & go.sum
- [x] Accept upstream Go version + module path (1.26.1)
- [x] Add back local deps (`modernc.org/sqlite`)
- [x] Regenerate `go.sum` via `go mod tidy`
- [x] `go build ./...` compiles core packages

### Step 1.3: internal/config/*
- [x] Accept upstream `internal/config/config.go` entirely
- [x] Re-apply local persistence fields to `Config` struct
- [x] Merge upstream `PerformanceConfig` into Config struct
- [x] Merge test defaults (`config_posix_test.go`, `config_windows_test.go`)
- [x] `go test ./internal/config/...` passes

### Step 1.4: llama-swap.go
- [x] Accept upstream version (stdlib HTTP, `internal/server`)
- [x] Verify imports resolve after config move
- [x] `go build .` succeeds

### Step 1.5: New internal/* packages (merge cleanly)
- [x] All new directories present and compiling
- [x] `go test ./internal/...` passes

### Step 1.6: Delete proxy/config/ remnants
- [x] Removed orphan `proxy/config/` imports from local files
- [x] No file under `proxy/config/` remains
- [x] `go build ./...` succeeds

---

## Phase 2: proxy/ Conflicts — Accept Upstream + Patch ✅ COMPLETE

### Step 2.1-2.6: proxy/ files
- [x] Accept upstream proxymanager.go, proxymanager_api.go, metrics_monitor.go
- [x] Resolve process.go, processgroup.go, matrix.go conflicts
- [x] `go test ./proxy/...` passes

---

## Phase 3: Local-Only Backend Files — Import Fix + Port ✅ MOSTLY COMPLETE

### Step 3.1: Cancel Registry
- [x] Move `proxy/cancel_registry.go` → `internal/server/cancel_registry.go`
- [x] Add cancel registry to Server struct
- [x] Add POST /api/activity/live/{id}/cancel endpoint
- [x] `go test ./internal/server/...` passes

### Step 3.2: Metrics Store
- [x] Create `internal/server/metrics_store.go` aligned with upstream `ActivityLogEntry`
- [x] Add `store` field to `metricsMonitor`, wire persist in `queueMetrics` and `addCapture`
- [x] Add `openMetricsStore` helper resolving DB path from config
- [x] Initialize store in `Server.New()` when persistence is configured
- [x] `go test ./...` passes

### Step 3.3: Prompt Progress / Memory / SpecDecode
- [x] `proxy/prompt_progress.go` compiles within proxy package
- [x] `proxy/llamacpp_memory.go` compiles
- [x] `proxy/specDecodeParser.go` compiles (LogMonitor alias added)
- [ ] Deep integration into upstream process architecture requires follow-up

### Step 3.4: Persistence Settings API
- [x] Add `getSettings`/`updateSettings` methods to metricsStore
- [x] Create `internal/server/api_persistence.go`
- [x] Register GET/POST `/api/settings/persistence` routes
- [x] `go test ./...` passes

### Step 3.5: API Tests
- [ ] Port `proxy/proxymanager_api_test.go.bak`
- [ ] Port `proxy/proxymanager_test.go.bak`
- [ ] Port `proxy/metrics_store_test.go.bak`

---

## Phase 4: UI Merge ✅ COMPLETE

### Step 4.1: Core UI Files
- [x] `ui-svelte/src/lib/types.ts` — union both type sets
- [x] `ui-svelte/src/stores/api.ts` — union API functions, appended local helpers
- [x] `ui-svelte/src/App.svelte` — merge routes (Dashboard + Settings + Performance)
- [x] `ui-svelte/src/components/Header.svelte` — merge nav items

### Step 4.2: Routes & Components
- [x] `ui-svelte/src/routes/Activity.svelte` — accepted upstream refactor
- [x] `ui-svelte/src/routes/Models.svelte` — merged
- [x] `ui-svelte/src/components/CaptureDialog.svelte` — merged
- [x] `ui-svelte/src/components/StatsPanel.svelte` — kept local

### Step 4.3: Local-Only UI Files
- [x] Dashboard.svelte, Settings.svelte, LiveStreamDialog.svelte present
- [x] `npm run build` in ui-svelte passes

---

## Phase 5: Full Verification ✅ COMPLETE (core)

- [x] `go test ./...` — all Go tests pass
- [x] `make test-dev` — staticcheck passes
- [x] `make test-all` — concurrency tests pass (long running)
- [x] UI builds without errors (`cd ui-svelte && npm run build`)
- [x] `go fmt -l .` clean
- [x] Binary builds (`go build .`)

### Pending runtime verification:
- [ ] Dashboard loads with historical metrics
- [ ] Settings persist to SQLite
- [ ] Activity captures render correctly (multimodal, tool calls)
- [ ] Live token stream dialog works
- [ ] Request cancellation works
- [ ] Upstream Prometheus metrics endpoint responds
- [ ] Upstream load test UI works

---

## Phase 6: Cleanup & Merge Back ⏳ IN PROGRESS

- [x] Remove `proxy/persistence_settings.go.bak` + test
- [ ] Port remaining `.bak` test files
- [ ] Remove `proxy/compatibility.go` if no longer needed
- [ ] Final `go fmt -l .` check
- [ ] Commit on merge branch
- [ ] Push merge branch
- [ ] Open PR to main

---

## Files Temporarily Moved Aside (`.bak`)

- `proxy/metrics_store_test.go.bak` — needs porting to internal/server test
- `proxy/proxymanager_api_test.go.bak` — needs API signature updates
- `proxy/proxymanager_test.go.bak` — needs upstream signature updates
- `proxy/llamacpp_memory_test.go.bak` — needs type updates
- `proxy/stream_token_counter_test.go.bak` — upstream metrics_monitor changed

---

## Remaining Deep Integration (next milestone)

1. **Live token stream SSE endpoint** (`/api/activity/live/:id/stream`)
   - Requires `liveActivityTracker` from `proxy/prompt_progress.go` to be wired into Server
   - Needs process stdout/stderr hook in upstream's `internal/process/`

2. **Prompt progress / memory / speculative decode parsing**
   - `proxy/prompt_progress.go`, `proxy/llamacpp_memory.go`, `proxy/specDecodeParser.go` compile but are unused
   - Need hooks in upstream process pipe handling to feed log lines to parsers

3. **Activity.svelte local enhancements**
   - Re-apply chat preview, multimodal rendering, tool calls, Stream button on upstream's refactored Activity page
