package system

import (
    "context"
    "fmt"
    "runtime"
    "sync"
    "time"

    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/disk"
    "github.com/shirou/gopsutil/v3/mem"
    "github.com/shirou/gopsutil/v3/net"

    "solana-rpc-monitor/internal/metrics"
)

type Collector struct {
    metrics *metrics.Metrics
    config  struct {
        EnableDiskMetrics bool
        EnableCPUMetrics  bool
    }
}

func NewCollector(metrics *metrics.Metrics, config struct {
    EnableDiskMetrics bool
    EnableCPUMetrics  bool
}) *Collector {
    return &Collector{
        metrics: metrics,
        config:  config,
    }
}

func (c *Collector) Name() string {
    return "system"
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 4)

    // Collect CPU metrics
    if c.config.EnableCPUMetrics {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := c.collectCPUMetrics(ctx); err != nil {
                errCh <- fmt.Errorf("cpu metrics: %w", err)
            }
        }()
    }

    // Collect memory metrics
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectMemoryMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("memory metrics: %w", err)
        }
    }()

    // Collect disk metrics
    if c.config.EnableDiskMetrics {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := c.collectDiskMetrics(ctx); err != nil {
                errCh <- fmt.Errorf("disk metrics: %w", err)
            }
        }()
    }

    // Collect network metrics
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectNetworkMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("network metrics: %w", err)
        }
    }()

    // Wait for all collectors
    wg.Wait()
    close(errCh)

    // Check for any errors
    for err := range errCh {
        if err != nil {
            return err
        }
    }

    return nil
}

func (c *Collector) collectCPUMetrics(ctx context.Context) error {
    percentages, err := cpu.PercentWithContext(ctx, time.Second, true)
    if err != nil {
        return err
    }

    for i, percentage := range percentages {
        c.metrics.CPUUsage.WithLabelValues(fmt.Sprintf("cpu%d", i)).Set(percentage)
    }

    return nil
}

func (c *Collector) collectMemoryMetrics(ctx context.Context) error {
    vmStat, err := mem.VirtualMemoryWithContext(ctx)
    if err != nil {
        return err
    }

    c.metrics.MemoryUsage.WithLabelValues("total").Set(float64(vmStat.Total))
    c.metrics.MemoryUsage.WithLabelValues("used").Set(float64(vmStat.Used))
    c.metrics.MemoryUsage.WithLabelValues("free").Set(float64(vmStat.Free))
    c.metrics.MemoryUsage.WithLabelValues("cached").Set(float64(vmStat.Cached))

    // Runtime memory stats
    var runtimeStats runtime.MemStats
    runtime.ReadMemStats(&runtimeStats)
    c.metrics.MemoryUsage.WithLabelValues("heap").Set(float64(runtimeStats.HeapAlloc))
    c.metrics.MemoryUsage.WithLabelValues("stack").Set(float64(runtimeStats.StackInuse))

    return nil
}

func (c *Collector) collectDiskMetrics(ctx context.Context) error {
    partitions, err := disk.PartitionsWithContext(ctx, false)
    if err != nil {
        return err
    }

    for _, partition := range partitions {
        usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
        if err != nil {
            continue
        }

        labels := []string{partition.Mountpoint}
        c.metrics.DiskUsage.WithLabelValues(append(labels, "total")...).Set(float64(usage.Total))
        c.metrics.DiskUsage.WithLabelValues(append(labels, "used")...).Set(float64(usage.Used))
        c.metrics.DiskUsage.WithLabelValues(append(labels, "free")...).Set(float64(usage.Free))
    }

    // Collect IO stats
    ioStats, err := disk.IOCountersWithContext(ctx)
    if err != nil {
        return err
    }

    for device, stats := range ioStats {
        c.metrics.DiskIOPS.WithLabelValues(device, "read").Set(float64(stats.ReadCount))
        c.metrics.DiskIOPS.WithLabelValues(device, "write").Set(float64(stats.WriteCount))
    }

    return nil
}

func (c *Collector) collectNetworkMetrics(ctx context.Context) error {
    netStats, err := net.IOCountersWithContext(ctx, true)
    if err != nil {
        return err
    }

    for _, stat := range netStats {
        c.metrics.NetworkIO.WithLabelValues(stat.Name, "bytes_sent").Add(float64(stat.BytesSent))
        c.metrics.NetworkIO.WithLabelValues(stat.Name, "bytes_recv").Add(float64(stat.BytesRecv))
        c.metrics.NetworkIO.WithLabelValues(stat.Name, "packets_sent").Add(float64(stat.PacketsSent))
        c.metrics.NetworkIO.WithLabelValues(stat.Name, "packets_recv").Add(float64(stat.PacketsRecv))
    }

    return nil
}
