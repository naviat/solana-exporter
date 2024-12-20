package rpc

import (
    "context"
    "encoding/json"
    "sync"
    "time"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client  *solana.Client
    metrics *metrics.Metrics
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics) *Collector {
    return &Collector{
        client:  client,
        metrics: metrics,
    }
}

func (c *Collector) Name() string {
    return "rpc"
}

type RPCPayload struct {
    Size int64
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 3)

    // Track common RPC methods
    methods := []string{
        "getSlot",
        "getBlock",
        "getTransaction",
        "getBalance",
        "getBlockHeight",
    }

    for _, method := range methods {
        wg.Add(1)
        go func(method string) {
            defer wg.Done()
            if err := c.measureRPCLatency(ctx, method); err != nil {
                errCh <- err
            }
        }(method)
    }

    // Collect request payload sizes
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.measureRequestSizes(ctx); err != nil {
            errCh <- err
        }
    }()

    // Track in-flight requests
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.trackInFlightRequests(ctx); err != nil {
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

func (c *Collector) measureRPCLatency(ctx context.Context, method string) error {
    start := time.Now()
    var result json.RawMessage

    err := c.client.Call(ctx, method, nil, &result)
    duration := time.Since(start).Seconds()

    // Record latency
    c.metrics.RPCLatency.WithLabelValues(method).Observe(duration)
    
    // Increment request counter
    c.metrics.RPCRequests.WithLabelValues(method).Inc()

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(method, "request_failed").Inc()
        return err
    }

    return nil
}

func (c *Collector) measureRequestSizes(ctx context.Context) error {
    // Sample different request types
    requests := map[string]interface{}{
        "getRecentBlockhash": nil,
        "getBlock": []interface{}{100},
        "getTransaction": []interface{}{"transaction_signature"},
    }

    for method, params := range requests {
        payload := RPCRequest{
            Jsonrpc: "2.0",
            ID:      1,
            Method:  method,
            Params:  params,
        }

        data, err := json.Marshal(payload)
        if err != nil {
            continue
        }

        c.metrics.RPCRequestSize.Observe(float64(len(data)))
    }

    return nil
}

func (c *Collector) trackInFlightRequests(ctx context.Context) error {
    // This would typically connect to RPC node's internal metrics
    // For now, we'll estimate based on recent requests
    var result struct {
        Current int `json:"current"`
    }

    err := c.client.Call(ctx, "getRecentPerformanceSamples", []interface{}{1}, &result)
    if err != nil {
        return err
    }

    c.metrics.RPCInFlight.Set(float64(result.Current))
    return nil
}
