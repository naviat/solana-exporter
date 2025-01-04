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

type PerformanceSample struct {
    Slot             uint64  `json:"slot"`
    NumTransactions  uint64  `json:"numTransactions"`
    NumSlots         uint64  `json:"numSlots"`
    SamplePeriodSecs float64 `json:"samplePeriodSecs"`
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
    log.Printf("Initializing RPC collector with labels: %v", labels)
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
    log.Printf("Starting metrics collection cycle...")
    var wg sync.WaitGroup
    errCh := make(chan error, 3)

    collectors := []struct {
        name string
        fn   func(context.Context) error
    }{
        {"slot", c.collectSlotMetrics},
        {"block", c.collectBlockMetrics},
        {"performance", c.collectPerformanceMetrics},
    }

    for _, collector := range collectors {
        wg.Add(1)
        go func(name string, fn func(context.Context) error) {
            defer wg.Done()
            log.Printf("Collecting %s metrics...", name)
            if err := fn(ctx); err != nil {
                log.Printf("Error collecting %s metrics: %v", name, err)
                errCh <- fmt.Errorf("%s: %w", name, err)
            } else {
                log.Printf("Successfully collected %s metrics", name)
            }
        }(collector.name, collector.fn)
    }

    // Collect cached metrics (version and health)
    if err := c.collectCachedMetrics(ctx); err != nil {
        log.Printf("Error collecting cached metrics: %v", err)
        errCh <- fmt.Errorf("cached metrics: %w", err)
    }

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

    log.Printf("Completed metrics collection cycle")
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

        // Set current slot metric
        c.metrics.CurrentSlot.WithLabelValues(
            c.nodeLabels["endpoint"],
            "finalized",
        ).Set(float64(localSlot))

        log.Printf("Local slot: %d", localSlot)

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

        log.Printf("Reference slot: %d", refSlot)

        c.metrics.NetworkSlot.WithLabelValues(
            c.nodeLabels["endpoint"],
        ).Set(float64(refSlot))

        // Calculate and set slot difference if we have both values
        if localSlot > 0 && refSlot > 0 {
            slotDiff := refSlot - localSlot
            if slotDiff >= 0 {
                log.Printf("Slot difference: %d", slotDiff)
                c.metrics.SlotBehind.WithLabelValues(
                    c.nodeLabels["endpoint"],
                ).Set(float64(slotDiff))
            }
        }
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

func (c *RPCCollector) collectBlockMetrics(ctx context.Context) error {
    // Get current block height first
    var blockHeight uint64
    start := time.Now()
    err := c.localClient.Call(ctx, "getBlockHeight", []interface{}{
        map[string]string{"commitment": "finalized"},
    }, &blockHeight)
    duration := time.Since(start)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(
            c.nodeLabels["endpoint"],
            "getBlockHeight",
            fmt.Sprintf("%d", err.(*solana.RPCError).Code),
        ).Inc()
        return err
    }

    c.metrics.BlockHeight.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(blockHeight))

    c.metrics.RPCLatency.WithLabelValues(
        c.nodeLabels["endpoint"],
        "getBlockHeight",
    ).Observe(duration.Seconds())

    // Get latest block time
    start = time.Now()
    var latestBlockTime struct {
        AbsoluteSlot uint64 `json:"absoluteSlot"`
        BlockTime    int64  `json:"blockTime"`
        BlockHeight  uint64 `json:"blockHeight"`
    }
    err = c.localClient.Call(ctx, "getBlockTime", []interface{}{blockHeight}, &latestBlockTime)
    duration = time.Since(start)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(
            c.nodeLabels["endpoint"],
            "getBlockTime",
            fmt.Sprintf("%d", err.(*solana.RPCError).Code),
        ).Inc()
        return err
    }

    if latestBlockTime.BlockTime > 0 {
        // Calculate seconds since last block
        timeSinceBlock := time.Now().Unix() - latestBlockTime.BlockTime
        c.metrics.BlockTime.WithLabelValues(c.nodeLabels["endpoint"]).
            Set(float64(timeSinceBlock))
    }

    c.metrics.RPCLatency.WithLabelValues(
        c.nodeLabels["endpoint"],
        "getBlockTime",
    ).Observe(duration.Seconds())

    return nil
}

func (c *RPCCollector) collectPerformanceMetrics(ctx context.Context) error {
    var samples []PerformanceSample
    start := time.Now()
    err := c.localClient.Call(ctx, "getRecentPerformanceSamples", []interface{}{5}, &samples)
    duration := time.Since(start)

    if err != nil {
        c.metrics.RPCErrors.WithLabelValues(
            c.nodeLabels["endpoint"],
            "getRecentPerformanceSamples",
            fmt.Sprintf("%d", err.(*solana.RPCError).Code),
        ).Inc()
        return err
    }

    c.metrics.RPCLatency.WithLabelValues(
        c.nodeLabels["endpoint"],
        "getRecentPerformanceSamples",
    ).Observe(duration.Seconds())

    if len(samples) > 0 {
        var totalTPS float64
        maxTPS := float64(0)
        minTPS := float64(samples[0].NumTransactions) / samples[0].SamplePeriodSecs
        var totalTx uint64

        for _, sample := range samples {
            tps := float64(sample.NumTransactions) / sample.SamplePeriodSecs
            totalTPS += tps
            totalTx += sample.NumTransactions

            if tps > maxTPS {
                maxTPS = tps
            }
            if tps < minTPS {
                minTPS = tps
            }
        }

        // Set average TPS
        avgTPS := totalTPS / float64(len(samples))
        c.metrics.TPS.WithLabelValues(c.nodeLabels["endpoint"]).Set(avgTPS)
        c.metrics.TPSMax.WithLabelValues(c.nodeLabels["endpoint"]).Set(maxTPS)
        c.metrics.TPSMin.WithLabelValues(c.nodeLabels["endpoint"]).Set(minTPS)
        
        // Update total transaction count
        c.metrics.TxCount.WithLabelValues(c.nodeLabels["endpoint"]).Add(float64(totalTx))
    }

    return nil
}

func (c *RPCCollector) collectCachedMetrics(ctx context.Context) error {
    c.cache.mu.RLock()
    needsVersion := time.Since(c.cache.versionTime) >= c.cache.cacheTimeout
    needsHealth := time.Since(c.cache.healthTime) >= c.cache.cacheTimeout
    c.cache.mu.RUnlock()

    var errs []error

    if needsVersion {
        start := time.Now()
        var versionInfo struct {
            SolanaCore string `json:"solana-core"`
        }

        err := c.localClient.Call(ctx, "getVersion", nil, &versionInfo)
        duration := time.Since(start)

        if err != nil {
            c.metrics.RPCErrors.WithLabelValues(
                c.nodeLabels["endpoint"],
                "getVersion",
                fmt.Sprintf("%d", err.(*solana.RPCError).Code),
            ).Inc()
            errs = append(errs, fmt.Errorf("version update: %w", err))
        } else {
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
        }
    }

    if needsHealth {
        start := time.Now()
        var healthStatus string
        err := c.localClient.Call(ctx, "getHealth", nil, &healthStatus)
        duration := time.Since(start)

        if err != nil {
            c.metrics.RPCErrors.WithLabelValues(
                c.nodeLabels["endpoint"],
                "getHealth",
                fmt.Sprintf("%d", err.(*solana.RPCError).Code),
            ).Inc()
            errs = append(errs, fmt.Errorf("health update: %w", err))
        } else {
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
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("multiple cached metric errors: %v", errs)
    }

    return nil
}

// Stop implements the StoppableCollector interface
func (c *RPCCollector) Stop() error {
    return nil
}
