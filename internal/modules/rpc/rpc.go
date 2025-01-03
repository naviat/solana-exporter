package rpc

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
    "solana-exporter/config"
)

// Key RPC methods we want to monitor for API performance
var monitoredMethods = []string{
    // Essential methods that affect all operations
    "getHealth",
    "getVersion",
    
    // Core transaction and account methods
    "getLatestBlockhash",
    "getSignatureStatuses",
    "getTransaction",
    "getBalance",
    "getAccountInfo",
    "getProgramAccounts",
    
    // Block and timing information
    "getSlot",
    "getBlock",
    "getBlockTime",
}

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    nodeLabels map[string]string
    config     *config.Config
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

func (c *Collector) Priority() int {
    return c.config.Collector.RPCPriority // Highest priority as it's core functionality
}

func (c *Collector) Collect(ctx context.Context) error {
    baseLabels := []string{c.nodeLabels["node_address"]}

    // Collect RPC metrics sequentially to avoid overwhelming the node
    for _, method := range monitoredMethods {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := c.measureMethod(ctx, method, baseLabels); err != nil {
                // Log error but continue with other methods
                fmt.Printf("Error measuring method %s: %v\n", method, err)
            }
            // Brief pause between methods
            time.Sleep(100 * time.Millisecond)
        }
    }

    // Collect queue metrics
    return c.collectQueueMetrics(ctx, baseLabels)
}

func (c *Collector) measureMethod(ctx context.Context, method string, baseLabels []string) error {
    labels := append(baseLabels, method)
    
    start := time.Now()
    var result json.RawMessage
    
    err := c.client.Call(ctx, method, c.getMethodParams(method), &result)
    duration := time.Since(start).Seconds()

    // Record base metrics
    c.metrics.RPCLatency.WithLabelValues(labels...).Observe(duration)
    c.metrics.RPCRequests.WithLabelValues(labels...).Inc()

    // Handle potential errors
    if err != nil {
        errorType := c.categorizeError(err)
        c.metrics.RPCErrors.WithLabelValues(append(labels, errorType)...).Inc()
        return err
    }

    return nil
}

func (c *Collector) getMethodParams(method string) []interface{} {
    switch method {
    case "getLatestBlockhash", "getSlot":
        return []interface{}{map[string]string{"commitment": "finalized"}}
    case "getSignatureStatuses":
        // Provide an empty array of signatures to get recent statuses
        return []interface{}{[]string{}}
    case "getBalance", "getAccountInfo":
        // Use system program address as a test account
        return []interface{}{"11111111111111111111111111111111"}
    case "getProgramAccounts":
        // Use system program ID
        return []interface{}{
            "11111111111111111111111111111111",
            map[string]interface{}{
                "encoding": "base64",
            },
        }
    case "getBlock":
        // Get the latest block
        return []interface{}{
            "finalized",
            map[string]interface{}{
                "encoding": "json",
                "transactionDetails": "none",
                "rewards": false,
            },
        }
    case "getBlockTime":
        // Get current slot first, then use that
        var slot uint64
        if err := c.client.Call(context.Background(), "getSlot", nil, &slot); err == nil {
            return []interface{}{slot}
        }
        return []interface{}{0} // fallback to slot 0
    default:
        return nil
    }
}

func (c *Collector) categorizeError(err error) string {
    if rpcErr, ok := err.(*solana.RPCError); ok {
        switch rpcErr.Code {
        case -32700:
            return "parse_error"
        case -32600:
            return "invalid_request"
        case -32601:
            return "method_not_found"
        case -32602:
            return "invalid_params"
        case -32603:
            return "internal_error"
        default:
            return fmt.Sprintf("rpc_error_%d", rpcErr.Code)
        }
    }
    return "unknown_error"
}

func (c *Collector) collectQueueMetrics(ctx context.Context, baseLabels []string) error {
    // Track in-flight requests
    inFlightCount := c.client.GetInflightRequests()
    c.metrics.RPCInFlight.WithLabelValues(baseLabels...).Set(float64(inFlightCount))

    return nil
}
