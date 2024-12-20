package system

import (
    "context"
    "testing"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "solana-rpc-monitor/internal/metrics"
)

// Integration test setup with real system metrics
func setupIntegrationTest(t *testing.T) (*metrics.Metrics, *Collector) {
    reg := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(reg)
    config := struct {
        EnableDiskMetrics bool
        EnableCPUMetrics  bool
    }{
        EnableDiskMetrics: true,
        EnableCPUMetrics:  true,
    }
    collector := NewCollector(metrics, config)
    return metrics, collector
}

func TestIntegration_FullMetricCollection(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    metrics, collector := setupIntegrationTest(t)
    ctx := context.Background()

    // Collect metrics multiple times to ensure stability
    for i := 0; i < 3; i++ {
        err := collector.Collect(ctx)
        require.NoError(t, err)
        time.Sleep(time.Second)
    }

    // Verify all metric types are present
    metricFamilies, err := prometheus.DefaultGatherer.Gather()
    require.NoError(t, err)

    expectedMetrics := map[string]bool{
        "solana_cpu_usage_percent":     false,
        "solana_memory_usage_bytes":    false,
        "solana_disk_usage_bytes":      false,
        "solana_disk_iops":            false,
        "solana_network_io_bytes":      false,
    }

    for _, mf := range metricFamilies {
        if _, ok := expectedMetrics[mf.GetName()]; ok {
            expectedMetrics[mf.GetName()] = true
        }
    }

    for metric, found := range expectedMetrics {
        assert.True(t, found, "Expected metric %s was not collected", metric)
    }
}

func TestIntegration_HighLoadScenario(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    metrics, collector := setupIntegrationTest(t)

    // Generate some system load
    go func() {
        generateCPULoad()
        generateMemoryLoad()
        generateDiskLoad()
    }()

    time.Sleep(time.Second) // Wait for load to start

    err := collector.Collect(context.Background())
    require.NoError(t, err)

    // Verify metrics under load
    assert.Greater(t, metrics.CPUUsage.WithLabelValues("cpu0").Get(), float64(10), 
        "CPU usage should be significant under load")
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("used").Get(), float64(1024*1024), 
        "Memory usage should be significant under load")
}

func TestIntegration_LongRunning(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping long-running integration test")
    }

    metrics, collector := setupIntegrationTest(t)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    var collections int
    for {
        select {
        case <-ctx.Done():
            t.Logf("Completed %d collections", collections)
            return
        case <-ticker.C:
            err := collector.Collect(ctx)
            require.NoError(t, err)
            collections++
        }
    }
}
