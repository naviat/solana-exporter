package cache

import (
    "context"
    "time"

    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
    "solana-exporter/config"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
    config     *config.Config
}

// CacheStats represents statistics for different types of caches
type CacheStats struct {
    Hits        uint64
    Misses      uint64
    Size        uint64
    MaxSize     uint64
    EvictionCnt uint64
}

var cacheTypes = []string{
    "accounts",     // Account data cache
    "programs",     // Program cache
    "blocks",       // Block cache
    "transactions", // Transaction cache
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics, labels map[string]string, cfg *config.Config) *Collector {
    return &Collector{
        client:     client,
        metrics:    metrics,
        nodeLabels: labels,
        config:     cfg,
    }
}

func (c *Collector) Name() string {
    return "cache"
}

func (c *Collector) Priority() int {
    return c.config.Collector.CachePriority
}

func (c *Collector) Collect(ctx context.Context) error {
    baseLabels := []string{c.nodeLabels["node_address"]}

    // Instead of trying to get cache stats directly, we can measure cache effectiveness
    // through other metrics like transaction processing time
    var slot uint64
    start := time.Now()
    err := c.client.Call(ctx, "getSlot", nil, &slot)
    duration := time.Since(start)

    if err == nil && c.metrics.CacheHitRate != nil {
        // Use response time as a proxy for cache effectiveness
        // Faster responses likely indicate cache hits
        hitRate := 100.0 * (1.0 - (duration.Seconds() / 1.0)) // 1 second is our baseline
        if hitRate < 0 {
            hitRate = 0
        }
        if hitRate > 100 {
            hitRate = 100
        }
        c.metrics.CacheHitRate.WithLabelValues(append(baseLabels, "slot_query")...).Set(hitRate)
    }

    return nil
}
