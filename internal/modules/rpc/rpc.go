package rpc

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/internal/modules/common"
    "solana-rpc-monitor/pkg/solana"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
}

// Common RPC methods to monitor
var monitoredMethods = []string{
    // Basic node information
    "getHealth",
    "getVersion",
    "getIdentity",
    
    // Block and slot information
    "getSlot",
    "getBlockHeight",
    "getBlock",
    "getBlockTime",
    "getRecentBlockhash",
    
    // Transaction related
    "getTransaction",
    "getSignaturesForAddress",
    "getRecentPerformanceSamples",
    "getTransactionCount",
    
    // Account and program information
    "getAccountInfo",
    "getBalance",
    "getProgramAccounts",
}

// Method categories for better organization
var methodCategories = map[string]string{
    "getHealth":                   "health",
    "getVersion":                  "health",
    "getIdentity":                "health",
    
    "getSlot":                    "block",
    "getBlock":                   "block",
    "getBlockHeight":             "block",
    "getBlockTime":               "block",
    
    "getTransaction":             "transaction",
    "getRecentPerformanceSamples": "transaction",
    "getTransactionCount":        "transaction",
    
    "getAccountInfo":             "account",
    "getBalance":                 "account",
    "getProgramAccounts":         "account",
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
    
    // Monitor RPC methods
    for _, method := range monitoredMethods {
        wg.Add(1)
        // Important: Create local variable for goroutine
        methodName := method
        go func() {
            defer wg.Done()
            if err := c.measureRPCLatency(ctx, methodName); err != nil {
                errCh <- fmt.Errorf("RPC measurement failed for %s: %w", methodName, err)
            }
        }()
    }

    // Existing collectors
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.measureRPCLatency(ctx); err != nil {
            errCh <- fmt.Errorf("RPC latency measurement failed: %w", err)
        }
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.measureRequestSizes(ctx); err != nil {
            errCh <- fmt.Errorf("request size measurement failed: %w", err)
        }
    }()

    // New collectors
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectAdvancedRPCMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("advanced RPC metrics failed: %w", err)
        }
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.collectCacheMetrics(ctx); err != nil {
            errCh <- fmt.Errorf("cache metrics failed: %w", err)
        }
    }()

    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := c.trackInFlightRequests(ctx); err != nil {
            errCh <- fmt.Errorf("in-flight tracking failed: %w", err)
        }
    }()

    // Additional collectors
    collectors := []struct {
        name    string
        collect func(context.Context) error
    }{
        {"request_sizes", c.measureRequestSizes},
        {"in_flight", c.trackInFlightRequests},
        {"advanced_metrics", c.collectAdvancedRPCMetrics},
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
        }(collectorName, collectFunc)
    }

    wg.Wait()
    close(errCh)

    return common.HandleErrors(errCh)
}

func (c *Collector) measureRPCLatency(ctx context.Context, method string) error {
    start := time.Now()
    var result json.RawMessage

    baseLabels := c.getBaseLabels()
    category := methodCategories[method]
    labels := append(baseLabels, method, category)

    err := c.client.Call(ctx, method, getDefaultParams(method), &result)
    duration := time.Since(start).Seconds()

    c.metrics.RPCLatency.WithLabelValues(labels...).Observe(duration)
    c.metrics.RPCRequests.WithLabelValues(labels...).Inc()

    if err != nil {
        errorType := "request_failed"
        if rpcErr, ok := err.(*solana.RPCError); ok {
            errorType = fmt.Sprintf("error_%d", rpcErr.Code)
        }
        c.metrics.RPCErrors.WithLabelValues(append(labels, errorType)...).Inc()
        return err
    }

    if result != nil {
        c.metrics.RPCResponseSize.WithLabelValues(labels...).Observe(float64(len(result)))
    }

    return nil
}

func (c *Collector) measureRequestSizes(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    for method := range methodCategories {
        payload := solana.RPCRequest{
            Jsonrpc: "2.0",
            ID:      1,
            Method:  method,
            Params:  getDefaultParams(method),
        }

        data, err := json.Marshal(payload)
        if err != nil {
            continue
        }

        labels := append(baseLabels, method, methodCategories[method])
        c.metrics.RPCRequestSize.WithLabelValues(labels...).Observe(float64(len(data)))
    }

    return nil
}

func (c *Collector) trackInFlightRequests(ctx context.Context) error {
    baseLabels := c.getBaseLabels()
    c.metrics.RPCInFlight.WithLabelValues(baseLabels...).Set(float64(0))
    return nil
}

func getDefaultParams(method string) []interface{} {
    switch method {
    case "getAccountInfo", "getBalance":
        return []interface{}{make([]byte, 32)}
    case "getBlock", "getBlockTime":
        return []interface{}{0}
    case "getTransaction":
        return []interface{}{make([]byte, 64)}
    case "getProgramAccounts":
        return []interface{}{
            make([]byte, 32),
            map[string]interface{}{
                "encoding": "base64",
            },
        }
    default:
        return nil
    }
}

func (c *Collector) collectAdvancedRPCMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Collect websocket metrics
    var wsStats struct {
        Connected     int `json:"connected"`
        MessagesSent  int `json:"messagesSent"`
        MessagesRecv  int `json:"messagesReceived"`
    }
    err := c.client.Call(ctx, "getWebsocketStats", nil, &wsStats)
    if err == nil {
        c.metrics.RPCWebsocketConns.WithLabelValues(baseLabels...).Set(float64(wsStats.Connected))
    }

    // Collect RPC queue metrics
    var queueStats struct {
        Depth     int `json:"depth"`
        RateLimit int `json:"rateLimit"`
    }
    err = c.client.Call(ctx, "getQueueStats", nil, &queueStats)
    if err == nil {
        c.metrics.RPCQueueDepth.WithLabelValues(baseLabels...).Set(float64(queueStats.Depth))
        if queueStats.RateLimit > 0 {
            c.metrics.RPCRateLimit.WithLabelValues(baseLabels...).Inc()
        }
    }

    return nil
}

func (c *Collector) collectCacheMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Account cache stats
    var accountCache struct {
        Size     int     `json:"size"`
        Hits     int64   `json:"hits"`
        Misses   int64   `json:"misses"`
        Evicted  int64   `json:"evicted"`
    }
    err := c.client.Call(ctx, "getAccountCacheStats", nil, &accountCache)
    if err == nil {
        c.metrics.AccountsCache.WithLabelValues(append(baseLabels, "size")...).Set(float64(accountCache.Size))
        c.metrics.AccountsCache.WithLabelValues(append(baseLabels, "hit_rate")...).
            Set(float64(accountCache.Hits) / float64(accountCache.Hits + accountCache.Misses))
    }

    // Transaction cache stats
    var txCache struct {
        Size     int     `json:"size"`
        Hits     int64   `json:"hits"`
        Misses   int64   `json:"misses"`
    }
    err = c.client.Call(ctx, "getTransactionCacheStats", nil, &txCache)
    if err == nil {
        c.metrics.TransactionCache.WithLabelValues(append(baseLabels, "size")...).Set(float64(txCache.Size))
        c.metrics.TransactionCache.WithLabelValues(append(baseLabels, "hit_rate")...).
            Set(float64(txCache.Hits) / float64(txCache.Hits + txCache.Misses))
    }

    return nil
}

// Helper function for error handling
func handleErrors(errCh chan error) error {
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
