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
    errCh := make(chan error, 3)

    collectors := []struct {
        name    string
        collect func(context.Context) error
    }{
        {"transaction_metrics", c.collectTransactionMetrics},
        {"slot_metrics", c.collectSlotMetrics},
        {"block_metrics", c.collectBlockMetrics},
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

    return common.HandleErrors(errCh)
}

func (c *Collector) collectBlockMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Get slot info
    var epochInfo struct {
        AbsoluteSlot uint64 `json:"absoluteSlot"`
        BlockHeight  uint64 `json:"blockHeight"`
    }
    err := c.client.Call(ctx, "getEpochInfo", nil, &epochInfo)
    if err == nil {
        if c.metrics.BlockHeight != nil {
            c.metrics.BlockHeight.WithLabelValues(baseLabels...).Set(float64(epochInfo.BlockHeight))
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
    baseLabels := c.getBaseLabels()

    // Get current slot
    var currentSlot uint64
    err := c.client.Call(ctx, "getSlot", nil, &currentSlot)
    if err == nil && c.metrics.CurrentSlot != nil {
        c.metrics.CurrentSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))
    }

    // Get finalized slot
    err = c.client.Call(ctx, "getSlot", []interface{}{map[string]string{"commitment": "finalized"}}, &currentSlot)
    if err == nil {
        if c.metrics.NetworkSlot != nil {
            c.metrics.NetworkSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))
        }
    }

    return nil
}

// func (c *Collector) collectConfirmationMetrics(ctx context.Context) error {
//     if c.metrics.TxConfirmationTime == nil {
//         return nil
//     }

//     type ConfirmationStats struct {
//         Mean   float64 `json:"mean"`
//         Stddev float64 `json:"stddev"`
//     }

//     var stats ConfirmationStats
//     err := c.client.Call(ctx, "getConfirmationTimeStats", nil, &stats)
//     if err != nil {
//         return err
//     }

//     baseLabels := c.getBaseLabels()
//     c.metrics.TxConfirmationTime.WithLabelValues(baseLabels...).Observe(stats.Mean)
    
//     return nil
// }

// func (c *Collector) collectBlockPerformance(ctx context.Context) error {
//     baseLabels := c.getBaseLabels()

//     // Collect block time metrics
//     if c.metrics.BlockTime != nil {
//         var blockTime struct {
//             Average float64 `json:"average"`
//             Max     float64 `json:"max"`
//             Min     float64 `json:"min"`
//         }
//         if err := c.client.Call(ctx, "getBlockTime", nil, &blockTime); err == nil {
//             c.metrics.BlockTime.WithLabelValues(baseLabels...).Set(blockTime.Average)
//         }
//     }

//     // Get block height and processing time
//     if c.metrics.BlockProcessingTime != nil {
//         var blockStats struct {
//             ProcessingTime float64 `json:"processingTime"`
//         }
//         if err := c.client.Call(ctx, "getRecentBlockStats", nil, &blockStats); err == nil {
//             c.metrics.BlockProcessingTime.WithLabelValues(baseLabels...).Observe(blockStats.ProcessingTime)
//         }
//     }

//     return nil
// }

// func (c *Collector) collectTransactionRetryMetrics(ctx context.Context) error {
//     if c.metrics.TransactionRetries == nil {
//         return nil
//     }

//     baseLabels := c.getBaseLabels()
    
//     var retryStats struct {
//         Total    uint64 `json:"total"`
//         Success  uint64 `json:"success"`
//         Failed   uint64 `json:"failed"`
//     }

//     err := c.client.Call(ctx, "getTransactionRetryStats", nil, &retryStats)
//     if err != nil {
//         return err
//     }

//     c.metrics.TransactionRetries.WithLabelValues(append(baseLabels, "total")...).Add(float64(retryStats.Total))
//     c.metrics.TransactionRetries.WithLabelValues(append(baseLabels, "success")...).Add(float64(retryStats.Success))
//     c.metrics.TransactionRetries.WithLabelValues(append(baseLabels, "failed")...).Add(float64(retryStats.Failed))

//     return nil
// }
