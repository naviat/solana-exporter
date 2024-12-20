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
    metrics    *metrics.Metrics
    nodeLabels map[string]string
    config     struct {
        EnableCPUMetrics     bool
        EnableMemoryMetrics  bool
        EnableDiskMetrics    bool
        EnableNetworkMetrics bool
    }
}

func NewCollector(metrics *metrics.Metrics, config struct {
    EnableCPUMetrics     bool
    EnableMemoryMetrics  bool
    EnableDiskMetrics    bool
    EnableNetworkMetrics bool
}, labels map[string]string) *Collector {
    return &Collector{
        metrics:    metrics,
        nodeLabels: labels,
        config:     config,
    }
}

func (c *Collector) Name() string {
    return "system"
}

func (c *Collector) getLabels(extraLabels ...string) []string {
    baseLabels := []string{
        c.nodeLabels["node_address"],
        c.nodeLabels["org"],
        c.nodeLabels["node_type"],
    }
    return append(baseLabels, extraLabels...)
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
                errCh <- fmt.Errorf("CPU metrics collection failed: %w", err)
            }
        }()
    }

    // Collect memory metrics
    if c.config.EnableMemoryMetrics {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := c.collectMemoryMetrics(ctx); err != nil {
                errCh <- fmt.Errorf("memory metrics collection failed: %w", err)
            }
        }()
    }

    // Collect disk metrics
    if c.config.EnableDiskMetrics {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := c.collectDiskMetrics(ctx); err != nil {
                errCh <- fmt.Errorf("disk metrics collection failed: %w", err)
            }
        }()
    }

    // Collect network metrics
    if c.config.EnableNetworkMetrics {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := c.collectNetworkMetrics(ctx); err != nil {
                errCh <- fmt.Errorf("network metrics collection failed: %w", err)
            }
        }()
    }

    wg.Wait()
    close(errCh)

    // Check for errors
    var errs []error
    for err := range errCh {
        if err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("multiple collection errors: %v", errs)
    }

    return nil
}

func (c *Collector) collectCPUMetrics(ctx context.Context) error {
    percentages, err := cpu.PercentWithContext(ctx, time.Second, true)
    if err != nil {
        return err
    }

    for i, percentage := range percentages {
        c.metrics.CPUUsage.WithLabelValues(
            append(c.getLabels(), fmt.Sprintf("cpu%d", i))...,
        ).Set(percentage)
    }

    return nil
}

func (c *Collector) collectMemoryMetrics(ctx context.Context) error {
    vmStat, err := mem.VirtualMemoryWithContext(ctx)
    if err != nil {
        return err
    }

    labels := c.getLabels()
    c.metrics.MemoryUsage.WithLabelValues(append(labels, "total")...).Set(float64(vmStat.Total))
    c.metrics.MemoryUsage.WithLabelValues(append(labels, "used")...).Set(float64(vmStat.Used))
    c.metrics.MemoryUsage.WithLabelValues(append(labels, "free")...).Set(float64(vmStat.Free))
    c.metrics.MemoryUsage.WithLabelValues(append(labels, "cached")...).Set(float64(vmStat.Cached))

    // Runtime memory stats
    var runtimeStats runtime.MemStats
    runtime.ReadMemStats(&runtimeStats)
    c.metrics.MemoryUsage.WithLabelValues(append(labels, "heap")...).Set(float64(runtimeStats.HeapAlloc))
    c.metrics.MemoryUsage.WithLabelValues(append(labels, "stack")...).Set(float64(runtimeStats.StackInuse))

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

        labels := append(c.getLabels(), partition.Mountpoint)
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
        labels := append(c.getLabels(), device)
        c.metrics.DiskIOPS.WithLabelValues(append(labels, "read")...).Set(float64(stats.ReadCount))
        c.metrics.DiskIOPS.WithLabelValues(append(labels, "write")...).Set(float64(stats.WriteCount))
    }

    return nil
}

func (c *Collector) collectNetworkMetrics(ctx context.Context) error {
    netStats, err := net.IOCountersWithContext(ctx, true)
    if err != nil {
        return err
    }

    for _, stat := range netStats {
        labels := c.getLabels()
        c.metrics.NetworkIO.WithLabelValues(append(labels, stat.Name, "sent")...).Add(float64(stat.BytesSent))
        c.metrics.NetworkIO.WithLabelValues(append(labels, stat.Name, "received")...).Add(float64(stat.BytesRecv))
    }

    return nil
}
