package health

import (
    "context"
    "fmt"
    "runtime"
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
    SolanaCore    string `json:"solana-core"`
    FeatureSet    uint32 `json:"feature-set"`
}

type NodeIdentity struct {
    Identity string `json:"identity"`
}

type ClusterNode struct {
    Pubkey       string `json:"pubkey"`
    Gossip      string `json:"gossip"`
    TPU         string `json:"tpu"`
    RPC         string `json:"rpc"`
    Version     string `json:"version"`
    FeatureSet  uint32 `json:"featureSet"`
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
    errCh := make(chan error, 4)

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

    // Collect node identity and cluster info
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectNodeInfo(ctx); err != nil {
            errCh <- fmt.Errorf("node info check failed: %w", err)
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
    start := time.Now()
    var healthResp HealthResponse

    baseLabels := c.getBaseLabels()
    err := c.client.Call(ctx, "getHealth", nil, &healthResp)
    duration := time.Since(start).Seconds()

    // Record RPC latency
    c.metrics.RPCLatency.WithLabelValues(append(baseLabels, "getHealth")...).Observe(duration)

    if err != nil {
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "error")...).Set(0)
        c.metrics.RPCErrors.WithLabelValues(append(baseLabels, "getHealth", "request_failed")...).Inc()
        return err
    }

    // Update health status
    if healthResp.Status == "ok" {
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "ok")...).Set(1)
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "error")...).Set(0)
    } else {
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "ok")...).Set(0)
        c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "error")...).Set(1)
    }

    return nil
}

func (c *Collector) collectVersion(ctx context.Context) error {
    start := time.Now()
    var versionResp VersionResponse

    baseLabels := c.getBaseLabels()
    err := c.client.Call(ctx, "getVersion", nil, &versionResp)
    duration := time.Since(start).Seconds()

    c.metrics.RPCLatency.WithLabelValues(append(baseLabels, "getVersion")...).Observe(duration)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(append(baseLabels, "getVersion", "request_failed")...).Inc()
        return err
    }

    // Update version metrics
    c.metrics.NodeVersion.Reset()
    c.metrics.NodeVersion.WithLabelValues(
        append(baseLabels, versionResp.SolanaCore, fmt.Sprintf("%d", versionResp.FeatureSet))...,
    ).Set(1)

    return nil
}

func (c *Collector) collectNodeInfo(ctx context.Context) error {
    // Get node identity
    var identity NodeIdentity
    err := c.client.Call(ctx, "getIdentity", nil, &identity)
    if err != nil {
        return fmt.Errorf("failed to get node identity: %w", err)
    }

    // Get cluster nodes
    var nodes []ClusterNode
    err = c.client.Call(ctx, "getClusterNodes", nil, &nodes)
    if err != nil {
        return fmt.Errorf("failed to get cluster nodes: %w", err)
    }

    baseLabels := c.getBaseLabels()

    // Find current node in cluster nodes
    for _, node := range nodes {
        if node.Pubkey == identity.Identity {
            if node.RPC != "" {
                c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "rpc_available")...).Set(1)
            } else {
                c.metrics.NodeHealth.WithLabelValues(append(baseLabels, "rpc_available")...).Set(0)
            }
            break
        }
    }

    return nil
}

func (c *Collector) collectSystemHealth(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Memory stats
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "heap")...).Set(float64(m.HeapAlloc))
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "stack")...).Set(float64(m.StackInuse))
    c.metrics.MemoryUsage.WithLabelValues(append(baseLabels, "system")...).Set(float64(m.Sys))

    // Goroutine count
    c.metrics.GoroutineCount.WithLabelValues(baseLabels...).Set(float64(runtime.NumGoroutine()))

    // Update uptime
    c.metrics.LastRestartTime.WithLabelValues(baseLabels...).Set(float64(time.Now().Unix()))

    return nil
}
