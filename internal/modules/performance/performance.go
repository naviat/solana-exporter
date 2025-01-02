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
        //{"websocket_performance", c.collectWebSocketPerformance},
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
    var errCh = make(chan error, 1)

    var samples []PerformanceSample
    err := c.client.Call(ctx, "getRecentPerformanceSamples", []interface{}{5}, &samples)
    if err != nil {
        errCh <- fmt.Errorf("failed to get performance samples: %w", err)
        close(errCh)
        return common.HandleErrors(errCh)
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

    close(errCh)
    return common.HandleErrors(errCh)
}

func (c *Collector) collectSlotMetrics(ctx context.Context) error {
    var errCh = make(chan error, 2) // 2 for both slot operations
    baseLabels := c.getBaseLabels()

    // Get current slot
    var currentSlot uint64
    if err := c.client.Call(ctx, "getSlot", nil, &currentSlot); err != nil {
        errCh <- fmt.Errorf("failed to get current slot: %w", err)
    } else if c.metrics.CurrentSlot != nil {
        c.metrics.CurrentSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))
    }

    // Get finalized slot
    if err := c.client.Call(ctx, "getSlot", []interface{}{map[string]string{"commitment": "finalized"}}, &currentSlot); err != nil {
        errCh <- fmt.Errorf("failed to get finalized slot: %w", err)
    } else if c.metrics.NetworkSlot != nil {
        c.metrics.NetworkSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))
    }

    close(errCh)
    return common.HandleErrors(errCh)
}

// func (c *Collector) collectWebSocketPerformance(ctx context.Context) error {
//     var errCh = make(chan error, 1)
//     baseLabels := c.getBaseLabels()

//     // Get WebSocket performance stats
//     var wsPerf struct {
//         Result struct {
//             AverageLatency    float64 `json:"averageLatency"`
//             MaxLatency        float64 `json:"maxLatency"`
//             EventsProcessed   uint64  `json:"eventsProcessed"`
//             ProcessingErrors  uint64  `json:"processingErrors"`
//         } `json:"result"`
//     }

//     if err := c.client.Call(ctx, "getWebsocketPerformance", nil, &wsPerf); err != nil {
//         errCh <- fmt.Errorf("failed to get websocket performance: %w", err)
//     } else {
//         // Record WebSocket latencies
//         c.metrics.WSLatency.WithLabelValues(append(baseLabels, "average")...).Observe(wsPerf.Result.AverageLatency)
//         c.metrics.WSLatency.WithLabelValues(append(baseLabels, "max")...).Observe(wsPerf.Result.MaxLatency)

//         // Record processing errors if any
//         if wsPerf.Result.ProcessingErrors > 0 {
//             c.metrics.WSErrors.WithLabelValues(append(baseLabels, "processing")...).
//                 Add(float64(wsPerf.Result.ProcessingErrors))
//         }
//     }

//     close(errCh)
//     return common.HandleErrors(errCh)
// }
