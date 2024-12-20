package performance

import (
    "context"
    "sync"
    "time"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client  *solana.Client
    metrics *metrics.Metrics
}

type PerformanceSample struct {
    Slot              uint64  `json:"slot"`
    NumTransactions   uint64  `json:"numTransactions"`
    NumSlots          uint64  `json:"numSlots"`
    SamplePeriodSecs  float64 `json:"samplePeriodSecs"`
}

type TransactionError struct {
    Count   uint64 `json:"count"`
    Message string `json:"message"`
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics) *Collector {
    return &Collector{
        client:  client,
        metrics: metrics,
    }
}

func (c *Collector) Name() string {
    return "performance"
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 4)

    // Collect TPS metrics
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectTransactionMetrics(ctx); err != nil {
            errCh <- err
        }
    }()

    // Collect block processing metrics
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectBlockProcessingMetrics(ctx); err != nil {
            errCh <- err
        }
    }()

    // Collect confirmation times
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectConfirmationMetrics(ctx); err != nil {
            errCh <- err
        }
    }()

    // Wait for all collectors
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

func (c *Collector) collectTransactionMetrics(ctx context.Context) error {
    var samples []PerformanceSample
    err := c.client.Call(ctx, "getRecentPerformanceSamples", []interface{}{5}, &samples)
    if err != nil {
        return err
    }

    if len(samples) > 0 {
        var totalTPS float64
        for _, sample := range samples {
            tps := float64(sample.NumTransactions) / sample.SamplePeriodSecs
            totalTPS += tps
            c.metrics.TxThroughput.Set(tps)
            c.metrics.TxPerSlot.WithLabelValues("processed").Observe(float64(sample.NumTransactions) / float64(sample.NumSlots))
        }

        // Set average TPS
        c.metrics.TxThroughput.Set(totalTPS / float64(len(samples)))
    }

    return nil
}

func (c *Collector) collectBlockProcessingMetrics(ctx context.Context) error {
    var blockTime struct {
        Average float64 `json:"average"`
    }

    err := c.client.Call(ctx, "getBlockTime", []interface{}{}, &blockTime)
    if err != nil {
        return err
    }

    c.metrics.BlockProcessingTime.Observe(blockTime.Average)
    return nil
}

func (c *Collector) collectConfirmationMetrics(ctx context.Context) error {
    var confirmationTime struct {
        Mean   float64 `json:"mean"`
        Stddev float64 `json:"stddev"`
    }

    err := c.client.Call(ctx, "getConfirmationTime", []interface{}{}, &confirmationTime)
    if err != nil {
        return err
    }

    c.metrics.TxConfirmationTime.Observe(confirmationTime.Mean)
    return nil
}
