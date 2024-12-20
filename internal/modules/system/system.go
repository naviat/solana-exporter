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
    "github.com/shirou/gopsutil/v3/process"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/internal/modules/common"
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

func (c *Collector) getBaseLabels() []string {
    return []string{c.nodeLabels["node_address"]}
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 6)

    collectors := []struct {
        enabled bool
        name    string
        collect func(context.Context) error
    }{
        {c.config.EnableCPUMetrics, "CPU", c.collectCPUMetrics},
        {c.config.EnableMemoryMetrics, "Memory", c.collectMemoryMetrics},
        {c.config.EnableDiskMetrics, "Disk", c.collectDiskMetrics},
        {c.config.EnableNetworkMetrics, "Network", c.collectNetworkMetrics},
        {true, "Advanced", c.collectAdvancedSystemMetrics},  // Always enabled
        {true, "IO", c.collectIOMetrics},                    // Always enabled
    }

    for _, collector := range collectors {
        if collector.enabled {
            wg.Add(1)
            go func(name string, collect func(context.Context) error) {
                defer wg.Done()
                if err := collect(ctx); err != nil {
                    errCh <- fmt.Errorf("%s metrics: %w", name, err)
                }
            }(collector.name, collector.collect)
        }
    }

    wg.Wait()
    close(errCh)

    return common.HandleErrors(errCh)
}

func (c *Collector) collectCPUMetrics(ctx context.Context) error {
    percentages, err := cpu.PercentWithContext(ctx, time.Second, true)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()
    for i, percentage := range percentages {
        c.metrics.CPUUsage.WithLabelValues(
            append(baseLabels, fmt.Sprintf("cpu%d", i))...,
        ).Set(percentage)
    }

    return nil
}

func (c *Collector) collectMemoryMetrics(ctx context.Context) error {
    vmStat, err := mem.VirtualMemoryWithContext(ctx)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "total")...).Set(float64(vmStat.Total))
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "used")...).Set(float64(vmStat.Used))
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "free")...).Set(float64(vmStat.Free))
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "cached")...).Set(float64(vmStat.Cached))

    // Runtime memory stats
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "heap")...).Set(float64(m.HeapAlloc))
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "stack")...).Set(float64(m.StackInuse))

    return nil
}

func (c *Collector) collectDiskMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Collect disk usage
    partitions, err := disk.PartitionsWithContext(ctx, false)
    if err != nil {
        return err
    }

    for _, partition := range partitions {
        usage, err := disk.UsageWithContext(ctx, partition.Mountpoint)
        if err != nil {
            continue
        }

        mountLabels := append(baseLabels, partition.Mountpoint)
        c.metrics.DiskUsage.WithLabelValues(append(mountLabels, "total")...).Set(float64(usage.Total))
        c.metrics.DiskUsage.WithLabelValues(append(mountLabels, "used")...).Set(float64(usage.Used))
        c.metrics.DiskUsage.WithLabelValues(append(mountLabels, "free")...).Set(float64(usage.Free))
    }

    // Collect IO stats
    ioStats, err := disk.IOCountersWithContext(ctx)
    if err != nil {
        return err
    }

    for device, stats := range ioStats {
        deviceLabels := append(baseLabels, device)
        c.metrics.DiskIOPS.WithLabelValues(append(deviceLabels, "read")...).Set(float64(stats.ReadCount))
        c.metrics.DiskIOPS.WithLabelValues(append(deviceLabels, "write")...).Set(float64(stats.WriteCount))
    }

    return nil
}

func (c *Collector) collectNetworkMetrics(ctx context.Context) error {
    netStats, err := net.IOCountersWithContext(ctx, true)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()
    for _, stat := range netStats {
        interfaceLabels := append(baseLabels, stat.Name)
        c.metrics.NetworkIO.WithLabelValues(append(interfaceLabels, "sent")...).Add(float64(stat.BytesSent))
        c.metrics.NetworkIO.WithLabelValues(append(interfaceLabels, "received")...).Add(float64(stat.BytesRecv))
    }

    return nil
}

func (c *Collector) collectAdvancedSystemMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Collect load averages
    if load, err := cpu.LoadAvg(); err == nil {
        c.metrics.SystemLoad.WithLabelValues(append(baseLabels, "1min")...).Set(load.Load1)
        c.metrics.SystemLoad.WithLabelValues(append(baseLabels, "5min")...).Set(load.Load5)
        c.metrics.SystemLoad.WithLabelValues(append(baseLabels, "15min")...).Set(load.Load15)
    }

    // Collect TCP connection stats
    connections, err := net.ConnectionsWithContext(ctx, "tcp")
    if err == nil {
        states := make(map[string]int)
        for _, conn := range connections {
            states[conn.Status]++
        }
        for state, count := range states {
            c.metrics.TCPConnections.WithLabelValues(append(baseLabels, state)...).Set(float64(count))
        }
    }

    // Collect file descriptor stats
    if procs, err := process.Processes(); err == nil {
        c.metrics.SystemFileDesc.WithLabelValues(baseLabels...).Set(float64(len(procs)))
    }

    return nil
}

func (c *Collector) collectIOMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Collect IO wait time
    cpuTimes, err := cpu.TimesWithContext(ctx, true)
    if err == nil {
        for _, ct := range cpuTimes {
            c.metrics.IOWaitTime.WithLabelValues(
                append(baseLabels, fmt.Sprintf("cpu%d", ct.CPU))...,
            ).Set(ct.Iowait)
        }
    }

    // Collect disk latency
    if ioStats, err := disk.IOCountersWithContext(ctx); err == nil {
        for device, stats := range ioStats {
            deviceLabels := append(baseLabels, device)
            
            if stats.ReadCount > 0 {
                readLatency := float64(stats.ReadTime) / float64(stats.ReadCount)
                c.metrics.DiskLatency.WithLabelValues(
                    append(deviceLabels, "read")...,
                ).Observe(readLatency)
            }
            
            if stats.WriteCount > 0 {
                writeLatency := float64(stats.WriteTime) / float64(stats.WriteCount)
                c.metrics.DiskLatency.WithLabelValues(
                    append(deviceLabels, "write")...,
                ).Observe(writeLatency)
            }
        }
    }

    return nil
}
