package collector

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"
    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
)

// Essential RPC methods to monitor
var essentialMethods = []string{
    "getHealth",
    "getSlot",
    "getVersion",
    "getBlockHeight",
    "getLatestBlockhash",
}

type RPCCollector struct {
    localClient     *solana.Client
    referenceClient *solana.Client
    metrics         *metrics.Metrics
    nodeLabels      map[string]string
    cache           *metricCache
}

type metricCache struct {
    mu             sync.RWMutex
    versionInfo    string
    versionTime    time.Time
    healthStatus   bool
    healthTime     time.Time
    cacheTimeout   time.Duration
}

func NewRPCCollector(localClient, referenceClient *solana.Client, metrics *metrics.Metrics, labels map[string]string) *RPCCollector {
    return &RPCCollector{
        localClient:     localClient,
        referenceClient: referenceClient,
        metrics:         metrics,
        nodeLabels:      labels,
        cache: &metricCache{
            cacheTimeout: 5 * time.Minute,
        },
    }
}

func (c *RPCCollector) Name() string {
    return "rpc"
}

func (c *RPCCollector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, 3)

    // Collect metrics concurrently
    collectors := []struct {
        name string
        fn   func(context.Context) error
    }{
        {"slot", c.collectSlotMetrics},
        {"cached", c.collectCachedMetrics},
        {"performance", c.collectPerformanceMetrics},
    }

    for _, collector := range collectors {
        wg.Add(1)
        go func(name string, fn func(context.Context) error) {
            defer wg.Done()
            if err := fn(ctx); err != nil {
                errCh <- fmt.Errorf("%s: %w", name, err)
            }
        }(collector.name, collector.fn)
    }

    // Monitor inflight requests
    c.metrics.RPCInflightRequests.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(c.localClient.GetInflightRequests()))

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

func (c *RPCCollector) collectSlotMetrics(ctx context.Context) error {
    var (
        localSlot uint64
        refSlot   uint64
        wg        sync.WaitGroup
        errCh     = make(chan error, 2)
    )

    // Get local node slot
    wg.Add(1)
    go func() {
        defer wg.Done()
        start := time.Now()
        err := c.localClient.Call(ctx, "getSlot", []interface{}{
            map[string]string{"commitment": "finalized"},
        }, &localSlot)
        duration := time.Since(start)

        if err != nil {
            errCh <- err
            c.metrics.RPCErrors.WithLabelValues(
                c.nodeLabels["endpoint"],
                "getSlot",
                fmt.Sprintf("%d", err.(*solana.RPCError).Code),
            ).Inc()
            return
        }

        c.metrics.CurrentSlot.WithLabelValues(
            c.nodeLabels["endpoint"],
            "finalized",
        ).Set(float64(localSlot))

        c.metrics.RPCLatency.WithLabelValues(
            c.nodeLabels["endpoint"],
            "getSlot",
        ).Observe(duration.Seconds())
    }()

    // Get reference node slot
    wg.Add(1)
    go func() {
        defer wg.Done()
        err := c.referenceClient.Call(ctx, "getSlot", []interface{}{
            map[string]string{"commitment": "finalized"},
        }, &refSlot)

        if err != nil {
            errCh <- err
            return
        }

        c.metrics.NetworkSlot.WithLabelValues(c.nodeLabels["endpoint"]).
            Set(float64(refSlot))

        // Calculate and set slot difference
        c.metrics.SlotBehind.WithLabelValues(c.nodeLabels["endpoint"]).
            Set(float64(refSlot - localSlot))
    }()

    wg.Wait()
    close(errCh)

    for err := range errCh {
        if err != nil {
            return fmt.Errorf("slot metrics: %w", err)
        }
    }

    return nil
}

func (c *RPCCollector) collectCachedMetrics(ctx context.Context) error {
    c.cache.mu.RLock()
    needsVersion := time.Since(c.cache.versionTime) >= c.cache.cacheTimeout
    needsHealth := time.Since(c.cache.healthTime) >= c.cache.cacheTimeout
    c.cache.mu.RUnlock()

    if needsVersion {
        if err := c.updateVersionInfo(ctx); err != nil {
            log.Printf("Failed to update version info: %v", err)
        }
    }

    if needsHealth {
        if err := c.updateHealthInfo(ctx); err != nil {
            log.Printf("Failed to update health info: %v", err)
        }
    }

    return nil
}

func (c *RPCCollector) updateVersionInfo(ctx context.Context) error {
    var versionInfo struct {
        SolanaCore string `json:"solana-core"`
    }

    start := time.Now()
    err := c.localClient.Call(ctx, "getVersion", nil, &versionInfo)
    duration := time.Since(start)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(
            c.nodeLabels["endpoint"],
            "getVersion",
            fmt.Sprintf("%d", err.(*solana.RPCError).Code),
        ).Inc()
        return err
    }

    c.cache.mu.Lock()
    c.cache.versionInfo = versionInfo.SolanaCore
    c.cache.versionTime = time.Now()
    c.cache.mu.Unlock()

    c.metrics.NodeVersion.WithLabelValues(
        c.nodeLabels["endpoint"],
        versionInfo.SolanaCore,
    ).Set(1)

    c.metrics.RPCLatency.WithLabelValues(
        c.nodeLabels["endpoint"],
        "getVersion",
    ).Observe(duration.Seconds())

    return nil
}

func (c *RPCCollector) updateHealthInfo(ctx context.Context) error {
    var healthStatus string
    
    start := time.Now()
    err := c.localClient.Call(ctx, "getHealth", nil, &healthStatus)
    duration := time.Since(start)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(
            c.nodeLabels["endpoint"],
            "getHealth",
            fmt.Sprintf("%d", err.(*solana.RPCError).Code),
        ).Inc()
        return err
    }

    isHealthy := healthStatus == "ok"
    c.cache.mu.Lock()
    c.cache.healthStatus = isHealthy
    c.cache.healthTime = time.Now()
    c.cache.mu.Unlock()

    healthValue := 0.0
    if isHealthy {
        healthValue = 1.0
    }

    c.metrics.NodeHealth.WithLabelValues(c.nodeLabels["endpoint"]).Set(healthValue)

    c.metrics.RPCLatency.WithLabelValues(
        c.nodeLabels["endpoint"],
        "getHealth",
    ).Observe(duration.Seconds())

    return nil
}

func (c *RPCCollector) collectPerformanceMetrics(ctx context.Context) error {
    // Collect metrics for each essential method
    for _, method := range essentialMethods {
        if method == "getSlot" || method == "getHealth" || method == "getVersion" {
            continue // Skip methods that are already collected elsewhere
        }

        start := time.Now()
        var result interface{}
        
        err := c.localClient.Call(ctx, method, nil, &result)
        duration := time.Since(start)

        // Record request count
        c.metrics.RPCRequests.WithLabelValues(
            c.nodeLabels["endpoint"],
            method,
        ).Inc()

        // Record latency
        c.metrics.RPCLatency.WithLabelValues(
            c.nodeLabels["endpoint"],
            method,
        ).Observe(duration.Seconds())

        // Record errors if any
        if err != nil {
            if rpcErr, ok := err.(*solana.RPCError); ok {
                c.metrics.RPCErrors.WithLabelValues(
                    c.nodeLabels["endpoint"],
                    method,
                    fmt.Sprintf("%d", rpcErr.Code),
                ).Inc()
            }
        }
    }

    // Get WebSocket stats if available
    stats := c.localClient.GetWSStats()
    if connected, ok := stats["connected"].(bool); ok && connected {
        c.metrics.WSConnections.WithLabelValues(c.nodeLabels["endpoint"]).
            Set(1)
    }

    return nil
}
