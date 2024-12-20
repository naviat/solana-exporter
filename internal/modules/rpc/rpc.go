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

// Expanded RPC methods to monitor
var monitoredMethods = []string{
    // Basic node information
    "getHealth",
    "getVersion",
    "getIdentity",
    "getClusterNodes",
    
    // Block and slot information
    "getSlot",
    "getBlockHeight",
    "getBlock",
    "getBlocks",
    "getBlockTime",
    "getRecentBlockhash",
    "getFirstAvailableBlock",
    "getLatestBlockhash",
    
    // Transaction related
    "getTransaction",
    "getSignaturesForAddress",
    "getRecentPerformanceSamples",
    "getTransactionCount",
    "simulateTransaction",
    "sendTransaction",
    
    // Account and program information
    "getAccountInfo",
    "getBalance",
    "getProgramAccounts",
    "getTokenAccountBalance",
    "getTokenAccountsByOwner",
    "getTokenAccounts",
    
    // Epoch and slot information
    "getEpochInfo",
    "getEpochSchedule",
    "getLeaderSchedule",
    "getSlotLeader",
    
    // System information
    "getRecentPrioritizationFees",
    "getMaxRetransmitSlot",
    "getMaxShredInsertSlot",
    "getMinimumBalanceForRentExemption",
    
    // Supply information
    "getSupply",
    "getLargestAccounts",
}

// Method categories for better organization
var methodCategories = map[string]string{
    "getHealth":                     "health",
    "getVersion":                    "health",
    "getIdentity":                   "health",
    "getClusterNodes":               "health",
    
    "getSlot":                       "block",
    "getBlock":                      "block",
    "getBlockHeight":                "block",
    "getBlockTime":                  "block",
    "getBlocks":                     "block",
    "getRecentBlockhash":            "block",
    "getFirstAvailableBlock":        "block",
    "getLatestBlockhash":            "block",
    
    "getTransaction":                "transaction",
    "getSignaturesForAddress":       "transaction",
    "getRecentPerformanceSamples":   "transaction",
    "getTransactionCount":           "transaction",
    "simulateTransaction":           "transaction",
    "sendTransaction":               "transaction",
    
    "getAccountInfo":                "account",
    "getBalance":                    "account",
    "getProgramAccounts":            "account",
    "getTokenAccountBalance":        "account",
    "getTokenAccountsByOwner":       "account",
    "getTokenAccounts":              "account",
    
    "getEpochInfo":                  "epoch",
    "getEpochSchedule":              "epoch",
    "getLeaderSchedule":             "epoch",
    "getSlotLeader":                 "epoch",
    
    "getRecentPrioritizationFees":   "system",
    "getMaxRetransmitSlot":          "system",
    "getMaxShredInsertSlot":         "system",
    "getMinimumBalanceForRentExemption": "system",
    
    "getSupply":                     "supply",
    "getLargestAccounts":            "supply",
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
    category := methodCategories[method]
    labels := append(baseLabels, method, category)

    err := c.client.Call(ctx, method, getDefaultParams(method), &result)
    duration := time.Since(start).Seconds()

    c.metrics.RPCLatency.WithLabelValues(labels...).Observe(duration)
    c.metrics.RPCRequests.WithLabelValues(labels...).Inc()

    if err != nil {
        errorType := "request_failed"
        if jsonErr, ok := err.(*solana.RPCError); ok {
            errorType = fmt.Sprintf("error_%d", jsonErr.Code)
        }
        c.metrics.RPCErrors.WithLabelValues(append(labels, errorType)...).Inc()
        return err
    }

    responseSize := len(result)
    c.metrics.RPCResponseSize.Observe(float64(responseSize))

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
        c.metrics.RPCRequestSize.Observe(float64(len(data)))
    }

    return nil
}

func (c *Collector) trackInFlightRequests(ctx context.Context) error {
    type resourceStats struct {
        RPCRequests struct {
            Current int `json:"current"`
            Max     int `json:"max"`
        } `json:"rpcRequests"`
    }

    var stats resourceStats
    err := c.client.Call(ctx, "getResourceConsumption", nil, &stats)
    if err != nil {
        return err
    }

    baseLabels := c.getBaseLabels()
    c.metrics.RPCInFlight.WithLabelValues(baseLabels...).Set(float64(stats.RPCRequests.Current))
    return nil
}

func getDefaultParams(method string) []interface{} {
    switch method {
    case "getAccountInfo", "getBalance", "getTokenAccountBalance":
        return []interface{}{make([]byte, 32)}

    case "getBlock", "getBlockTime":
        return []interface{}{0}

    case "getBlocks":
        return []interface{}{0, 100}

    case "getTransaction", "getSignaturesForAddress":
        return []interface{}{make([]byte, 64)}

    case "getProgramAccounts":
        return []interface{}{
            make([]byte, 32),
            map[string]interface{}{
                "encoding": "base64",
            },
        }

    case "getTokenAccountsByOwner":
        return []interface{}{
            make([]byte, 32),
            map[string]interface{}{
                "programId": make([]byte, 32),
            },
        }

    case "simulateTransaction":
        return []interface{}{
            "base64_encoded_transaction",
            map[string]interface{}{
                "sigVerify": false,
                "preflight": true,
            },
        }

    case "getRecentPerformanceSamples":
        return []interface{}{10}

    case "getSignaturesForAddress":
        return []interface{}{
            make([]byte, 32),
            map[string]interface{}{
                "limit": 10,
            },
        }

    case "getLeaderSchedule":
        return []interface{}{
            nil,
            map[string]interface{}{
                "identity": make([]byte, 32),
            },
        }

    default:
        return nil
    }
}
