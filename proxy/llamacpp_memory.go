package proxy

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const llamaCppMemorySource = "llamacpp_logs"

var (
	llamaCppBufferSizeRe      = regexp.MustCompile(`^[^:]+:\s+(.+?)\s+(model|KV|output|compute) buffer size =\s+([0-9]+(?:\.[0-9]+)?)\s+([KMGT]i?B|[KMGT]B|B)\b`)
	llamaCppDeviceBreakdownRe = regexp.MustCompile(`^llama_memory_breakdown_print:\s+\|\s+-\s+(.+?)\s+\|\s+([0-9]+)\s*=\s*([0-9]+)\s*\+\s*\(\s*([0-9]+)\s*=\s*([0-9]+)\s*\+\s*([0-9]+)\s*\+\s*([0-9]+)\s*\)\s*\+\s*([0-9]+)\s*\|`)
	llamaCppOtherBreakdownRe  = regexp.MustCompile(`^llama_memory_breakdown_print:\s+\|\s+-\s+(.+?)\s+\|\s+([0-9]+)\s*=\s*([0-9]+)\s*\+\s*([0-9]+)\s*\+\s*([0-9]+)\s*\|`)
)

type LlamaCppMemorySnapshot struct {
	Source            string                    `json:"source"`
	UpdatedAt         time.Time                 `json:"updated_at"`
	DeviceTotalBytes  uint64                    `json:"device_total_bytes"`
	HostTotalBytes    uint64                    `json:"host_total_bytes"`
	TotalTrackedBytes uint64                    `json:"total_tracked_bytes"`
	Devices           []LlamaCppMemoryComponent `json:"devices,omitempty"`
	Host              []LlamaCppMemoryComponent `json:"host,omitempty"`
	Unknown           []LlamaCppMemoryComponent `json:"unknown,omitempty"`
}

type LlamaCppMemoryComponent struct {
	Name                string  `json:"name"`
	ModelBytes          uint64  `json:"model_bytes,omitempty"`
	KVBytes             uint64  `json:"kv_bytes,omitempty"`
	ComputeBytes        uint64  `json:"compute_bytes,omitempty"`
	OutputBytes         uint64  `json:"output_bytes,omitempty"`
	TrackedBytes        uint64  `json:"tracked_bytes"`
	DeviceCapacityBytes uint64  `json:"device_capacity_bytes,omitempty"`
	DeviceFreeBytes     uint64  `json:"device_free_bytes,omitempty"`
	UnaccountedBytes    *uint64 `json:"unaccounted_bytes,omitempty"`
}

type llamaCppMemoryTracker struct {
	mu         sync.RWMutex
	partial    string
	components map[string]*LlamaCppMemoryComponent
	updatedAt  time.Time
	hasData    bool
}

func newLlamaCppMemoryTracker() *llamaCppMemoryTracker {
	return &llamaCppMemoryTracker{
		components: make(map[string]*LlamaCppMemoryComponent),
	}
}

func (t *llamaCppMemoryTracker) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	data := t.partial + string(p)
	lines := strings.Split(data, "\n")
	t.partial = lines[len(lines)-1]
	if len(t.partial) > 64*1024 {
		t.partial = t.partial[len(t.partial)-64*1024:]
	}

	for _, line := range lines[:len(lines)-1] {
		t.parseLine(strings.TrimRight(line, "\r"))
	}

	return len(p), nil
}

func (t *llamaCppMemoryTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.partial = ""
	t.components = make(map[string]*LlamaCppMemoryComponent)
	t.updatedAt = time.Time{}
	t.hasData = false
}

func (t *llamaCppMemoryTracker) Snapshot() *LlamaCppMemorySnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.hasData {
		return nil
	}

	snapshot := &LlamaCppMemorySnapshot{
		Source:    llamaCppMemorySource,
		UpdatedAt: t.updatedAt,
	}

	for _, component := range t.components {
		copied := *component
		copied.TrackedBytes = copied.ModelBytes + copied.KVBytes + copied.ComputeBytes + copied.OutputBytes
		if component.UnaccountedBytes != nil {
			unaccounted := *component.UnaccountedBytes
			copied.UnaccountedBytes = &unaccounted
		}

		switch classifyLlamaCppMemoryComponent(copied.Name) {
		case "device":
			snapshot.DeviceTotalBytes += copied.TrackedBytes
			snapshot.Devices = append(snapshot.Devices, copied)
		case "host":
			snapshot.HostTotalBytes += copied.TrackedBytes
			snapshot.Host = append(snapshot.Host, copied)
		default:
			snapshot.Unknown = append(snapshot.Unknown, copied)
		}
		snapshot.TotalTrackedBytes += copied.TrackedBytes
	}
	sort.Slice(snapshot.Devices, func(i, j int) bool { return snapshot.Devices[i].Name < snapshot.Devices[j].Name })
	sort.Slice(snapshot.Host, func(i, j int) bool { return snapshot.Host[i].Name < snapshot.Host[j].Name })
	sort.Slice(snapshot.Unknown, func(i, j int) bool { return snapshot.Unknown[i].Name < snapshot.Unknown[j].Name })
	return snapshot
}

func (t *llamaCppMemoryTracker) parseLine(line string) {
	if matches := llamaCppBufferSizeRe.FindStringSubmatch(line); matches != nil {
		name := strings.TrimSpace(matches[1])
		bytes, ok := parseLlamaCppMemoryBytes(matches[3], matches[4])
		if !ok {
			return
		}
		component := t.component(name)
		switch matches[2] {
		case "model":
			component.ModelBytes = bytes
		case "KV":
			component.KVBytes = bytes
		case "output":
			component.OutputBytes = bytes
		case "compute":
			component.ComputeBytes = bytes
		}
		t.markUpdated()
		return
	}

	if matches := llamaCppDeviceBreakdownRe.FindStringSubmatch(line); matches != nil {
		name := strings.TrimSpace(matches[1])
		component := t.component(name)
		component.DeviceCapacityBytes = parseMiB(matches[2])
		component.DeviceFreeBytes = parseMiB(matches[3])
		if component.ModelBytes == 0 {
			component.ModelBytes = parseMiB(matches[5])
		}
		if component.KVBytes == 0 {
			component.KVBytes = parseMiB(matches[6])
		}
		if component.ComputeBytes == 0 {
			component.ComputeBytes = parseMiB(matches[7])
		}
		unaccounted := parseMiB(matches[8])
		component.UnaccountedBytes = &unaccounted
		t.markUpdated()
		return
	}

	if matches := llamaCppOtherBreakdownRe.FindStringSubmatch(line); matches != nil {
		name := strings.TrimSpace(matches[1])
		component := t.component(name)
		if component.ModelBytes == 0 {
			component.ModelBytes = parseMiB(matches[3])
		}
		if component.KVBytes == 0 {
			component.KVBytes = parseMiB(matches[4])
		}
		if component.ComputeBytes == 0 {
			component.ComputeBytes = parseMiB(matches[5])
		}
		t.markUpdated()
	}
}

func (t *llamaCppMemoryTracker) component(name string) *LlamaCppMemoryComponent {
	if component, ok := t.components[name]; ok {
		return component
	}
	component := &LlamaCppMemoryComponent{Name: name}
	t.components[name] = component
	return component
}

func (t *llamaCppMemoryTracker) markUpdated() {
	t.updatedAt = time.Now()
	t.hasData = true
}

func parseLlamaCppMemoryBytes(value, unit string) (uint64, bool) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}

	multiplier := float64(1)
	switch strings.ToLower(unit) {
	case "kib":
		multiplier = 1024
	case "kb":
		multiplier = 1000
	case "mib":
		multiplier = 1024 * 1024
	case "mb":
		multiplier = 1000 * 1000
	case "gib":
		multiplier = 1024 * 1024 * 1024
	case "gb":
		multiplier = 1000 * 1000 * 1000
	case "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "tb":
		multiplier = 1000 * 1000 * 1000 * 1000
	case "b":
		multiplier = 1
	default:
		return 0, false
	}
	return uint64(parsed*multiplier + 0.5), true
}

func parseMiB(value string) uint64 {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed * 1024 * 1024
}

func classifyLlamaCppMemoryComponent(name string) string {
	normalized := strings.TrimSpace(name)
	if normalized == "Host" || strings.HasPrefix(normalized, "CPU") || strings.Contains(normalized, "_Host") || strings.Contains(normalized, "_Mapped") {
		return "host"
	}

	for _, prefix := range []string{"CUDA", "ROCm", "Vulkan", "SYCL", "MUSA", "CANN", "Metal"} {
		if strings.HasPrefix(normalized, prefix) {
			return "device"
		}
	}
	return "unknown"
}
