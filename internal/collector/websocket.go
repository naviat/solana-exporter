package collector

import (
    "context"
    "fmt"
    "time"

    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
)

// Essential WebSocket subscriptions to monitor
var monitoredSubscriptions = []string{
    "slot",     // For slot updates
    "blocks",   // For block production monitoring
    "roots",    // For root tracking
}

type WSCollector struct {
    client      *solana.Client
    metrics     *metrics.Metrics
    nodeLabels  map[string]string
}

func NewWSCollector(client *solana.Client, metrics *metrics.Metrics, labels map[string]string) *WSCollector {
    return &WSCollector{
        client:     client,
        metrics:    metrics,
        nodeLabels: labels,
    }
}

func (c *WSCollector) Name() string {
    return "websocket"
}

func (c *WSCollector) Collect(ctx context.Context) error {
    // Collect WebSocket metrics concurrently
    var errs []error

    // Collect connection metrics
    if err := c.collectConnectionMetrics(ctx); err != nil {
        errs = append(errs, fmt.Errorf("connection metrics: %w", err))
    }

    // Collect subscription metrics
    if err := c.collectSubscriptionMetrics(ctx); err != nil {
        errs = append(errs, fmt.Errorf("subscription metrics: %w", err))
    }

    // Collect performance metrics
    if err := c.collectPerformanceMetrics(ctx); err != nil {
        errs = append(errs, fmt.Errorf("performance metrics: %w", err))
    }

    if len(errs) > 0 {
        return fmt.Errorf("websocket collection errors: %v", errs)
    }

    return nil
}

func (c *WSCollector) collectConnectionMetrics(ctx context.Context) error {
    stats := c.client.GetWSStats()
    
    // Get current connection count from stats
    if connected, ok := stats["connected"].(bool); ok && connected {
        c.metrics.WSConnections.WithLabelValues(
            c.nodeLabels["endpoint"],
        ).Set(1)
    } else {
        c.metrics.WSConnections.WithLabelValues(
            c.nodeLabels["endpoint"],
        ).Set(0)
    }

    return nil
}

func (c *WSCollector) collectSubscriptionMetrics(ctx context.Context) error {
    stats := c.client.GetWSStats()
    
    // Get subscription count
    if count, ok := stats["subscription_count"].(int); ok {
        c.metrics.WSSubscriptions.WithLabelValues(
            c.nodeLabels["endpoint"],
            "total",
        ).Set(float64(count))
    }

    // Test each subscription type
    for _, subType := range monitoredSubscriptions {
        // Attempt subscription
        start := time.Now()
        err := c.client.Subscribe(ctx, fmt.Sprintf("%sSubscribe", subType), nil)
        duration := time.Since(start)

        if err != nil {
            c.metrics.WSErrors.WithLabelValues(
                c.nodeLabels["endpoint"],
                fmt.Sprintf("subscription_%s", subType),
            ).Inc()
        } else {
            c.metrics.WSLatency.WithLabelValues(
                c.nodeLabels["endpoint"],
                subType,
            ).Observe(duration.Seconds())
        }
    }

    return nil
}

func (c *WSCollector) collectPerformanceMetrics(ctx context.Context) error {
    stats := c.client.GetWSStats()
    
    // Record message counts
    if msgCount, ok := stats["message_count"].(int64); ok {
        c.metrics.WSMessageRate.WithLabelValues(
            c.nodeLabels["endpoint"],
            "total",
        ).Add(float64(msgCount))
    }

    // Record error count
    if errCount, ok := stats["error_count"].(int64); ok {
        c.metrics.WSErrors.WithLabelValues(
            c.nodeLabels["endpoint"],
            "total",
        ).Add(float64(errCount))
    }

    return nil
}

func (c *WSCollector) Stop() error {
    // Nothing to clean up as the client handles WebSocket connection cleanup
    return nil
}
