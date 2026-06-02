# Merge Plan: Upstream `mostlygeek/llama-swap` → Local `main`

## Executive Summary

Upstream has introduced a **massive architectural refactor** (#790 "Introduce new routing backend") that reorganized the codebase from a flat `proxy/` package into a layered `internal/` architecture (`internal/router/`, `internal/process/`, `internal/server/`, `internal/config/`, etc.).

Local has **33 custom commits** building a rich metrics dashboard, persistence layer, live token tracking, capture rendering, and UI enhancements on top of the **old architecture**.

**Result**: A direct merge produces **20 content conflicts + 1 modify/delete conflict**. Core backend files have overlapping but architecturally divergent changes. A line-by-line conflict resolution is impractical for the backend.

**Recommended Strategy**: "Upstream-first Structured Merge" — accept upstream's architecture, resolve mechanical conflicts intelligently, then re-integrate local features in logical groups on top of the new structure.

---

## 1. Divergence Snapshot

| Metric | Local | Upstream |
|--------|-------|----------|
| Commits since divergence | 33 | 44 |
| Files changed | ~47 | ~138 |
| Files in conflict zone | 25 | 25 |
| New packages (local) | `proxy/metrics_store.go`, `proxy/prompt_progress.go`, `proxy/llamacpp_memory.go`, `proxy/persistence_settings.go`, `proxy/specDecodeParser.go`, UI dashboard/settings | — |
| New packages (upstream) | `internal/router/`, `internal/process/`, `internal/server/`, `internal/cache/`, `internal/chain/`, `internal/perf/`, `internal/ring/`, `internal/shared/`, `internal/watcher/` | — |

### Key Upstream Changes to Account For
1. **#790 New Routing Backend** — `proxy/` logic extracted to `internal/router/`, `internal/server/`, `internal/process/`
2. **`proxy/config/` → `internal/config/`** — config package renamed and expanded
3. **Prometheus metrics endpoint** (`internal/perf/prometheus.go`)
4. **Performance monitoring** via `nvidia-smi`/`rocm-smi` on Win/Unix/Darwin (`internal/perf/`)
5. **fsnotify replaced with stat-poll watcher + SIGHUP reload** (`internal/watcher/`)
6. **Versionless API endpoint** (`/v1/models` improvements)
7. **Concurrency middleware JSON payload fix**
8. **Load testing UI** (`ui-svelte/src/routes/Performance.svelte`)
9. **Activity Page refactor** (upstream simplified their activity UI)
10. **`StatsPanel.svelte` deleted upstream** (functionality likely moved to Performance or Activity)

### Local Features to Preserve
1. **Metrics Dashboard** (`ui-svelte/src/routes/Dashboard.svelte` + stats components)
2. **Metrics Persistence** (`proxy/metrics_store.go`, SQLite-based)
3. **Persistence Settings** (`proxy/persistence_settings.go`, YAML↔SQLite sync)
4. **Live Prompt Processing Progress** (`proxy/prompt_progress.go`)
5. **Live Generated Token Tracking** (in `proxy/metrics_monitor.go`)
6. **Llama.cpp Memory Parsing** (`proxy/llamacpp_memory.go`)
7. **Speculative Decoding Stats** (`proxy/specDecodeParser.go`)
8. **Multimodal Detection & Chat Capture Render** (`ui-svelte/src/components/CaptureChatRender.svelte`)
9. **Model Configuration Viewer** (`ui-svelte/src/components/ModelConfigurationDialog.svelte`)
10. **Tool Call Rendering** (in capture dialog)
11. **Activity Capture Downloads** (UI + API)

---

## 2. Conflict Analysis (21 files)

### 🔴 Backend: High Complexity — Accept Upstream, Re-apply Local Features

| File | Local Δ | Upstream Δ | Conflict Nature | Resolution Strategy |
|------|---------|------------|-----------------|---------------------|
| `proxy/proxymanager.go` | +106/-5 | +357/-280 | Upstream refactored core routing; local added live-progress hooks | Accept upstream, re-apply progress hooks in new `internal/router/` or `internal/server/` |
| `proxy/proxymanager_api.go` | +356/-13 | +64/-21 | Local added 350+ lines of dashboard/settings/capture APIs; upstream kept skeleton | Accept upstream skeleton, extract local API additions and port to `internal/server/api.go` or new extension file |
| `proxy/metrics_monitor.go` | +624/-75 | +316/-219 | Both heavily modified. Local added persistence, token tracking, memory stats, speculative decoding | Most complex merge. Use upstream as base, manually weave in local metric-tracking features. Consider extracting local-only additions into a decorator/wrapper |
| `proxy/process.go` | +57/-2 | +9/-8 | Local added memory-tracking hooks; upstream moved most logic to `internal/process/process.go` | Accept upstream, port memory hooks to `internal/process/process.go` |
| `proxy/processgroup.go` | +5/-1 | +6/-5 | Minor local additions; upstream changed shutdown behavior | Accept upstream, re-apply any local group-level hooks |
| `proxy/matrix.go` | +5/-1 | +6/-5 | Local added metrics-skip logic | Accept upstream, re-apply metrics-skip flag |

### 🟡 Config: Renamed Package — Mechanical Resolution

| File | Reason for Conflict | Resolution Strategy |
|------|---------------------|---------------------|
| `internal/config/config.go` | Upstream renamed `proxy/config/` → `internal/config/`; local modified old location | Accept upstream version entirely. Local had no config structs changes, only YAML field additions for persistence. Re-apply persistence fields to new `internal/config/` structs |
| `internal/config/config_posix_test.go` | Same rename | Accept upstream |
| `internal/config/config_windows_test.go` | Same rename | Accept upstream |

### 🟡 Logging: New Upstream File

| File | Reason for Conflict | Resolution Strategy |
|------|---------------------|---------------------|
| `internal/logmon/logging.go` | New file in upstream (`internal/logmon/`); local had changes in old logging location | Accept upstream. Verify local log-parsing regexes (for prompt progress, memory) are applied in right place |

### 🟢 UI Svelte: Moderate Complexity — Preserve Local, Upstream Additions

| File | Local vs Upstream | Resolution Strategy |
|------|-------------------|---------------------|
| `ui-svelte/src/App.svelte` | Both added routes (local: Dashboard, Settings; upstream: Performance) | Merge routes. Keep all three: Dashboard, Settings, Performance |
| `ui-svelte/src/lib/types.ts` | Both added types | Union of both type sets |
| `ui-svelte/src/stores/api.ts` | Both added API functions | Union of both API functions |
| `ui-svelte/src/routes/Activity.svelte` | Local heavily enhanced; upstream refactored | Keep upstream refactor structure, re-apply local enhancements (multimodal rendering, chat preview, tool calls) |
| `ui-svelte/src/routes/Models.svelte` | Local added config viewer button | Accept both |
| `ui-svelte/src/components/CaptureDialog.svelte` | Both enhanced | Weave together: upstream may have added CBOR/zstd; local added chat render + tool calls |
| `ui-svelte/src/components/Header.svelte` | Both added nav items | Merge nav items |
| `ui-svelte/src/components/StatsPanel.svelte` | **Deleted upstream**, modified locally | ⚠️ **Modify/delete conflict**. Upstream removed this component. Local still uses it in Dashboard. **Resolution**: Keep local file for now (it belongs to Dashboard), verify Dashboard still compiles |

### 🟢 Build/Docs: Trivial

| File | Resolution Strategy |
|------|---------------------|
| `go.mod` | Accept both dependency sets (merge module lines) |
| `go.sum` | Regenerate after resolving `go.mod` (`go mod tidy`) |
| `README.md` | Preserve local supercharged branding + screenshots, merge upstream feature docs |
| `config.example.yaml` | Merge example keys from both |
| `config-schema.json` | Merge schema additions from both |
| `docs/configuration.md` | Merge documentation |

---

## 3. Recommended Execution Strategy

### Phase A: Preparation (Safe, Reversible)

1. **Create a merge branch**
   ```bash
   git checkout -b merge/upstream-main main
   ```

2. **Push local `main` to origin** (if not already done)
   ```bash
   git push origin main
   ```

3. **Lock in the upstream merge base**
   ```bash
   git fetch upstream
   git branch -f upstream-base upstream/main
   ```

4. **Identify local-only files that won't conflict**
   These 29 files can be copied/ported after the core merge:
   - `proxy/metrics_store.go`, `proxy/metrics_store_test.go`
   - `proxy/prompt_progress.go`, `proxy/prompt_progress_test.go`
   - `proxy/llamacpp_memory.go`, `proxy/llamacpp_memory_test.go`
   - `proxy/persistence_settings.go`, `proxy/persistence_settings_test.go`
   - `proxy/specDecodeParser.go`, `proxy/specDecodeParser_test.go`
   - `proxy/proxymanager_api_test.go`
   - `ui-svelte/src/routes/Dashboard.svelte`
   - `ui-svelte/src/routes/Settings.svelte`
   - `ui-svelte/src/components/CaptureChatRender.svelte`
   - `ui-svelte/src/components/ModelConfigurationDialog.svelte`
   - `ui-svelte/src/components/stats/*.svelte`
   - `ui-svelte/src/lib/captureChat.ts`, `captureChat.test.ts`
   - `ui-svelte/src/lib/chartPaths.ts`, `chartPaths.test.ts`
   - `ui-svelte/src/lib/llamaServerOptions.ts`, `llamaServerOptions.test.ts`
   - `ui-svelte/src/lib/metricsStats.ts`, `metricsStats.test.ts`
   - `ui-svelte/src/stores/api.test.ts`
   - `scripts/test_stats_dashboard.py`

### Phase B: Mechanical Merge (Commit-By-Commit Conflict Resolution)

Instead of resolving everything at once, resolve in **dependency order**:

#### Step B1: Config & Build Foundation
Resolve `go.mod`/`go.sum` first so the project can compile.
- Accept upstream's module decl + Go version
- Union dependency blocks (keep upstream deps + local additions like SQLite, etc.)
- `go mod tidy`

#### Step B2: Config Package (`internal/config/`)
- Accept upstream's `internal/config/config.go` entirely
- Re-apply local persistence fields:
  - `MetricsRetentionDays`
  - `UsageMetricsPersistence`
  - `ActivityPersistence`
  - `ActivityCapturePersistence`
  - `CaptureRedactHeaders`
  - `LoggingEnabled` (verify upstream doesn't have equivalent)

#### Step B3: Core Backend (`proxy/` → `internal/`)
This is the hardest step. Recommended approach:
1. For `proxy/proxymanager.go`, `proxy/process.go`, `proxy/processgroup.go`, `proxy/matrix.go`
   - Accept upstream versions
   - Immediately open `internal/router/`, `internal/process/`, `internal/server/` to identify where local hooks belonged
2. For `proxy/metrics_monitor.go`
   - Accept upstream version
   - Create a **new file** `internal/server/metrics_extended.go` (or similar) that wraps/extends upstream's metrics with local features (SQLite persistence, speculative decoding, prompt progress, memory tracking)
   - This avoids fighting with upstream's ongoing metrics evolution
3. For `proxy/proxymanager_api.go`
   - Accept upstream version
   - Port local API endpoints to `internal/server/api.go` or a new `internal/server/api_dashboard.go`

#### Step B4: UI Core (`ui-svelte/src/App.svelte`, `types.ts`, `stores/api.ts`, `Header.svelte`)
- Manually merge route lists, types, and store functions
- Keep upstream's new `Performance.svelte` route
- Keep local's `Dashboard.svelte` and `Settings.svelte` routes

#### Step B5: UI Routes & Components
- `Activity.svelte`: Upstream refactored. Apply local chat-render, multimodal, and tool-call features on top of upstream's new structure
- `CaptureDialog.svelte`: Merge both feature sets
- `Models.svelte`: Merge
- `StatsPanel.svelte`: **Keep local version**. It is required by local `Dashboard.svelte`. Upstream deleted it because they replaced its usage. No action needed other than keeping the file.

#### Step B6: Docs & Metadata
- `README.md`: Preserve local branding, merge upstream usage changes
- `config.example.yaml` / `config-schema.json`: Merge keys

### Phase C: Porting Local-Only Backend Files

After the mechanical merge compiles, port the 11 local-only `proxy/*.go` files to work with upstream's architecture.

| Local File | Likely New Home | Porting Notes |
|------------|-----------------|---------------|
| `proxy/metrics_store.go` | `internal/server/metrics_store.go` or `internal/metrics/` | Depends on `internal/config` instead of `proxy/config`. Update imports |
| `proxy/prompt_progress.go` | `internal/logmon/prompt_progress.go` or `internal/process/` | It parses llama-server stderr. Check if upstream's `internal/logmon/` is the right place |
| `proxy/llamacpp_memory.go` | `internal/logmon/memory_parser.go` | Same parsing context |
| `proxy/specDecodeParser.go` | `internal/logmon/spec_decode.go` | Same |
| `proxy/persistence_settings.go` | `internal/config/persistence.go` or `internal/server/persistence.go` | Depends on `ProxyManager`; may need to become a `Server` method or standalone service |
| `proxy/proxymanager_api_test.go` | `internal/server/api_test.go` (port relevant tests) | |

### Phase D: UI Port

The local-only UI files should mostly work as-is because Svelte/UI is less coupled to Go architecture. However:
- Update API paths if upstream changed any endpoint URLs
- Update type imports from `lib/types.ts` (already merged in Phase B4)
- Build the UI: `cd ui-svelte && npm run build` (or `make ui`)

### Phase E: Verification

1. `go test ./...` — fix any broken tests
2. `make test-dev` — run `go test` + `staticcheck` (per project instructions)
3. `make test-all` — full concurrency tests
4. Build and run the binary locally
5. Verify dashboard loads, settings persist, captures render

---

## 4. Risk Register

| Risk | Severity | Mitigation |
|------|----------|------------|
| Upstream `internal/router/` doesn't expose hooks that local `proxymanager.go` relied on | **High** | Audit `internal/router/router.go` before merge. If necessary, fork/wrap the router in `internal/server/` |
| `proxy/metrics_monitor.go` merge is so complex it introduces silent metric bugs | **High** | Extract local features into a separate file instead of merging line-by-line. Add integration tests |
| Upstream's `internal/process/` changed shutdown behavior; local progress trackers may lose final log lines | **Medium** | Port `prompt_progress.go` and `llamacpp_memory.go` carefully. Verify process stdout/stderr piping is equivalent |
| UI type conflicts cause build failures | **Medium** | Build UI frequently during Phase B4–D. Fix TypeScript errors incrementally |
| Local README screenshots and branding lost | **Low** | Handle `README.md` manually; preserve `docs/assets/readme/supercharged-*.png` |

---

## 5. Alternative Approaches (Rejected but Documented)

| Approach | Why Not Recommended |
|----------|---------------------|
| **Pure rebase** (`git rebase upstream/main`) | Local commits are on old architecture. Every commit would need rewriting. 33 commits × architecture mismatch = extreme pain |
| **Cherry-pick upstream onto local** | Same problem inverted. Upstream commits assume new `internal/` packages that don't exist in local base |
| **Accept-ours merge** (`-X ours`) | Would discard upstream's routing backend refactor, Prometheus metrics, perf monitoring, and bugfixes. Unacceptable |
| **Accept-theirs merge** (`-X theirs`) | Would instantly lose all 33 local commits. Fast but loses all supercharged features |

---

## 6. Suggested Next Action

1. Review this plan and confirm the "Upstream-first Structured Merge" strategy
2. I will create the `merge/upstream-main` branch and begin **Step B1** (`go.mod`/`go.sum`) + **Step B2** (`internal/config/`) immediately
3. After config compiles, proceed to the core backend files in **Step B3**
4. Pause after each major step for review/tests

**Estimated effort**: 2–4 hours of focused conflict resolution + 1–2 hours of porting local-only files + 1 hour verification.
