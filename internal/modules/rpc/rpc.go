package rpc

import (
    "context"
    "encoding/json"
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

// Common RPC methods to monitor
var monitoredMethods = []string{
    "getAccountInfo",
    "getBalance",
    "getBlockHeight",
    "getSlot",
    "getTransaction",
    "getRecentBlockhash",
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
    errCh := make(chan error, len(monitoredMethods)+2)

    // Monitor RPC performance
    for _, method := range monitoredMethods {
        wg.Add(1)
        go func(method string) {
            defer wg.Done()
            if err := c.measureRPCLatency(ctx, method); err != nil {
                errCh <- fmt.Errorf("RPC measurement failed for %s: %w", method, err)
            }
        }(method)
    }

    // Collect request sizes
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.measureRequestSizes(ctx); err != nil {
            errCh <- fmt.Errorf("request size measurement failed: %w", err)
        }
    }()

    // Track in-flight requests
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.trackInFlightRequests(ctx); err != nil {
            errCh <- fmt.Errorf("in-flight tracking failed: %w", err)
        }
    }()

    wg.Wait()
    close(errCh)

    // Check for errors
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

    err := c.client.Call(ctx, method, getDefaultParams(method), &result)
    duration := time.Since(start).Seconds()

    // Record metrics with metadata
    c.metrics.RPCLatency.WithLabelValues(c.getLabels(method)...).Observe(duration)
    c.metrics.RPCRequests.WithLabelValues(c.getLabels(method)...).Inc()

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(c.getLabels(method, "request_failed")...).Inc()
        return err
    }

    return nil
}

func (c *Collector) measureRequestSizes(ctx context.Context) error {
    // Sample different request types
    sampleRequests := map[string][]interface{}{
        "getRecentBlockhash": nil,
        "getBlock": {100},
        "getTransaction": {"sample_signature"},
    }

    for method, params := range sampleRequests {
        payload := solana.RPCRequest{
            Jsonrpc: "2.0",
            ID:      1,
            Method:  method,
            Params:  params,
        }

        data, err := json.Marshal(payload)
        if err != nil {
            continue
        }

        c.metrics.RPCRequestSize.WithLabelValues(c.getLabels()...).Observe(float64(len(data)))
    }

    return nil
}

func (c *Collector) trackInFlightRequests(ctx context.Context) error {
    type rpcStats struct {
        CurrentRequests int `json:"currentRequests"`
    }

    var stats rpcStats
    err := c.client.Call(ctx, "getResourceConsumption", nil, &stats)
    if err != nil {
        return err
    }

    c.metrics.RPCInFlight.WithLabelValues(c.getLabels()...).Set(float64(stats.CurrentRequests))
    return nil
}

func getDefaultParams(method string) []interface{} {
    switch method {
    case "getAccountInfo":
        return []interface{}{make([]byte, 32)} // Empty pubkey
    case "getBlock":
        return []interface{}{0} // Genesis block
    case "getTransaction":
        return []interface{}{make([]byte, 64)} // Empty signature
    default:
        return nil
    }
}
