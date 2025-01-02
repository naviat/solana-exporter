package rpc

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "sync"
    "log"
    "strings"       // for Split

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
    "solana-rpc-monitor/internal/modules/common"
    "solana-rpc-monitor/config"
)

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
    config     *config.Config
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

// WebSocket subscription types to monitor
var monitoredSubscriptions = []string{
    "account",
    "block",
    "logs",
    "program",
    "root",
    "signature",
    "slot",
    "slotsUpdates",
    "vote",
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics, labels map[string]string, cfg *config.Config) *Collector {
    return &Collector{
        client:     client,
        metrics:    metrics,
        nodeLabels: labels,
        config:     cfg,
    }
}

func (c *Collector) Name() string {
    return "rpc"
}

func (c *Collector) getBaseLabels() []string {
    // Parse HTTP address to get host
    httpHost := strings.Split(c.nodeLabels["node_address"], ":")[0]
    wsAddress := fmt.Sprintf("%s:%d", httpHost, c.client.GetWSPort())
    return []string{wsAddress}
}

func (c *Collector) getHTTPLabels() []string {
    return []string{c.nodeLabels["node_address"]}
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(monitoredMethods)+3)
    
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
        name string
        collect func(context.Context) error
    }{
        {"queue_metrics", c.collectQueueMetrics},
        {"websocket_metrics", c.collectWebSocketMetrics},
        {"websocket_subscriptions", c.collectSubscriptionMetrics},
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

    baseLabels := c.getHTTPLabels()
    labels := append(baseLabels, method)
    // Skip health check method if it's getHealth
    if method == "getHealth" && !c.config.RPC.HealthCheckEnabled {
        return nil
    }
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

    // Handle errors with specific error codes
    if err != nil {
        errorType := "unknown"
        // Log the actual error for debugging
        log.Printf("RPC error for method %s: %v (type: %T)", method, err, err)
        if rpcErr, ok := err.(*solana.RPCError); ok {
            log.Printf("RPC error details - Code: %d, Message: %s", rpcErr.Code, rpcErr.Message)
            switch rpcErr.Code {
            case -32005:
                errorType = "node_behind"
            case -32601:
                errorType = "method_not_found"
            case -32602:
                errorType = "invalid_params"
            case -32603:
                errorType = "internal_error"
            case -32000:
                errorType = "server_error"
            case -32001:
                errorType = "block_cleaned"
            default:
                errorType = fmt.Sprintf("error_%d", rpcErr.Code)
            }
        } else {
            // Check if it's a network/connection error
            if strings.Contains(err.Error(), "connection refused") {
                errorType = "connection_refused"
            } else if strings.Contains(err.Error(), "timeout") {
                errorType = "timeout"
            }
        }
        c.metrics.RPCErrors.WithLabelValues(append(labels, errorType)...).Inc()
        return err
    }

    return nil
}

func (c *Collector) collectQueueMetrics(ctx context.Context) error {
    baseLabels := c.getHTTPLabels()

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

func (c *Collector) collectWebSocketMetrics(ctx context.Context) error {
    var errCh = make(chan error, 2)
    wsLabels := c.getBaseLabels() // Using WebSocket port for WS metrics

    // Track WebSocket connections
    wsCount := c.client.GetWSConnectionCount()
    if c.metrics.WSConnections != nil {
        c.metrics.WSConnections.WithLabelValues(wsLabels...).Set(float64(wsCount))
    }

    // Track subscriptions
    for _, subType := range monitoredSubscriptions {
        count := c.client.GetWSSubscriptionCount(subType)
        if c.metrics.WSSubscriptions != nil {
            c.metrics.WSSubscriptions.WithLabelValues(append(wsLabels, subType)...).Set(float64(count))
        }
    }

    // Get WebSocket stats if available
    var wsStats struct {
        Result struct {
            MessagesSent     uint64 `json:"messagesSent"`
            MessagesReceived uint64 `json:"messagesReceived"`
            ErrorCount       uint64 `json:"errorCount"`
        } `json:"result"`
    }

    err := c.client.Call(ctx, "getWebsocketStats", nil, &wsStats)
    if err == nil {
        if c.metrics.WSMessages != nil {
            // Record message counts
            c.metrics.WSMessages.WithLabelValues(append(wsLabels, "sent")...).Add(float64(wsStats.Result.MessagesSent))
            c.metrics.WSMessages.WithLabelValues(append(wsLabels, "received")...).Add(float64(wsStats.Result.MessagesReceived))
        }

        // Record error count
        if wsStats.Result.ErrorCount > 0 && c.metrics.WSErrors != nil {
            c.metrics.WSErrors.WithLabelValues(append(wsLabels, "general")...).Add(float64(wsStats.Result.ErrorCount))
        }
    }

    close(errCh)
    return common.HandleErrors(errCh)
}

func (c *Collector) collectSubscriptionMetrics(ctx context.Context) error {
    baseLabels := c.getBaseLabels()

    // Track subscription counts for each type
    for _, subType := range monitoredSubscriptions {
        count := c.client.GetWSSubscriptionCount(subType)
        c.metrics.WSSubscriptions.WithLabelValues(append(baseLabels, subType)...).Set(float64(count))
    }

    return nil
}

func (c *Collector) detectNetwork(ctx context.Context) (string, error) {
    var genesisHash string
    if err := c.client.Call(ctx, "getGenesisHash", nil, &genesisHash); err != nil {
        return "", fmt.Errorf("failed to get genesis hash: %w", err)
    }

    // Known genesis hashes
    switch genesisHash {
    case "5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d":
        return "mainnet-beta", nil
    case "EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG":
        return "devnet", nil
    case "4uhcVJyU9pJkvQyS88uRDiswHXSCkY3zQawwpjk2NsNY":
        return "testnet", nil
    default:
        return "", fmt.Errorf("unknown network genesis hash: %s", genesisHash)
    }
}

func (c *Collector) collectSlotMetrics(ctx context.Context) error {
    var errCh = make(chan error, 3)
    baseLabels := c.getHTTPLabels()

    // Detect network and get appropriate public RPC
    network, err := c.detectNetwork(ctx)
    if err != nil {
        errCh <- fmt.Errorf("failed to detect network: %w", err)
        close(errCh)
        return common.HandleErrors(errCh)
    }

    // Get local node slot
    params := []interface{}{map[string]string{"commitment": "processed"}}
    var currentSlot uint64
    if err := c.client.Call(ctx, "getSlot", params, &currentSlot); err != nil {
        errCh <- fmt.Errorf("failed to get current slot: %w", err)
    } else if c.metrics.CurrentSlot != nil {
        c.metrics.CurrentSlot.WithLabelValues(baseLabels...).Set(float64(currentSlot))
    }

    // Get appropriate public RPC endpoint
    var publicRPC string
    switch network {
    case "mainnet-beta":
        publicRPC = "https://api.mainnet-beta.solana.com"
    case "devnet":
        publicRPC = "https://api.devnet.solana.com"
    case "testnet":
        publicRPC = "https://api.testnet.solana.com"
    }

    // Get network slot
    if publicRPC != "" {
        publicClient := solana.NewClient(publicRPC, 0, time.Second*5, nil)
        networkParams := []interface{}{map[string]string{"commitment": "confirmed"}}
        var networkSlot uint64
        if err := publicClient.Call(ctx, "getSlot", networkParams, &networkSlot); err != nil {
            errCh <- fmt.Errorf("failed to get network slot: %w", err)
        } else if c.metrics.NetworkSlot != nil {
            c.metrics.NetworkSlot.WithLabelValues(baseLabels...).Set(float64(networkSlot))
            if c.metrics.SlotDiff != nil {
                c.metrics.SlotDiff.WithLabelValues(baseLabels...).Set(float64(networkSlot) - float64(currentSlot))
            }
        }
    }

    close(errCh)
    return common.HandleErrors(errCh)
}
