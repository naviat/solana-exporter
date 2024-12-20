package health

import (
    "context"
    "fmt"
    "sync"
    "time"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
}

type HealthResponse struct {
    Status string `json:"status"`
}

type VersionResponse struct {
    SolanaCore string `json:"solana-core"`
    FeatureSet uint32 `json:"feature-set"`
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics, labels map[string]string) *Collector {
    return &Collector{
        client:     client,
        metrics:    metrics,
        nodeLabels: labels,
    }
}

func (c *Collector) Name() string {
    return "health"
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
    errCh := make(chan error, 3)

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

    // Wait for collectors
    wg.Wait()
    close(errCh)

    // Check for errors
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

    // Record RPC latency with metadata
    c.metrics.RPCLatency.WithLabelValues(c.getLabels("getHealth")...).Observe(duration)

    if err != nil {
        c.metrics.NodeHealth.WithLabelValues(c.getLabels("error")...).Set(0)
        c.metrics.RPCErrors.WithLabelValues(c.getLabels("getHealth", "request_failed")...).Inc()
        return err
    }

    // Update health status with metadata
    if healthResp.Status == "ok" {
        c.metrics.NodeHealth.WithLabelValues(c.getLabels("ok")...).Set(1)
        c.metrics.NodeHealth.WithLabelValues(c.getLabels("error")...).Set(0)
    } else {
        c.metrics.NodeHealth.WithLabelValues(c.getLabels("ok")...).Set(0)
        c.metrics.NodeHealth.WithLabelValues(c.getLabels("error")...).Set(1)
    }

    return nil
}

func (c *Collector) collectVersion(ctx context.Context) error {
    start := time.Now()
    var versionResp VersionResponse

    err := c.client.Call(ctx, "getVersion", nil, &versionResp)
    duration := time.Since(start).Seconds()

    // Record RPC latency with metadata
    c.metrics.RPCLatency.WithLabelValues(c.getLabels("getVersion")...).Observe(duration)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(c.getLabels("getVersion", "request_failed")...).Inc()
        return err
    }

    // Update version metrics with metadata
    c.metrics.NodeVersion.Reset()
    c.metrics.NodeVersion.WithLabelValues(
        append(c.getLabels(), versionResp.SolanaCore, fmt.Sprintf("%d", versionResp.FeatureSet))...,
    ).Set(1)

    return nil
}
