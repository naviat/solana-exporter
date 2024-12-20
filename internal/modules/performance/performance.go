package performance

import (
    "context"
    "fmt"
    "sync"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/internal/modules/common"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
}

type PerformanceSample struct {
    Slot              uint64  `json:"slot"`
    NumTransactions   uint64  `json:"numTransactions"`
    NumSlots          uint64  `json:"numSlots"`
    SamplePeriodSecs  float64 `json:"samplePeriodSecs"`
}

type BlockProductionStats struct {
    NumBlocks     uint64  `json:"numBlocks"`
    NumSlots      uint64  `json:"numSlots"`
    SamplePeriod  float64 `json:"samplePeriodSecs"`
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
    errCh := make(chan error, 5)

    // Existing collectors
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectTransactionMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("transaction metrics: %w", err)
        }
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectSlotMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("slot metrics: %w", err)
        }
    }()

    // New collectors
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectBlockPerformance(ctx); err != nil {
            errCh <- fmt.Errorf("block performance metrics: %w", err)
        }
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectTransactionRetryMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("transaction retry metrics: %w", err)
        }
    }()

    wg.Wait()
    close(errCh)

    return common.HandleErrors(errCh)
}

func (c *Collector) collectTransactionMetrics(ctx context.Context) error {
    var samples []PerformanceSample
    err := c.client.Call(ctx, "getRecentPerformanceSamples", []interface{}{5}, &samples)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()

    if len(samples) > 0 {
        var totalTPS float64
        for _, sample := range samples {
            tps := float64(sample.NumTransactions) / sample.SamplePeriodSecs
            totalTPS += tps

            c.metrics.TxThroughput.WithLabelValues(baseLabels...).Set(tps)
            c.metrics.TxPerSlot.WithLabelValues(append(baseLabels, "processed")...).
                Observe(float64(sample.NumTransactions) / float64(sample.NumSlots))
        }

        // Set average TPS
        avgTPS := totalTPS / float64(len(samples))
        c.metrics.TxThroughput.WithLabelValues(baseLabels...).Set(avgTPS)
    }

    return nil
}

func (c *Collector) collectSlotMetrics(ctx context.Context) error {
    var currentSlot, networkSlot uint64
    baseLabels := c.getBaseLabels()

    // Get current slot
    err := c.client.Call(ctx, "getSlot", nil, &currentSlot)
    if err != nil {
        return err
    }
    c.metrics.CurrentSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))

    // Get network slot
    err = c.client.Call(ctx, "getSlot", []interface{}{map[string]string{"commitment": "finalized"}}, &networkSlot)
    if err != nil {
        return err
    }
    c.metrics.NetworkSlot.WithLabelValues(baseLabels...).Set(float64(networkSlot))
    
    // Calculate slot difference
    c.metrics.SlotDiff.WithLabelValues(baseLabels...).Set(float64(networkSlot - currentSlot))

    return nil
}

func (c *Collector) collectConfirmationMetrics(ctx context.Context) error {
    type ConfirmationStats struct {
        Mean   float64 `json:"mean"`
        Stddev float64 `json:"stddev"`
    }

    var stats ConfirmationStats
    err := c.client.Call(ctx, "getConfirmationTimeStats", nil, &stats)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()
    c.metrics.TxConfirmationTime.WithLabelValues(baseLabels...).Observe(stats.Mean)
    
    return nil
}

func (c *Collector) collectBlockPerformance(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Collect block time metrics
    var blockTime struct {
        Average float64 `json:"average"`
        Max     float64 `json:"max"`
        Min     float64 `json:"min"`
    }
    err := c.client.Call(ctx, "getBlockTime", nil, &blockTime)
    if err == nil {
        c.metrics.BlockTime.WithLabelValues(append(baseLabels, "average")...).Set(blockTime.Average)
        c.metrics.BlockTime.WithLabelValues(append(baseLabels, "max")...).Set(blockTime.Max)
        c.metrics.BlockTime.WithLabelValues(append(baseLabels, "min")...).Set(blockTime.Min)
    }

    // Collect slot processing metrics
    var slotLeaders struct {
        CurrentSlot uint64 `json:"currentSlot"`
        Leaders     []string `json:"leaders"`
    }
    err = c.client.Call(ctx, "getSlotLeaders", nil, &slotLeaders)
    if err == nil {
        c.metrics.ValidatorCount.WithLabelValues(baseLabels...).Set(float64(len(slotLeaders.Leaders)))
    }

    return nil
}

func (c *Collector) collectTransactionRetryMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()
    
    var retryStats struct {
        Total    uint64 `json:"total"`
        Success  uint64 `json:"success"`
        Failed   uint64 `json:"failed"`
    }

    err := c.client.Call(ctx, "getTransactionRetryStats", nil, &retryStats)
    if err != nil {
        return err
    }

    c.metrics.TransactionRetries.WithLabelValues(append(baseLabels, "total")...).Add(float64(retryStats.Total))
    c.metrics.TransactionRetries.WithLabelValues(append(baseLabels, "success")...).Add(float64(retryStats.Success))
    c.metrics.TransactionRetries.WithLabelValues(append(baseLabels, "failed")...).Add(float64(retryStats.Failed))

    return nil
}
