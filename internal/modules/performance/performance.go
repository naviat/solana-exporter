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

type TransactionStats struct {
    Confirmed   uint64 `json:"numTransactionsConfirmed"`
    Failed      uint64 `json:"numTransactionsFailed"`
    ProcessTime float64 `json:"avgTransactionProcessTime"`
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
    errCh := make(chan error, 4)

    // Collect transaction performance
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectTransactionMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("transaction metrics: %w", err)
        }
    }()

    // Collect block performance
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectBlockMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("block metrics: %w", err)
        }
    }()

    // Collect slot performance
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectSlotMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("slot metrics: %w", err)
        }
    }()

    // Collect confirmation performance
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectConfirmationMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("confirmation metrics: %w", err)
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

        c.metrics.TxThroughput.WithLabelValues(baseLabels...).
            Set(totalTPS / float64(len(samples)))
    }

    // Collect additional transaction stats
    var txStats TransactionStats
    err = c.client.Call(ctx, "getTransactionCount", []interface{}{map[string]string{"commitment": "processed"}}, &txStats)
    if err != nil {
        return err
    }

    c.metrics.TxSuccessRate.WithLabelValues(baseLabels...).
        Set(float64(txStats.Confirmed) / float64(txStats.Confirmed+txStats.Failed) * 100)
    c.metrics.TxErrorRate.WithLabelValues(baseLabels...).
        Set(float64(txStats.Failed) / float64(txStats.Confirmed+txStats.Failed) * 100)
    c.metrics.TxConfirmationTime.WithLabelValues(baseLabels...).
        Observe(txStats.ProcessTime)

    return nil
}

func (c *Collector) collectBlockMetrics(ctx context.Context) error {
    var blockStats BlockProductionStats
    err := c.client.Call(ctx, "getBlockProduction", nil, &blockStats)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()

    blockRate := float64(blockStats.NumBlocks) / blockStats.SamplePeriod
    c.metrics.BlockProcessingTime.WithLabelValues(baseLabels...).
        Observe(blockStats.SamplePeriod / float64(blockStats.NumBlocks))

    // Get current block height
    var blockHeight uint64
    err = c.client.Call(ctx, "getBlockHeight", nil, &blockHeight)
    if err != nil {
        return err
    }

    c.metrics.BlockHeight.WithLabelValues(baseLabels...).Set(float64(blockHeight))

    return nil
}

func (c *Collector) collectSlotMetrics(ctx context.Context) error {
    // Get current and network slots
    var currentSlot, networkSlot uint64
    
    err := c.client.Call(ctx, "getSlot", nil, &currentSlot)
    if err != nil {
        return err
    }

    err = c.client.Call(ctx, "getSlot", []interface{}{map[string]string{"commitment": "finalized"}}, &networkSlot)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()
    
    c.metrics.CurrentSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))
    c.metrics.NetworkSlot.WithLabelValues(baseLabels...).Set(float64(networkSlot))
    c.metrics.SlotDiff.WithLabelValues(baseLabels...).Set(float64(networkSlot - currentSlot))

    return nil
}

func (c *Collector) collectConfirmationMetrics(ctx context.Context) error {
    // Get confirmation performance
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
