package system

import (
    "context"
    "testing"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"

    "solana-rpc-monitor/internal/metrics"
)

func setupTest() (*metrics.Metrics, *Collector) {
    registry := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(registry)
    
    config := struct {
        EnableCPUMetrics     bool
        EnableMemoryMetrics  bool
        EnableDiskMetrics    bool
        EnableNetworkMetrics bool
    }{
        EnableCPUMetrics:     true,
        EnableMemoryMetrics:  true,
        EnableDiskMetrics:    true,
        EnableNetworkMetrics: true,
    }

    labels := map[string]string{
        "node_address": "test-node:8799",
        "org":          "test-org",
        "node_type":    "rpc",
    }

    collector := NewCollector(metrics, config, labels)
    return metrics, collector
}

func TestSystemCollector_CPUMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectCPUMetrics(context.Background())
    assert.NoError(t, err)

    // Verify CPU metrics were collected
    cpuMetric := metrics.CPUUsage.WithLabelValues("test-node:8799", "test-org", "rpc", "cpu0")
    assert.NotNil(t, cpuMetric)
}

func TestSystemCollector_MemoryMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectMemoryMetrics(context.Background())
    assert.NoError(t, err)

    // Verify memory metrics
    labels := []string{"test-node:8799", "test-org", "rpc"}
    
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues(append(labels, "total")...).Get(), float64(0))
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues(append(labels, "used")...).Get(), float64(0))
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues(append(labels, "heap")...).Get(), float64(0))
}

func TestSystemCollector_DiskMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectDiskMetrics(context.Background())
    assert.NoError(t, err)

    // Check if disk metrics were collected
    labels := []string{"test-node:8799", "test-org", "rpc"}
    
    // Get all metric families
    mfs, err := prometheus.DefaultGatherer.Gather()
    assert.NoError(t, err)

    foundDiskMetrics := false
    for _, mf := range mfs {
        if mf.GetName() == "solana_disk_usage_bytes" {
            foundDiskMetrics = true
            break
        }
    }
    assert.True(t, foundDiskMetrics)
}

func TestSystemCollector_NetworkMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectNetworkMetrics(context.Background())
    assert.NoError(t, err)

    // Verify network metrics
    labels := []string{"test-node:8799", "test-org", "rpc"}
    
    foundNetworkMetrics := false
    mfs, err := prometheus.DefaultGatherer.Gather()
    assert.NoError(t, err)

    for _, mf := range mfs {
        if mf.GetName() == "solana_network_io_bytes" {
            foundNetworkMetrics = true
            break
        }
    }
    assert.True(t, foundNetworkMetrics)
}

func TestSystemCollector_DisabledMetrics(t *testing.T) {
    registry := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(registry)
    
    config := struct {
        EnableCPUMetrics     bool
        EnableMemoryMetrics  bool
        EnableDiskMetrics    bool
        EnableNetworkMetrics bool
    }{
        EnableCPUMetrics:     false,
        EnableMemoryMetrics:  false,
        EnableDiskMetrics:    false,
        EnableNetworkMetrics: false,
    }

    labels := map[string]string{
        "node_address": "test-node:8799",
        "org":          "test-org",
        "node_type":    "rpc",
    }

    collector := NewCollector(metrics, config, labels)
    err := collector.Collect(context.Background())
    assert.NoError(t, err)
}

func TestSystemCollector_ConcurrentCollection(t *testing.T) {
    metrics, collector := setupTest()

    // Run multiple collections concurrently
    var errCh = make(chan error, 5)
    for i := 0; i < 5; i++ {
        go func() {
            errCh <- collector.Collect(context.Background())
        }()
    }

    // Check for errors
    for i := 0; i < 5; i++ {
        assert.NoError(t, <-errCh)
    }
}

func TestSystemCollector_ContextCancellation(t *testing.T) {
    _, collector := setupTest()

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context canceled")
}

func TestSystemCollector_Timeout(t *testing.T) {
    _, collector := setupTest()

    ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
    defer cancel()

    time.Sleep(2 * time.Millisecond) // Force timeout
    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context deadline exceeded")
}
