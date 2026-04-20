package proxy

import (
	"io"
	"testing"

	"github.com/mostlygeek/llama-swap/proxy/config"
	"github.com/stretchr/testify/require"
)

func TestLlamaCppMemory_ParsesVulkanLoadLogs(t *testing.T) {
	tracker := newLlamaCppMemoryTracker()

	_, err := tracker.Write([]byte(`load_tensors:   CPU_Mapped model buffer size =     4.94 MiB
load_tensors:      Vulkan0 model buffer size =    12.56 MiB
llama_context: Vulkan_Host  output buffer size =     0.12 MiB
llama_kv_cache:    Vulkan0 KV buffer size =     1.69 MiB
sched_reserve:    Vulkan0 compute buffer size =    16.32 MiB
sched_reserve: Vulkan_Host compute buffer size =     0.43 MiB
`))
	require.NoError(t, err)

	snapshot := tracker.Snapshot()
	require.NotNil(t, snapshot)
	require.Equal(t, llamaCppMemorySource, snapshot.Source)
	require.Len(t, snapshot.Devices, 1)
	require.Len(t, snapshot.Host, 2)

	device := snapshot.Devices[0]
	require.Equal(t, "Vulkan0", device.Name)
	require.Equal(t, bytesFromMiB(12.56), device.ModelBytes)
	require.Equal(t, bytesFromMiB(1.69), device.KVBytes)
	require.Equal(t, bytesFromMiB(16.32), device.ComputeBytes)
	require.Equal(t, device.ModelBytes+device.KVBytes+device.ComputeBytes, device.TrackedBytes)
	require.Equal(t, device.TrackedBytes, snapshot.DeviceTotalBytes)
	require.Equal(t, bytesFromMiB(4.94)+bytesFromMiB(0.12)+bytesFromMiB(0.43), snapshot.HostTotalBytes)
}

func TestLlamaCppMemory_ParsesCudaAndSplitLines(t *testing.T) {
	tracker := newLlamaCppMemoryTracker()

	_, err := tracker.Write([]byte("load_tensors:      CUDA0 model buffer size = 100.50 Mi"))
	require.NoError(t, err)
	require.Nil(t, tracker.Snapshot())

	_, err = tracker.Write([]byte("B\nllama_kv_cache:    CUDA0 KV buffer size = 12.25 MiB\nsched_reserve:    CUDA0 compute buffer size = 33.00 MiB\n"))
	require.NoError(t, err)

	snapshot := tracker.Snapshot()
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Devices, 1)
	require.Equal(t, "CUDA0", snapshot.Devices[0].Name)
	require.Equal(t, bytesFromMiB(100.50), snapshot.Devices[0].ModelBytes)
	require.Equal(t, bytesFromMiB(12.25), snapshot.Devices[0].KVBytes)
	require.Equal(t, bytesFromMiB(33.00), snapshot.Devices[0].ComputeBytes)
}

func TestLlamaCppMemory_ParsesMemoryBreakdownTable(t *testing.T) {
	tracker := newLlamaCppMemoryTracker()

	_, err := tracker.Write([]byte(`llama_memory_breakdown_print: | memory breakdown [MiB]                         |  total     free    self   model   context   compute    unaccounted |
llama_memory_breakdown_print: |   - Vulkan0 (8060S Graphics (RADV STRIX_HALO)) | 131584 = 127877 + (  30 =    12 +       1 +      16) +        3675 |
llama_memory_breakdown_print: |   - Host                                       |                       5 =     4 +       0 +       0                |
`))
	require.NoError(t, err)

	snapshot := tracker.Snapshot()
	require.NotNil(t, snapshot)
	require.Len(t, snapshot.Devices, 1)
	require.Len(t, snapshot.Host, 1)

	device := snapshot.Devices[0]
	require.Equal(t, "Vulkan0 (8060S Graphics (RADV STRIX_HALO))", device.Name)
	require.Equal(t, uint64(131584*1024*1024), device.DeviceCapacityBytes)
	require.Equal(t, uint64(127877*1024*1024), device.DeviceFreeBytes)
	require.Equal(t, uint64(12*1024*1024), device.ModelBytes)
	require.Equal(t, uint64(1*1024*1024), device.KVBytes)
	require.Equal(t, uint64(16*1024*1024), device.ComputeBytes)
	require.NotNil(t, device.UnaccountedBytes)
	require.Equal(t, uint64(3675*1024*1024), *device.UnaccountedBytes)

	host := snapshot.Host[0]
	require.Equal(t, "Host", host.Name)
	require.Equal(t, uint64(4*1024*1024), host.ModelBytes)
}

func TestLlamaCppMemory_ResetClearsSnapshot(t *testing.T) {
	tracker := newLlamaCppMemoryTracker()

	_, err := tracker.Write([]byte("load_tensors:      CUDA0 model buffer size = 100.50 MiB\n"))
	require.NoError(t, err)
	require.NotNil(t, tracker.Snapshot())

	tracker.Reset()
	require.Nil(t, tracker.Snapshot())
}

func TestLlamaCppMemory_IgnoresUnrelatedLogs(t *testing.T) {
	tracker := newLlamaCppMemoryTracker()

	_, err := tracker.Write([]byte("main: server is listening on http://127.0.0.1:8080\n"))
	require.NoError(t, err)
	require.Nil(t, tracker.Snapshot())
}

func TestProxyManager_ModelStatusIncludesMemory(t *testing.T) {
	logger := NewLogMonitorWriter(io.Discard)
	cfg := config.Config{
		Models: map[string]config.ModelConfig{
			"model1": {
				Name:        "Model One",
				Description: "test model",
			},
		},
		Groups: map[string]config.GroupConfig{
			config.DEFAULT_GROUP_ID: {
				Members: []string{"model1"},
			},
		},
	}
	process := NewProcess("model1", 1, cfg.Models["model1"], logger, logger)
	process.forceState(StateReady)
	_, err := process.memoryTracker.Write([]byte("load_tensors:      CUDA0 model buffer size = 100.50 MiB\n"))
	require.NoError(t, err)

	pm := &ProxyManager{
		config: cfg,
		processGroups: map[string]*ProcessGroup{
			config.DEFAULT_GROUP_ID: {
				id:        config.DEFAULT_GROUP_ID,
				config:    cfg,
				processes: map[string]*Process{"model1": process},
			},
		},
	}

	models := pm.getModelStatus()
	require.Len(t, models, 1)
	require.Equal(t, "model1", models[0].Id)
	require.NotNil(t, models[0].Memory)
	require.Equal(t, bytesFromMiB(100.50), models[0].Memory.DeviceTotalBytes)
}

func bytesFromMiB(value float64) uint64 {
	return uint64(value*1024*1024 + 0.5)
}
