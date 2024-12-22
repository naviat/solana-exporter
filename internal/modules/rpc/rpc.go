package rpc

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "sync"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
}

// Common RPC methods to monitor - updated for actual Solana RPC methods
var monitoredMethods = []string{
    // Basic node information
    "getHealth",
    "getVersion",
    "getIdentity",
    
    // Block and slot information
    "getSlot",
    "getBlockHeight",
    "getLatestBlockhash",
    
    // Transaction related
    "getRecentPerformanceSamples",
    "getTransactionCount",
    
    // Account information
    "getInflationRate",
    "getInflationGovernor",
    "getEpochInfo",
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics, labels map[string]string) *Collector {
    return &Collector{
        client:     client,
        metrics:    metrics,
        nodeLabels: labels,
    }
}

func (c *Collector) Name() string {
    return "rpc"
}

func (c *Collector) getBaseLabels() []string {
    return []string{c.nodeLabels["node_address"]}
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(monitoredMethods)+2)
    
    // Monitor each RPC method
    for _, method := range monitoredMethods {
        wg.Add(1)
        methodName := method
        go func() {
            defer wg.Done()
            if err := c.measureRPCLatency(ctx, methodName); err != nil {
                errCh <- fmt.Errorf("RPC measurement failed for %s: %w", methodName, err)
            }
        }()
    }

    // Additional collectors
    collectors := []struct {
        name    string
        collect func(context.Context) error
    }{
        {"queue_metrics", c.collectQueueMetrics},
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

    // Collect any errors
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

func (c *Collector) measureRPCLatency(ctx context.Context, method string) error {
    start := time.Now()
    var result json.RawMessage

    baseLabels := c.getBaseLabels()
    labels := append(baseLabels, method)

    // Add appropriate params based on method
    var params []interface{}
    switch method {
    case "getLatestBlockhash":
        params = []interface{}{map[string]string{"commitment": "finalized"}}
    case "getSlot":
        params = []interface{}{map[string]string{"commitment": "finalized"}}
    case "getBlockHeight":
        params = []interface{}{map[string]string{"commitment": "finalized"}}
    case "getRecentPerformanceSamples":
        params = []interface{}{10} // Get last 10 samples
    default:
        params = nil
    }

    err := c.client.Call(ctx, method, params, &result)
    duration := time.Since(start).Seconds()

    // Record latency metric
    c.metrics.RPCLatency.WithLabelValues(labels...).Observe(duration)
    
    // Record request count
    c.metrics.RPCRequests.WithLabelValues(labels...).Inc()

    // Handle errors
    if err != nil {
        errorType := "request_failed"
        if rpcErr, ok := err.(*solana.RPCError); ok {
            errorType = fmt.Sprintf("error_%d", rpcErr.Code)
        }
        c.metrics.RPCErrors.WithLabelValues(append(labels, errorType)...).Inc()
        return err
    }

    return nil
}

func (c *Collector) collectQueueMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Track in-flight requests
    inFlightCount := c.client.GetInflightRequests()
    if c.metrics.RPCInFlight != nil {
        c.metrics.RPCInFlight.WithLabelValues(baseLabels...).Set(float64(inFlightCount))
    }

    // Get mempool/queue size using getPendingTransactionCount
    var pendingTxResp struct {
        Result uint64 `json:"result"`
    }
    err := c.client.Call(ctx, "getTransactionCount", []interface{}{map[string]string{
        "commitment": "processed",
    }}, &pendingTxResp)
    if err == nil && c.metrics.RPCQueueDepth != nil {
        c.metrics.RPCQueueDepth.WithLabelValues(baseLabels...).Set(float64(pendingTxResp.Result))
    }

    return nil
}
