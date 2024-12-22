package health

import (
    "context"
    "fmt"
    "sync"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
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

func (c *Collector) getBaseLabels() []string {
    return []string{c.nodeLabels["node_address"]}
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 2)

    // Collect node health
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

    wg.Wait()
    close(errCh)

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

func (c *Collector) collectHealth(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Solana's getHealth returns "ok" as a string directly
    var healthStatus string
    err := c.client.Call(ctx, "getHealth", nil, &healthStatus)

    if err != nil {
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "error")...).Set(0)
        return err
    }

    if healthStatus == "ok" {
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "ok")...).Set(1)
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "error")...).Set(0)
    } else {
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "ok")...).Set(0)
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "error")...).Set(1)
    }

    return nil
}

func (c *Collector) collectVersion(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    type VersionResponse struct {
        SolanaCore string `json:"solana-core"`
        FeatureSet uint32 `json:"feature-set"`
    }

    var versionResp VersionResponse
    err := c.client.Call(ctx, "getVersion", nil, &versionResp)
    if err != nil {
        return err
    }

    // Update version metrics
    c.metrics.NodeVersion.WithLabelValues(
        append(baseLabels, versionResp.SolanaCore)...,
    ).Set(1)

    return nil
}
