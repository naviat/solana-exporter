package health

import (
    "context"
    "fmt"
    "runtime"
    "time"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client  *solana.Client
    metrics *metrics.Metrics
}

type HealthResponse struct {
    Status string `json:"status"`
}

type VersionResponse struct {
    SolanaCore    string            `json:"solana-core"`
    FeatureSet    uint32           `json:"feature-set"`
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics) *Collector {
    return &Collector{
        client:  client,
        metrics: metrics,
    }
}

func (c *Collector) Name() string {
    return "health"
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 3) // Buffer for potential errors

    // Collect health metrics
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectHealth(ctx); err != nil {
            errCh <- fmt.Errorf("health check failed: %w", err)
        }
    }()

    // Collect version info
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectVersion(ctx); err != nil {
            errCh <- fmt.Errorf("version check failed: %w", err)
        }
    }()

    // Collect system health
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectSystemHealth(ctx); err != nil {
            errCh <- fmt.Errorf("system health check failed: %w", err)
        }
    }()

    // Wait for all collectors to finish
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

func (c *Collector) collectHealth(ctx context.Context) error {
    start := time.Now()
    var healthResp HealthResponse

    err := c.client.Call(ctx, "getHealth", nil, &healthResp)
    duration := time.Since(start).Seconds()

    // Record RPC latency
    c.metrics.RPCLatency.WithLabelValues("getHealth").Observe(duration)

    if err != nil {
        c.metrics.NodeHealth.WithLabelValues("error").Set(0)
        c.metrics.RPCErrors.WithLabelValues("getHealth", "request_failed").Inc()
        return err
    }

    // Update health status
    if healthResp.Status == "ok" {
        c.metrics.NodeHealth.WithLabelValues("ok").Set(1)
        c.metrics.NodeHealth.WithLabelValues("error").Set(0)
    } else {
        c.metrics.NodeHealth.WithLabelValues("ok").Set(0)
        c.metrics.NodeHealth.WithLabelValues("error").Set(1)
    }

    return nil
}

func (c *Collector) collectVersion(ctx context.Context) error {
    var versionResp VersionResponse

    start := time.Now()
    err := c.client.Call(ctx, "getVersion", nil, &versionResp)
    duration := time.Since(start).Seconds()

    c.metrics.RPCLatency.WithLabelValues("getVersion").Observe(duration)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues("getVersion", "request_failed").Inc()
        return err
    }

    // Update version metrics
    c.metrics.NodeVersion.Reset()
    c.metrics.NodeVersion.WithLabelValues(
        versionResp.SolanaCore,
        fmt.Sprintf("%d", versionResp.FeatureSet),
    ).Set(1)

    return nil
}

func (c *Collector) collectSystemHealth(ctx context.Context) error {
    // Collect system metrics
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    // Update memory metrics
    c.metrics.MemoryUsage.WithLabelValues("heap").Set(float64(m.HeapAlloc))
    c.metrics.MemoryUsage.WithLabelValues("stack").Set(float64(m.StackInuse))
    c.metrics.MemoryUsage.WithLabelValues("system").Set(float64(m.Sys))

    // Update goroutine count
    c.metrics.GoroutineCount.Set(float64(runtime.NumGoroutine()))

    // Get current time for uptime calculation
    currentTime := time.Now().Unix()
    c.metrics.LastRestartTime.Set(float64(currentTime))

    return nil
}
