package system

import (
    "context"
    "testing"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"

    "solana-rpc-monitor/internal/metrics"
)

func setupTest() (*metrics.Metrics, *Collector) {
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

func TestCollectCPUMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectCPUMetrics(context.Background())
    assert.NoError(t, err)

    // Check if at least one CPU metric was collected
    found := false
    for i := 0; i < runtime.NumCPU(); i++ {
        value := metrics.CPUUsage.WithLabelValues(fmt.Sprintf("cpu%d", i)).Get()
        if value > 0 {
            found = true
            break
        }
    }
    assert.True(t, found, "No CPU metrics were collected")
}

func TestCollectMemoryMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectMemoryMetrics(context.Background())
    assert.NoError(t, err)

    // Verify memory metrics
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("total").Get(), float64(0))
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("used").Get(), float64(0))
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("heap").Get(), float64(0))
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("stack").Get(), float64(0))
}

func TestCollectDiskMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectDiskMetrics(context.Background())
    assert.NoError(t, err)

    // Check if disk metrics were collected
    foundUsage := false
    foundIOPS := false

    // Get all metric families
    mfs, err := prometheus.DefaultGatherer.Gather()
    assert.NoError(t, err)

    for _, mf := range mfs {
        switch mf.GetName() {
        case "solana_disk_usage_bytes":
            foundUsage = true
            assert.Greater(t, len(mf.GetMetric()), 0, "No disk usage metrics found")
        case "solana_disk_iops":
            foundIOPS = true
            assert.Greater(t, len(mf.GetMetric()), 0, "No disk IOPS metrics found")
        }
    }

    assert.True(t, foundUsage, "Disk usage metrics not found")
    assert.True(t, foundIOPS, "Disk IOPS metrics not found")
}

func TestCollectNetworkMetrics(t *testing.T) {
    metrics, collector := setupTest()

    err := collector.collectNetworkMetrics(context.Background())
    assert.NoError(t, err)

    // Check network metrics
    foundNetworkMetrics := false
    mfs, err := prometheus.DefaultGatherer.Gather()
    assert.NoError(t, err)

    for _, mf := range mfs {
        if mf.GetName() == "solana_network_io_bytes" {
            foundNetworkMetrics = true
            assert.Greater(t, len(mf.GetMetric()), 0, "No network metrics found")
            break
        }
    }

    assert.True(t, foundNetworkMetrics, "Network metrics not found")
}

func TestCollector_Name(t *testing.T) {
    _, collector := setupTest()
    assert.Equal(t, "system", collector.Name())
}

func TestCollector_Collect(t *testing.T) {
    _, collector := setupTest()

    err := collector.Collect(context.Background())
    assert.NoError(t, err)
}

func TestCollector_WithDisabledMetrics(t *testing.T) {
    reg := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(reg)
    config := struct {
        EnableDiskMetrics bool
        EnableCPUMetrics  bool
    }{
        EnableDiskMetrics: false,
        EnableCPUMetrics:  false,
    }
    collector := NewCollector(metrics, config)

    err := collector.Collect(context.Background())
    assert.NoError(t, err)
}

func TestCollector_WithCancelledContext(t *testing.T) {
    _, collector := setupTest()

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context canceled")
}

func TestCollector_WithTimeout(t *testing.T) {
    _, collector := setupTest()

    ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
    defer cancel()

    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestCollector_ConcurrentAccess(t *testing.T) {
    _, collector := setupTest()

    // Test concurrent metric collection
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := collector.Collect(context.Background())
            assert.NoError(t, err)
        }()
    }

    wg.Wait()
}

func BenchmarkCollector_Collect(b *testing.B) {
    _, collector := setupTest()
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        err := collector.Collect(ctx)
        assert.NoError(b, err)
    }
}
