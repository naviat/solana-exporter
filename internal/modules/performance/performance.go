package performance

import (
    "context"
    "fmt"
    "sync"

    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
}

type PerformanceSample struct {
    Slot             uint64  `json:"slot"`
    NumTransactions  uint64  `json:"numTransactions"`
    NumSlots         uint64  `json:"numSlots"`
    SamplePeriodSecs float64 `json:"samplePeriodSecs"`
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

func (c *Collector) getBaseLabels() []string {
    return []string{c.nodeLabels["node_address"]}
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 2)

    collectors := []struct {
        name    string
        collect func(context.Context) error
    }{
        {"transaction_metrics", c.collectTransactionMetrics},
        {"slot_metrics", c.collectSlotMetrics},
    }

    for _, collector := range collectors {
        wg.Add(1)
        collectorName := collector.name
        collectFunc := collector.collect
        go func() {
            defer wg.Done()
            if err := collectFunc(ctx); err != nil {
                errCh <- fmt.Errorf("%s failed: %w", collectorName, err)
            }
        }()
    }

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

func (c *Collector) collectTransactionMetrics(ctx context.Context) error {
    var samples []PerformanceSample
    err := c.client.Call(ctx, "getRecentPerformanceSamples", []interface{}{5}, &samples)
    if err != nil {
        return fmt.Errorf("failed to get performance samples: %w", err)
    }

    baseLabels := c.getBaseLabels()

    if len(samples) > 0 {
        var totalTPS float64
        for _, sample := range samples {
            tps := float64(sample.NumTransactions) / sample.SamplePeriodSecs
            totalTPS += tps
        }

        // Set average TPS
        avgTPS := totalTPS / float64(len(samples))
        if c.metrics.TxThroughput != nil {
            c.metrics.TxThroughput.WithLabelValues(baseLabels...).Set(avgTPS)
        }
    }

    return nil
}

func (c *Collector) collectSlotMetrics(ctx context.Context) error {
    var errCh = make(chan error, 2)
    baseLabels := c.getBaseLabels()

    // Get current slot
    params := []interface{}{map[string]string{"commitment": "processed"}}
    var currentSlot uint64
    if err := c.client.Call(ctx, "getSlot", params, &currentSlot); err != nil {
        errCh <- fmt.Errorf("failed to get current slot: %w", err)
    } else if c.metrics.CurrentSlot != nil {
        c.metrics.CurrentSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))
    }

    // Get finalized slot
    finalizedParams := []interface{}{map[string]string{"commitment": "finalized"}}
    var finalizedSlot uint64
    if err := c.client.Call(ctx, "getSlot", finalizedParams, &finalizedSlot); err != nil {
        errCh <- fmt.Errorf("failed to get finalized slot: %w", err)
    } else if c.metrics.NetworkSlot != nil {
        c.metrics.NetworkSlot.WithLabelValues(baseLabels...).Set(float64(finalizedSlot))
    }

    close(errCh)
    return nil
}
