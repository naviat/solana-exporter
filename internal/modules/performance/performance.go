package performance

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

type PerformanceSample struct {
    NumTransactions   uint64  `json:"numTransactions"`
    NumSlots         uint64  `json:"numSlots"`
    SamplePeriodSecs float64 `json:"samplePeriodSecs"`
    Slot            uint64  `json:"slot"`
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics, labels map[string]string) *Collector {
    return &Collector{
        client:     client,
        metrics:    metrics,
        nodeLabels: labels,
    }
}

func (c *Collector) Name() string {
    return "performance"
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

    // Collect transaction metrics
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectTransactionMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("transaction metrics failed: %w", err)
        }
    }()

    // Collect slot metrics
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectSlotMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("slot metrics failed: %w", err)
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

func (c *Collector) collectTransactionMetrics(ctx context.Context) error {
    start := time.Now()
    var samples []PerformanceSample

    err := c.client.Call(ctx, "getRecentPerformanceSamples", []interface{}{5}, &samples)
    duration := time.Since(start).Seconds()

    // Record RPC latency with metadata
    c.metrics.RPCLatency.WithLabelValues(c.getLabels("getRecentPerformanceSamples")...).Observe(duration)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(c.getLabels("getRecentPerformanceSamples", "request_failed")...).Inc()
        return err
    }

    if len(samples) > 0 {
        var totalTPS float64
        for _, sample := range samples {
            tps := float64(sample.NumTransactions) / sample.SamplePeriodSecs
            totalTPS += tps

            // Record metrics with metadata
            c.metrics.TxThroughput.WithLabelValues(c.getLabels()...).Set(tps)
            c.metrics.TxPerSlot.WithLabelValues(c.getLabels("processed")...).
                Observe(float64(sample.NumTransactions) / float64(sample.NumSlots))
        }

        // Set average TPS with metadata
        c.metrics.TxThroughput.WithLabelValues(c.getLabels()...).
            Set(totalTPS / float64(len(samples)))
    }

    return nil
}

func (c *Collector) collectSlotMetrics(ctx context.Context) error {
    start := time.Now()
    var currentSlot, networkSlot uint64

    // Get current slot
    err := c.client.Call(ctx, "getSlot", nil, &currentSlot)
    if err != nil {
        return err
    }

    // Get network slot
    err = c.client.Call(ctx, "getSlot", []interface{}{map[string]string{"commitment": "finalized"}}, &networkSlot)
    if err != nil {
        return err
    }

    // Record metrics with metadata
    c.metrics.CurrentSlot.WithLabelValues(c.getLabels()...).Set(float64(currentSlot))
    c.metrics.NetworkSlot.WithLabelValues(c.getLabels()...).Set(float64(networkSlot))
    c.metrics.SlotDiff.WithLabelValues(c.getLabels()...).Set(float64(networkSlot - currentSlot))

    return nil
}
