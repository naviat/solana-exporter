package websocket

import (
    "context"
    "fmt"

    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
    "solana-exporter/config"
)

var monitoredSubscriptions = []string{
    "slot",        // New slot notifications
    "logs",        // Program log messages
    "account",     // Account updates
    "program",     // Program account changes
    "signature",   // Transaction status
    "block",       // New block notifications
}

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
    config     *config.Config
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
    return "websocket"
}

func (c *Collector) Priority() int {
    return c.config.Collector.WSPriority
}

func (c *Collector) Collect(ctx context.Context) error {
    // Get base labels first
    baseLabels := []string{c.nodeLabels["node_address"]}
    
    // Ensure WebSocket connection is established
    if err := c.client.ConnectWebSocket(ctx); err != nil {
        // Safely record the error metric
        if c.metrics.WSErrors != nil {
            c.metrics.WSErrors.WithLabelValues(append(baseLabels, "connection")...).Inc()
        }
        return fmt.Errorf("websocket connection failed: %w", err)
    }

    // Track connection count (with nil check)
    if c.metrics.WSConnections != nil {
        c.metrics.WSConnections.WithLabelValues(baseLabels...).Set(
            float64(c.client.GetWSConnectionCount()))
    }

    // Track message counts (with nil checks)
    if c.metrics.WSMessages != nil {
        c.metrics.WSMessages.WithLabelValues(
            append(baseLabels, "inbound")...,
        ).Add(float64(c.client.GetWSMessageCount("inbound")))

        c.metrics.WSMessages.WithLabelValues(
            append(baseLabels, "outbound")...,
        ).Add(float64(c.client.GetWSMessageCount("outbound")))
    }

    return nil
}
